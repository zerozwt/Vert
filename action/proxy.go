package action

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
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

	return &reverseProxy{
		target_addr:     v,
		mod_rsp_header:  make([]rspHeaderModifier, 0),
		mod_rsp_content: make([]rspContentModifier, 0),
	}, nil
}

type reverseProxy struct {
	target_addr     Variable
	mod_rsp_header  []rspHeaderModifier
	mod_rsp_content []rspContentModifier
}

func (self *reverseProxy) AddRspHeaderModifier(item rspHeaderModifier) {
	self.mod_rsp_header = append([]rspHeaderModifier{item}, self.mod_rsp_header...)
}

func (self *reverseProxy) AddRspContentModifier(item rspContentModifier) {
	self.mod_rsp_content = append([]rspContentModifier{item}, self.mod_rsp_content...)
}

func (self *reverseProxy) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	upstream_addr := self.target_addr.Parse(req)

	upstream_req, err := http.NewRequest(req.Method, upstream_addr, req.Body)
	if err != nil {
		ERROR_LOG("create upstream request (%s) failed: %v", upstream_addr, err)
		http.Error(rsp, err.Error(), 502)
		return
	}
	upstream_req.Header = req.Header.Clone()
	upstream_req.Header.Set("Host", upstream_req.URL.Host)

	//if there is any content modifier, Accept-Encoding should be deleted from request's header
	if len(self.mod_rsp_content) > 0 {
		upstream_req.Header.Del("Accept-Encoding")
	}

	upstream_rsp, err := http.DefaultClient.Do(upstream_req)
	if err != nil {
		ERROR_LOG("upstream request (%s) failed: %v", upstream_addr, err)
		http.Error(rsp, err.Error(), 502)
		return
	}

	for _, header_modifier := range self.mod_rsp_header {
		upstream_rsp.Header = header_modifier.ModifyHeader(req, upstream_rsp.Header)
	}

	for key, value_list := range upstream_rsp.Header {
		for _, value := range value_list {
			rsp.Header().Add(key, value)
		}
	}
	defer upstream_rsp.Body.Close()

	if len(self.mod_rsp_content) == 0 {
		buf := make([]byte, 4096)
		rsp.WriteHeader(upstream_rsp.StatusCode)
		io.CopyBuffer(rsp, upstream_rsp.Body, buf)
	} else {
		content, err := ioutil.ReadAll(upstream_rsp.Body)
		if err != nil {
			panic(err)
		}
		if isTextContentType(upstream_rsp.Header.Get("Content-Type")) {
			for _, content_modifier := range self.mod_rsp_content {
				content = content_modifier.ModifyContent(req, content)
			}
			if strings.Index(req.Header.Get("Accept-Encoding"), "gzip") >= 0 {
				rsp.Header().Set("Content-Encoding", "gzip")
				buf := bytes.NewBuffer(nil)
				zw := gzip.NewWriter(buf)
				zw.Write(content)
				zw.Close()
				content = buf.Bytes()
			}
		}
		rsp.Header().Set("Content-Length", fmt.Sprint(len(content)))
		rsp.WriteHeader(upstream_rsp.StatusCode)
		rsp.Write(content)
	}
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

var ctAppText map[string]bool = map[string]bool{
	"application/atom+xml":   true,
	"application/ecmascript": true,
	"application/json":       true,
	"application/javascript": true,
	"application/rss+xml":    true,
	"application/soap+xml":   true,
	"application/xhtml+xml":  true,
	"application/xml":        true,
}

func isTextContentType(content_type string) bool {
	if len(content_type) == 0 {
		return true
	}
	if strings.HasPrefix(content_type, "text/") {
		return true
	}
	_, ok := ctAppText[strings.Split(content_type, ";")[0]]
	return ok
}
