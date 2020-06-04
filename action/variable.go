package action

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type Variable interface {
	Parse(*http.Request) string
}

//-----------------------------------------------------------------------------

type vChain struct {
	v_list []Variable
}

func (self *vChain) Parse(req *http.Request) string {
	tmp := make([]string, 0, len(self.v_list))

	for _, item := range self.v_list {
		tmp = append(tmp, item.Parse(req))
	}

	return strings.Join(tmp, "")
}

//-----------------------------------------------------------------------------

type vEncode struct {
	underlying Variable
}

func (self vEncode) Parse(req *http.Request) string {
	return url.QueryEscape(self.underlying.Parse(req))
}

//-----------------------------------------------------------------------------

type vConst string

func (self vConst) Parse(*http.Request) string { return string(self) }

//-----------------------------------------------------------------------------

type vPath struct {
	from int
	to   int
}

func (self *vPath) Parse(req *http.Request) string {
	if self.from < 0 && self.to < 0 {
		return req.URL.Path
	}

	from := self.from
	to := self.to

	if from < 0 {
		from = 0
	} else if from >= len(req.URL.Path) {
		from = len(req.URL.Path)
	}

	if to < 0 || to >= len(req.URL.Path) {
		to = len(req.URL.Path)
	}

	if from >= to {
		return ""
	}

	return req.URL.Path[from:to]
}

//-----------------------------------------------------------------------------

type vPathSeg struct {
	idx  int
	from int
	to   int
}

func (self *vPathSeg) Parse(req *http.Request) string {
	path := req.URL.Path
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	segments := strings.Split(path, "/")

	if self.idx >= 0 {
		if self.idx >= len(segments) {
			return ""
		}
		return segments[self.idx]
	}

	from := self.from
	to := self.to

	if from < 0 {
		from = 0
	} else if from >= len(segments) {
		from = len(segments)
	}

	if to < 0 || to >= len(segments) {
		to = len(segments)
	}

	if from >= to {
		return ""
	}

	return strings.Join(segments[from:to], "/")
}

//-----------------------------------------------------------------------------

type vHasQuery bool

func (self vHasQuery) Parse(req *http.Request) string {
	if len(req.URL.Query()) > 0 {
		return "?"
	}
	return ""
}

//-----------------------------------------------------------------------------

type vQueryAll bool

func (self vQueryAll) Parse(req *http.Request) string {
	return req.URL.RawQuery
}

//-----------------------------------------------------------------------------

type vQuerySingle string

func (self vQuerySingle) Parse(req *http.Request) string {
	return req.URL.Query().Get(string(self))
}

//-----------------------------------------------------------------------------

type vQueryList struct {
	reverse  bool
	key_list []string
}

func (self *vQueryList) Parse(req *http.Request) string {
	tmp := req.URL.Query()
	if len(tmp) == 0 {
		return ""
	}

	ret := make([]string, 0)

	if !self.reverse {
		for _, key := range self.key_list {
			if _, ok := tmp[key]; ok {
				ret = append(ret, key+"="+url.QueryEscape(tmp.Get(key)))
			}
		}
	} else {
		key_map := make(map[string]bool)
		for _, key := range self.key_list {
			key_map[key] = true
		}
		for key := range tmp {
			if _, ok := key_map[key]; !ok {
				ret = append(ret, key+"="+url.QueryEscape(tmp.Get(key)))
			}
		}
	}

	return strings.Join(ret, "&")
}

//-----------------------------------------------------------------------------

type vHasFragment struct{}

func (self vHasFragment) Parse(req *http.Request) string {
	if len(req.URL.Fragment) > 0 {
		return "#"
	}
	return ""
}

//-----------------------------------------------------------------------------

type vFragment struct{}

func (self vFragment) Parse(req *http.Request) string {
	return req.URL.Fragment
}

//-----------------------------------------------------------------------------

type vMuxVar string

func (self vMuxVar) Parse(req *http.Request) string {
	tmp := mux.Vars(req)
	if len(tmp) == 0 {
		return ""
	}

	ret, ok := tmp[string(self)]
	if !ok {
		return ""
	}

	return ret
}

//-----------------------------------------------------------------------------

type vReVar int

func (self vReVar) Parse(req *http.Request) string {
	return fmt.Sprintf("${%d}", int(self))
}

//-----------------------------------------------------------------------------

var matchVar *regexp.Regexp = regexp.MustCompile(`\{(%?)([a-z_\^]+)(:?)([^\}]*)\}`)

var matchKey *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
var matchKeyInt *regexp.Regexp = regexp.MustCompile(`^\[[0-9]+\]$`)
var matchFromTo *regexp.Regexp = regexp.MustCompile(`^\[[0-9]*:[0-9]*\]$`)
var matchKetList *regexp.Regexp = regexp.MustCompile(`^\[[a-zA-Z0-9_\-]+(?:,[a-zA-Z0-9_\-]+)*\]$`)

func convertActionParam(param string) (Variable, error) {
	ret := &vChain{v_list: make([]Variable, 0)}
	param_data := []byte(param)
	var_pos_list := matchVar.FindAllSubmatchIndex(param_data, -1)

	if len(var_pos_list) == 0 {
		return vConst(param), nil
	}

	last_idx := 0

	for _, item := range var_pos_list {
		if item[0] > last_idx {
			ret.v_list = append(ret.v_list, vConst(param[last_idx:item[0]]))
		}
		last_idx = item[1]

		need_encode := item[2] != item[3]
		cmd_name := param[item[4]:item[5]]
		has_param := item[6] != item[7]
		param_raw := param[item[8]:item[9]]

		tmp, err := buildVar(cmd_name, has_param, param_raw)
		if err != nil {
			return nil, err
		}

		if need_encode {
			ret.v_list = append(ret.v_list, vEncode{underlying: tmp})
		} else {
			ret.v_list = append(ret.v_list, tmp)
		}
	}

	if len(param) > last_idx {
		ret.v_list = append(ret.v_list, vConst(param[last_idx:]))
	}

	if len(ret.v_list) == 1 {
		return ret.v_list[0], nil
	}

	return ret, nil
}

func buildVar(cmd_name string, has_param bool, param_raw string) (Variable, error) {
	var err error
	if cmd_name == "path" {
		if has_param {
			return nil, errors.New("'path' variable cannot have ':'")
		}

		if len(param_raw) == 0 {
			return &vPath{-1, -1}, nil
		}

		if !matchFromTo.MatchString(param_raw) {
			return nil, errors.New("invalid param for variable 'path': " + param_raw)
		}

		ret := &vPath{}
		ret.from, ret.to, err = buildFromTo(param_raw)
		return ret, err
	}

	if cmd_name == "seg" {
		if has_param {
			return nil, errors.New("'seg' variable cannot have ':'")
		}

		if matchKeyInt.MatchString(param_raw) {
			ret, err := strconv.Atoi(param_raw[1 : len(param_raw)-1])
			return &vPathSeg{idx: ret}, err
		}

		if !matchFromTo.MatchString(param_raw) {
			return nil, errors.New("invalid param for variable 'seg': " + param_raw)
		}

		ret := &vPathSeg{idx: -1}
		ret.from, ret.to, err = buildFromTo(param_raw)
		return ret, err
	}

	if cmd_name == "has_query" {
		if has_param || len(param_raw) > 0 {
			return nil, errors.New("'has_query' variable cannot have ':' or params")
		}
		return vHasQuery(false), nil
	}

	if cmd_name == "query" {
		if !has_param && len(param_raw) == 0 {
			return vQueryAll(false), nil
		}
		if !has_param || len(param_raw) == 0 {
			return nil, errors.New("malformat 'query' variable")
		}

		if matchKey.MatchString(param_raw) {
			return vQuerySingle(param_raw), nil
		}

		if !matchKetList.MatchString(param_raw) {
			return nil, errors.New("invalid param for 'query' variable: " + param_raw)
		}

		ret := &vQueryList{key_list: strings.Split(param_raw[1:len(param_raw)-1], ",")}
		return ret, nil
	}

	if cmd_name == "^query" {
		if !has_param || len(param_raw) == 0 {
			return nil, errors.New("malformat '^query' variable")
		}

		if !matchKetList.MatchString(param_raw) {
			return nil, errors.New("invalid param for 'query' variable: " + param_raw)
		}

		ret := &vQueryList{reverse: true, key_list: strings.Split(param_raw[1:len(param_raw)-1], ",")}
		return ret, nil
	}

	if cmd_name == "has_fragment" {
		if has_param || len(param_raw) > 0 {
			return nil, errors.New("'has_fragment' variable cannot have ':' or params")
		}
		return vHasFragment{}, nil
	}

	if cmd_name == "fragment" {
		if has_param || len(param_raw) > 0 {
			return nil, errors.New("'fragment' variable cannot have ':' or params")
		}
		return vFragment{}, nil
	}

	if cmd_name == "mux" {
		if !has_param || len(param_raw) == 0 {
			return nil, errors.New("malformat 'mux' variable")
		}

		if matchKey.MatchString(param_raw) {
			return vMuxVar(param_raw), nil
		}

		return nil, errors.New("malformat 'mux' variable")
	}

	if cmd_name == "re" {
		if has_param {
			return nil, errors.New("'re' variable cannot have ':'")
		}

		if matchKeyInt.MatchString(param_raw) {
			ret, err := strconv.Atoi(param_raw[1 : len(param_raw)-1])
			return vReVar(ret), err
		}

		return nil, errors.New("malformat 're' variable")
	}

	return nil, errors.New("Unsupported variable cmd: " + cmd_name)
}

func buildFromTo(param_raw string) (int, int, error) {
	idx := strings.Index(param_raw, ":")
	from_str := param_raw[1:idx]
	to_str := param_raw[idx+1 : len(param_raw)-1]

	from := -1
	to := -1
	var err error

	if len(from_str) > 0 {
		from, err = strconv.Atoi(from_str)
		if err != nil {
			return 0, 0, err
		}
	}

	if len(to_str) > 0 {
		to, err = strconv.Atoi(to_str)
		if err != nil {
			return 0, 0, err
		}
	}

	return from, to, nil
}
