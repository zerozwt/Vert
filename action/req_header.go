package action

import (
	"errors"
	"net/http"
)

func init() {
	registerActionFunc("set-header", set_header)
	registerActionFunc("del-header", del_header)
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
