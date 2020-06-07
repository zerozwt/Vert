package action

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/zerozwt/Vert/env"
)

func TestVariable_PATH(t *testing.T) {
	target := `[{path}] [{path[10:]}] [{path[:10]}] [{path[10:20]}]`

	uri := url.URL{Path: "/hello/world/yjsnpi/tohno"}
	req := http.Request{URL: &uri}

	check := fmt.Sprintf("[%s] [%s] [%s] [%s]", uri.Path, uri.Path[10:], uri.Path[:10], uri.Path[10:20])

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != check {
		t.Errorf("'path' convert not as expected: expected=%s actual=%s", check, tmp)
	}
}

func TestVariable_SEG(t *testing.T) {
	target := `[{seg[0]}] [{seg[1:]}] [{seg[:2]}] [{seg[1:3]}]`

	uri := url.URL{Path: "/hello/world/yjsnpi/tohno"}
	req := http.Request{URL: &uri}

	check := `[hello] [world/yjsnpi/tohno] [hello/world] [world/yjsnpi]`

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != check {
		t.Errorf("'seg' convert not as expected: expected=%s actual=%s", check, tmp)
	}
}

func TestVariable_Query(t *testing.T) {
	target := `{has_query} [{query}] [{query:d}] [{%query:d}] [{query:[a,v,b,c]}] [{^query:[a,b,c,e]}]`

	uri := url.URL{RawQuery: "a=1&b=2&c=3&d=%2a&e=5"}
	req := http.Request{URL: &uri}

	check := `? [a=1&b=2&c=3&d=%2a&e=5] [*] [%2A] [a=1&b=2&c=3] [d=%2A]`

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != check {
		t.Errorf("'query' convert not as expected: expected=%s actual=%s", check, tmp)
	}
}

func TestVariable_Fragment(t *testing.T) {
	target := `{has_fragment} [{fragment}]`

	uri := url.URL{Fragment: "hello-1"}
	req := http.Request{URL: &uri}

	check := `# [hello-1]`

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != check {
		t.Errorf("'fragment' convert not as expected: expected=%s actual=%s", check, tmp)
	}
}

func TestVariable_Other(t *testing.T) {
	target := `[{mux:domain}] [{re[1]}]`

	uri := url.URL{Fragment: "hello-1"}
	req := http.Request{URL: &uri}

	check := `[] [${1}]`

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != check {
		t.Errorf("'others' convert not as expected: expected=%s actual=%s", check, tmp)
	}
}

func TestVariableHost(t *testing.T) {
	target := `{host}`
	req := env.WrapRequest(&http.Request{Host: "www.yjsnpi.com"})
	req.Host = ""

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(req); tmp != "www.yjsnpi.com" {
		t.Errorf("'host' convert not as expected: %s", tmp)
	}
}

func TestVariableUpstream(t *testing.T) {
	target := `{up:yjsnpi}`

	if err := env.AddUpsteam(map[string][]string{"yjsnpi": {"www.yjsnpi.com"}}); err != nil {
		t.Error(err)
	}

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(nil); tmp != "www.yjsnpi.com" {
		t.Errorf("'up' convert not as expected: %s", tmp)
	}
}

func TestVariableFullPath(t *testing.T) {
	target := `{fullpath}`
	path := `/hello/world?a=1#yjsnpi`

	uri, err := url.Parse(path)
	if err != nil {
		t.Error(err)
	}
	req := http.Request{URL: uri}

	v, err := convertActionParam(target)
	if err != nil {
		t.Error("build variable failed:", err)
		return
	}

	if tmp := v.Parse(&req); tmp != path {
		t.Errorf("'query' convert not as expected: expected=%s actual=%s", path, tmp)
	}
}
