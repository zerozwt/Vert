package action

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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

	if acceptGZip(req) {
		tmp := &gzipRspWriter{
			underlying: rsp,
			writer:     noopWriteCloser{rsp},
		}
		defer tmp.Close()
		self.fs.ServeHTTP(tmp, req)
		return
	}

	self.fs.ServeHTTP(rsp, req)
}

type gzipRspWriter struct {
	underlying http.ResponseWriter

	writer io.WriteCloser
	once   int32
}

func (self *gzipRspWriter) Header() http.Header {
	return self.underlying.Header()
}

func (self *gzipRspWriter) WriteHeader(statusCode int) {
	if !atomic.CompareAndSwapInt32(&(self.once), 0, 1) {
		return
	}

	defer self.underlying.WriteHeader(statusCode)

	if len(self.underlying.Header().Values("Content-Encoding")) > 0 {
		// do not enable gzip if content encoding is set
		return
	}

	if !isTextContentType(self.underlying.Header().Get("Content-Type")) {
		// also do not enable gzip if content type is not text
		return
	}

	self.underlying.Header().Set("Content-Encoding", "gzip")
	self.underlying.Header().Del("Content-Length")

	self.writer = gzip.NewWriter(self.underlying)
}

func (self *gzipRspWriter) Write(buf []byte) (int, error) {
	self.WriteHeader(200)
	return self.writer.Write(buf)
}

func (self *gzipRspWriter) Close() error {
	return self.writer.Close()
}

type noopWriteCloser struct {
	io.Writer
}

func (self noopWriteCloser) Close() error { return nil }
