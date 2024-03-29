package middleware

// Derived from Goa project, MIT Licensed
// https://github.com/goadesign/goa/blob/v3/http/middleware/requestid.go

import (
	"context"
	"net/http"

	"github.com/gofrs/uuid/v5"
)

// Key to use when setting the request ID.
type ctxKeyRequestID int

// RequestIDKey is the key that holds the unique request ID in a request context.
const RequestIDKey ctxKeyRequestID = 0

// RequestID is a middleware that injects a request ID into the context of each
// request.
func RequestID(options ...RequestIDOption) func(http.Handler) http.Handler {
	o := newRequestIDOptions(options...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			var id string

			if o.UseRequestID() {
				id = r.Header.Get("X-Request-Id")
				if o.requestIDLimit > 0 && len(id) > o.requestIDLimit {
					id = id[:o.requestIDLimit]
				}
			}

			if id == "" {
				id = uuid.Must(uuid.NewV4()).String()[:6]
			}

			ctx = context.WithValue(ctx, RequestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetReqID returns a request ID from the given context if one is present.
// Returns the empty string if a request ID cannot be found.
func GetReqID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}

func UseXRequestIDHeaderOption(f bool) RequestIDOption {
	return func(o *RequestIDOptions) *RequestIDOptions {
		o.useXRequestID = f
		return o
	}
}

func XRequestIDLimitOption(limit int) RequestIDOption {
	return func(o *RequestIDOptions) *RequestIDOptions {
		o.requestIDLimit = limit
		return o
	}
}

type (
	RequestIDOption func(*RequestIDOptions) *RequestIDOptions

	RequestIDOptions struct {
		// useXRequestID enabled the use of the X-Request-Id request header as
		// the request ID.
		useXRequestID bool

		// requestIDLimit is the maximum length of the X-Request-Id header
		// allowed. Values longer than this value are truncated. Zero value
		// means no limit.
		requestIDLimit int
	}
)

func newRequestIDOptions(options ...RequestIDOption) *RequestIDOptions {
	o := new(RequestIDOptions)
	for _, opt := range options {
		o = opt(o)
	}
	return o
}

func (o *RequestIDOptions) UseRequestID() bool {
	return o.useXRequestID
}
