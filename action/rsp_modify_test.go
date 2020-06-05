package action

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type dummyRsp struct {
	data   []byte
	header http.Header
}

func (self *dummyRsp) Header() http.Header            { return self.header }
func (self *dummyRsp) WriteHeader(statusCode int)     {}
func (self *dummyRsp) Write(data []byte) (int, error) { self.data = data; return len(data), nil }

func newDummyRsp() *dummyRsp {
	return &dummyRsp{data: make([]byte, 0), header: http.Header{}}
}

type testHandler struct {
	target_addr Variable

	mod_rsp_header  []rspHeaderModifier
	mod_rsp_content []rspContentModifier
}

func (self *testHandler) AddRspHeaderModifier(item rspHeaderModifier) {
	self.mod_rsp_header = append(self.mod_rsp_header, item)
}

func (self *testHandler) AddRspContentModifier(item rspContentModifier) {
	self.mod_rsp_content = append(self.mod_rsp_content, item)
}

func (self *testHandler) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	upstream_addr := self.target_addr.Parse(req)
	fmt.Println(upstream_addr)

	rsp_header := http.Header{}
	rsp_header.Add("Set-Cookie", "mur=kmr; Domain=yjsnpi.com; Secure; HttpOnly")
	rsp_header.Add("Set-Cookie", "szk=yjsnpi; Domain=yjsnpi.com; Secure; HttpOnly")
	rsp_header.Add("Header-To-Delete", "xxxxxxxx")

	for _, header_modifier := range self.mod_rsp_header {
		rsp_header = header_modifier.ModifyHeader(req, rsp_header)
	}

	for key, value_list := range rsp_header {
		for _, value := range value_list {
			rsp.Header().Add(key, value)
		}
	}

	content := []byte(`<a href="https://kmr.yjsnpi.com/chapter_4.mp4">Tohno</a>`)
	for _, content_modifier := range self.mod_rsp_content {
		content = content_modifier.ModifyContent(req, content)
	}
	rsp.Header().Set("Content-Length", fmt.Sprint(len(content)))
	rsp.Write(content)
}

func newTestHandler(target string) (http.Handler, error) {
	v, err := convertActionParam(target)
	if err != nil {
		return nil, err
	}
	return &testHandler{
		target_addr:     v,
		mod_rsp_header:  make([]rspHeaderModifier, 0),
		mod_rsp_content: make([]rspContentModifier, 0),
	}, nil
}

func testActionSet(uri string, actions []string, t *testing.T) *dummyRsp {
	req, err := http.NewRequest("GET", uri, bytes.NewReader([]byte{}))
	if err != nil {
		t.Error(err)
		return nil
	}

	handler, err := newTestHandler("https://www.yjsnpi.com/")
	if err != nil {
		t.Error(err)
		return nil
	}

	for i := len(actions) - 1; i >= 0; i-- {
		handler, err = ActionHandler(actions[i], handler)
		if err != nil {
			t.Error(err)
			return nil
		}
	}

	rsp := newDummyRsp()

	handler.ServeHTTP(rsp, req)

	return rsp
}

func TestContentModify(t *testing.T) {
	uri := "http://www.mur.com/"
	actions := []string{
		"set-rsp-header Hello world",
		"del-rsp-header Header-To-Delete",
		"proxy-cookie mur.com yjsnpi.com",
		`filter-content ://([a-z]+).yjsnpi.com/ ://www.mur.com/yjsnpi/{re[1]}/`,
	}

	rsp := testActionSet(uri, actions, t)
	if rsp == nil {
		t.Errorf("testActionSet failed")
		return
	}

	expect_data := []byte(`<a href="https://www.mur.com/yjsnpi/kmr/chapter_4.mp4">Tohno</a>`)
	expect_length := fmt.Sprint(len(expect_data))

	if tmp := rsp.header.Get("Header-To-Delete"); tmp != "" {
		t.Errorf("Header-To-Delete not deleted: %s", tmp)
		return
	}

	if tmp := rsp.header.Get("Hello"); tmp != "world" {
		t.Errorf("Set rsp header failed: Hello=%s", tmp)
		return
	}

	for _, cookie := range rsp.header.Values("Set-Cookie") {
		if strings.Index(cookie, "mur.com") < 0 {
			t.Errorf("Set rsp cookie failed: Cookie=%s", cookie)
			return
		}
	}

	if len(rsp.header.Values("Set-Cookie")) != 2 {
		t.Errorf("Set rsp cookie failed: cookies count changed: %v", rsp.header.Values("Set-Cookie"))
		return
	}

	if tmp := rsp.header.Get("Content-Length"); tmp != expect_length {
		t.Errorf("Content length is not expect(%s): %s", expect_length, tmp)
		return
	}

	if string(rsp.data) != string(expect_data) {
		t.Errorf("Content is not expect(%s): %s", expect_data, rsp.data)
		return
	}
}
