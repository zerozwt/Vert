package action

import (
	"errors"
	"net/http"
	"strings"
)

func init() {
	registerActionFunc("wwwroot", wwwroot)
}

func wwwroot(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("wwwroor params count invalid")
	}

	return &safeWWWRoot{fs: http.FileServer(http.Dir(params[0]))}, nil
}

type safeWWWRoot struct {
	fs http.Handler
}

func (self *safeWWWRoot) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	//forbid father directory
	if strings.Contains(req.URL.Path, "..") {
		http.Error(rsp, "Forbidden", 403)
		return
	}

	//forbid hidden files
	if len(req.URL.Path) > 0 {
		check := req.URL.Path
		idx := strings.Index(check, "/")
		for idx >= 0 && idx < len(check)-1 {
			if check[idx+1] == '.' {
				http.Error(rsp, "Not found", 404)
				return
			}
			check = check[idx+1:]
			idx = strings.Index(check, "/")
		}
	}

	self.fs.ServeHTTP(rsp, req)
}
