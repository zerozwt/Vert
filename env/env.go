package env

import (
	"context"
	"net/http"
)

const VERT_CONTEXT_KEY string = "vert"

func WrapRequest(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), VERT_CONTEXT_KEY, &ctxValue{host: req.Host})
	return req.WithContext(ctx)
}
