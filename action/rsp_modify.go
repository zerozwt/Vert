package action

import (
	"errors"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"
)

func init() {
	registerActionFunc("set-rsp-header", set_rsp_header)
	registerActionFunc("del-rsp-header", del_rsp_header)
	registerActionFunc("proxy-cookie", proxy_cookie)
	registerActionFunc("filter-content", filter_content)
}

//-----------------------------------------------------------------------------

type rspHeaderModifier interface {
	ModifyHeader(*http.Request, http.Header) http.Header
}

type rspHeaderMutable interface {
	AddRspHeaderModifier(rspHeaderModifier)
}

type rspContentModifier interface {
	ModifyContent(*http.Request, []byte) []byte
}

type rspContentMutable interface {
	AddRspContentModifier(rspContentModifier)
}

//-----------------------------------------------------------------------------

type rspHeaderSetter struct {
	key   string
	value Variable
}

func (self *rspHeaderSetter) ModifyHeader(req *http.Request, header http.Header) http.Header {
	header.Set(self.key, self.value.Parse(req))
	return header
}

func set_rsp_header(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 2 {
		return nil, errors.New("set-rsp-header params count invalid")
	}

	v, err := convertActionParam(params[1])
	if err != nil {
		return nil, err
	}

	if tmp, ok := underlying.(rspHeaderMutable); ok {
		tmp.AddRspHeaderModifier(&rspHeaderSetter{key: params[0], value: v})
		return underlying, nil
	}

	return nil, errors.New("underlying action dos not support set-rsp-header")
}

//-----------------------------------------------------------------------------

type rspHeaderDel string

func (self rspHeaderDel) ModifyHeader(req *http.Request, header http.Header) http.Header {
	header.Del(string(self))
	return header
}

func del_rsp_header(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 1 {
		return nil, errors.New("del-rsp-header params count invalid")
	}

	if tmp, ok := underlying.(rspHeaderMutable); ok {
		tmp.AddRspHeaderModifier(rspHeaderDel(params[0]))
		return underlying, nil
	}

	return nil, errors.New("underlying action dos not support del-rsp-header")
}

//-----------------------------------------------------------------------------

type rspCookie struct {
	this_domain     Variable
	upstream_domain Variable
}

func (self *rspCookie) ModifyHeader(req *http.Request, header http.Header) http.Header {
	set_cookie := textproto.CanonicalMIMEHeaderKey("Set-Cookie")

	if len(header.Values(set_cookie)) == 0 {
		return header
	}

	this_domain := self.this_domain.Parse(req)
	upstream_domain := self.upstream_domain.Parse(req)

	orig_cookies := header.Values(set_cookie)
	header.Del(set_cookie)

	for _, cookie := range orig_cookies {
		segs := strings.Split(cookie, "; ")
		domain_idx := -1
		lowercase := false
		for idx, seg := range segs {
			if seg == "Domain="+upstream_domain {
				domain_idx = idx
				break
			} else if seg == "domain="+upstream_domain {
				domain_idx = idx
				lowercase = true
				break
			}
		}
		if domain_idx >= 0 {
			if !lowercase {
				segs[domain_idx] = "Domain=" + this_domain
			} else {
				segs[domain_idx] = "domain=" + this_domain
			}
		}
		header.Add(set_cookie, strings.Join(segs, "; "))
	}

	return header
}

func proxy_cookie(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 2 {
		return nil, errors.New("proxy-cookie params count invalid")
	}

	if tmp, ok := underlying.(rspHeaderMutable); ok {
		v_this, err := convertActionParam(params[0])
		if err != nil {
			return nil, err
		}

		v_up, err := convertActionParam(params[1])
		if err != nil {
			return nil, err
		}
		tmp.AddRspHeaderModifier(&rspCookie{this_domain: v_this, upstream_domain: v_up})
		return underlying, nil
	}

	return nil, errors.New("underlying action dos not support proxy-cookie")
}

//-----------------------------------------------------------------------------

type filterContent struct {
	pattern     *regexp.Regexp
	replacement Variable
}

func (self *filterContent) ModifyContent(req *http.Request, content []byte) []byte {
	repl := []byte(self.replacement.Parse(req))
	return self.pattern.ReplaceAll(content, repl)
}

func filter_content(params []string, underlying http.Handler) (http.Handler, error) {
	if len(params) != 2 {
		return nil, errors.New("filter-content params count invalid")
	}

	pattern, err := regexp.Compile(params[0])
	if err != nil {
		return nil, err
	}

	repl, err := convertActionParam(params[1])
	if err != nil {
		return nil, err
	}

	if tmp, ok := underlying.(rspContentMutable); ok {
		tmp.AddRspContentModifier(&filterContent{pattern: pattern, replacement: repl})
		return underlying, nil
	}

	return nil, errors.New("underlying action dos not support filter-content")
}
