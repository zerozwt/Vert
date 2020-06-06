package env

import (
	"net/http"
)

type ctxValue struct {
	host string
}

func Host(req *http.Request) string {
	ctx := req.Context()
	if ctx == nil {
		return ""
	}

	value := ctx.Value(VERT_CONTEXT_KEY)
	if value == nil {
		return ""
	}

	if ret, ok := value.(*ctxValue); ok {
		return ret.host
	}

	return ""
}
