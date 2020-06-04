package action

import (
	"errors"
	"net/http"
)

func init() {
	registerActionFunc("redirect", redirect)
}

func redirect(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("redirect params count invalid")
	}

	v, err := convertActionParam(params[0])
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		http.Redirect(rsp, req, v.Parse(req), 301)
	}), nil
}
