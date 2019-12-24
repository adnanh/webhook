package middleware

// Derived from from the Goa project, MIT Licensed
// https://github.com/goadesign/goa/blob/v3/http/middleware/debug.go

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"
)

// responseDupper tees the response to a buffer and a response writer.
type responseDupper struct {
	http.ResponseWriter
	Buffer *bytes.Buffer
	Status int
}

// Dumper returns a debug middleware which prints detailed information about
// incoming requests and outgoing responses including all headers, parameters
// and bodies.
func Dumper(w io.Writer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			buf := &bytes.Buffer{}
			// Request ID
			rid := r.Context().Value(RequestIDKey)

			// Request URL
			buf.WriteString(fmt.Sprintf("> [%s] %s %s", rid, r.Method, r.URL.String()))

			// Request Headers
			keys := make([]string, len(r.Header))
			i := 0
			for k := range r.Header {
				keys[i] = k
				i++
			}
			sort.Strings(keys)
			for _, k := range keys {
				buf.WriteString(fmt.Sprintf("\n> [%s] %s: %s", rid, k, strings.Join(r.Header[k], ", ")))
			}

			// Request parameters
			params := mux.Vars(r)
			keys = make([]string, len(params))
			i = 0
			for k := range params {
				keys[i] = k
				i++
			}
			sort.Strings(keys)
			for _, k := range keys {
				buf.WriteString(fmt.Sprintf("\n> [%s] %s: %s", rid, k, strings.Join(r.Header[k], ", ")))
			}

			// Request body
			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				b = []byte("failed to read body: " + err.Error())
			}
			if len(b) > 0 {
				buf.WriteByte('\n')
				lines := strings.Split(string(b), "\n")
				for _, line := range lines {
					buf.WriteString(fmt.Sprintf("> [%s] %s\n", rid, line))
				}
			}
			r.Body = ioutil.NopCloser(bytes.NewBuffer(b))

			dupper := &responseDupper{ResponseWriter: rw, Buffer: &bytes.Buffer{}}
			h.ServeHTTP(dupper, r)

			buf.WriteString(fmt.Sprintf("\n< [%s] %s", rid, http.StatusText(dupper.Status)))
			keys = make([]string, len(dupper.Header()))
			i = 0
			for k := range dupper.Header() {
				keys[i] = k
				i++
			}
			sort.Strings(keys)
			for _, k := range keys {
				buf.WriteString(fmt.Sprintf("\n< [%s] %s: %s", rid, k, strings.Join(dupper.Header()[k], ", ")))
			}
			if dupper.Buffer.Len() > 0 {
				buf.WriteByte('\n')
				lines := strings.Split(dupper.Buffer.String(), "\n")
				for _, line := range lines {
					buf.WriteString(fmt.Sprintf("< [%s] %s\n", rid, line))
				}
			}
			buf.WriteByte('\n')
			w.Write(buf.Bytes())
		})
	}
}

// Write writes the data to the buffer and connection as part of an HTTP reply.
func (r *responseDupper) Write(b []byte) (int, error) {
	r.Buffer.Write(b)
	return r.ResponseWriter.Write(b)
}

// WriteHeader records the status and sends an HTTP response header with status code.
func (r *responseDupper) WriteHeader(s int) {
	r.Status = s
	r.ResponseWriter.WriteHeader(s)
}

// Hijack supports the http.Hijacker interface.
func (r *responseDupper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("dumper middleware: inner ResponseWriter cannot be hijacked: %T", r.ResponseWriter)
}
