package action

import (
	"errors"
	"net/http"
	"strings"
)

type actionFunc func([]string, http.Handler) (http.Handler, error)

var gActionBuilder map[string]actionFunc = make(map[string]actionFunc)

func registerActionFunc(cmd string, builder actionFunc) {
	gActionBuilder[cmd] = builder
}

func ActionHandler(action string, underlying http.Handler) (http.Handler, error) {
	cmd, params, err := compileAction(action)
	if err != nil {
		return nil, err
	}

	if builder, ok := gActionBuilder[cmd]; ok {
		return builder(params, underlying)
	}

	return nil, errors.New("Invalid action: " + cmd)
}

func compileAction(action string) (string, []string, error) {
	action = strings.Trim(action, " \r\n\t")
	if len(action) == 0 {
		return "", nil, errors.New("Empty action string")
	}

	idx := strings.IndexAny(action, " \r\n\t")
	if idx < 0 {
		return action, nil, nil
	}

	cmd := action[:idx]
	params := splitFields(strings.Trim(action[idx:], " \r\n\t"))

	return cmd, params, nil
}

func splitFields(field string) []string {
	if len(field) == 0 {
		return nil
	}

	ret := make([]string, 0)

	curr := ""
	state := 0
	idx := 0

	for idx < len(field) {
		ch := field[idx]
		switch state {
		case 0:
			idx++
			if isWhitespace(ch) {
				if len(curr) > 0 {
					ret = append(ret, curr)
					curr = ""
				}
				state = 1
			} else if ch == '\\' {
				state = 2
			} else if ch == '\'' {
				state = 3
			} else if ch == '"' {
				state = 4
			} else {
				curr += string(ch)
			}
		case 1:
			if !isWhitespace(ch) {
				state = 0
			} else {
				idx++
			}
		case 2:
			curr += string(ch)
			idx++
			state = 0
		case 3:
			idx++
			if ch == '\'' {
				state = 0
			} else {
				curr += string(ch)
			}
		case 4:
			idx++
			if ch == '"' {
				state = 0
			} else {
				curr += string(ch)
			}
		}
	}

	if len(curr) > 0 {
		ret = append(ret, curr)
		curr = ""
	}

	return ret
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

type Logger interface {
	DEBUG_LOG(string, []interface{})
	INFO_LOG(string, []interface{})
	ERROR_LOG(string, []interface{})
}

var logger Logger = nil

func SetLogger(l Logger) { logger = l }

func DEBUG_LOG(fmt string, args ...interface{}) { logger.DEBUG_LOG(fmt, args) }
func INFO_LOG(fmt string, args ...interface{})  { logger.INFO_LOG(fmt, args) }
func ERROR_LOG(fmt string, args ...interface{}) { logger.ERROR_LOG(fmt, args) }
