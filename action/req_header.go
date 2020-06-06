package action

import (
	"errors"
	"net/http"
	"strings"
)

func init() {
	registerActionFunc("set-header", set_header)
	registerActionFunc("del-header", del_header)
	registerActionFunc("limit-referer", limit_referer)
}

func set_header(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 2 {
		return nil, errors.New("set-header params count invalid")
	}

	v, err := convertActionParam(params[1])
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		req.Header.Set(params[0], v.Parse(req))
		underlying.ServeHTTP(rsp, req)
	}), nil
}

func del_header(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("del-header params count invalid")
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		req.Header.Del(params[0])
		underlying.ServeHTTP(rsp, req)
	}), nil
}

func limit_referer(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("limit-referer params count invalid")
	}

	v, err := convertActionParam(params[0])
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" && !strings.HasPrefix(req.Header.Get("Referer"), v.Parse(req)) {
			http.Error(rsp, "Forbidden", 403)
			return
		}
		underlying.ServeHTTP(rsp, req)
	}), nil
}
