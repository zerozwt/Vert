package action

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1 << 14,
	WriteBufferSize: 1 << 14,
}

func init() {
	registerActionFunc("proxy", proxy)
}

func proxy(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("proxy params count invalid")
	}

	uri, err := url.Parse(params[0])
	if err != nil {
		return nil, err
	}

	if uri.Scheme == "http" || uri.Scheme == "https" {
		return proxyNormal(params[0])
	}

	if uri.Scheme == "ws" || uri.Scheme == "wss" {
		return proxyWebsocket(params[0])
	}

	return nil, errors.New("invalid proxy scheme: " + uri.Scheme)
}

func proxyNormal(param string) (http.Handler, error) {
	v, err := convertActionParam(param)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		upstream_addr := v.Parse(req)
		upstream_req, err := http.NewRequest(req.Method, upstream_addr, req.Body)
		if err != nil {
			ERROR_LOG("create upstream request (%s) failed: %v", upstream_addr, err)
			http.Error(rsp, err.Error(), 502)
			return
		}
		upstream_req.Header = req.Header.Clone()

		upstream_rsp, err := http.DefaultClient.Do(upstream_req)
		if err != nil {
			ERROR_LOG("upstream request (%s) failed: %v", upstream_addr, err)
			http.Error(rsp, err.Error(), 502)
			return
		}

		rsp.WriteHeader(upstream_rsp.StatusCode)
		for key, value_list := range upstream_rsp.Header {
			for _, value := range value_list {
				rsp.Header().Add(key, value)
			}
		}
		defer upstream_rsp.Body.Close()

		io.Copy(rsp, upstream_rsp.Body)
	}), nil
}

func proxyWebsocket(param string) (http.Handler, error) {
	v, err := convertActionParam(param)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		upstream_addr := v.Parse(req)
		dailer := &websocket.Dialer{}
		up_conn, _, err := dailer.Dial(upstream_addr, req.Header.Clone())
		if err != nil {
			ERROR_LOG("create upstream websocket (%s) failed: %v", upstream_addr, err)
			http.Error(rsp, err.Error(), 502)
			return
		}
		defer up_conn.Close()

		conn, err := upgrader.Upgrade(rsp, req, nil)
		if err != nil {
			ERROR_LOG("upgrade to websocket (%s) failed: %v", upstream_addr, err)
			http.Error(rsp, err.Error(), 502)
			return
		}
		defer conn.Close()

		ch := make(chan bool, 2)

		copy := func(from, to *websocket.Conn) {
			defer func() { ch <- true }()
			for {
				mt, msg, err := from.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						ERROR_LOG("websocket read failed: %v", err)
					}
					to.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
					return
				}
				if mt == 1 || mt == 2 {
					if err := to.WriteMessage(mt, msg); err != nil {
						ERROR_LOG("websocket write failed: %v", err)
						return
					}
				}
			}
		}

		go copy(conn, up_conn)
		go copy(up_conn, conn)

		<-ch
		<-ch
	}), nil
}
