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

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		http.Redirect(rsp, req, params[0], 301)
	}), nil
}
