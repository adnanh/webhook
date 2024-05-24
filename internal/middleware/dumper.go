package middleware

// Derived from the Goa project, MIT Licensed
// https://github.com/goadesign/goa/blob/v3/http/middleware/debug.go

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sort"
	"strings"
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

			// Dump request

			bd, err := httputil.DumpRequest(r, true)
			if err != nil {
				buf.WriteString(fmt.Sprintf("[%s] Error dumping request for debugging: %s\n", rid, err))
			}

			sc := bufio.NewScanner(bytes.NewBuffer(bd))
			sc.Split(bufio.ScanLines)
			for sc.Scan() {
				buf.WriteString(fmt.Sprintf("> [%s] ", rid))
				buf.WriteString(sc.Text() + "\n")
			}

			w.Write(buf.Bytes())
			buf.Reset()

			// Dump Response

			dupper := &responseDupper{ResponseWriter: rw, Buffer: &bytes.Buffer{}}
			h.ServeHTTP(dupper, r)

			// Response Status
			buf.WriteString(fmt.Sprintf("< [%s] %d %s\n", rid, dupper.Status, http.StatusText(dupper.Status)))

			// Response Headers
			keys := make([]string, len(dupper.Header()))
			i := 0
			for k := range dupper.Header() {
				keys[i] = k
				i++
			}
			sort.Strings(keys)
			for _, k := range keys {
				buf.WriteString(fmt.Sprintf("< [%s] %s: %s\n", rid, k, strings.Join(dupper.Header()[k], ", ")))
			}

			// Response Body
			if dupper.Buffer.Len() > 0 {
				buf.WriteString(fmt.Sprintf("< [%s]\n", rid))
				sc = bufio.NewScanner(dupper.Buffer)
				sc.Split(bufio.ScanLines)
				for sc.Scan() {
					buf.WriteString(fmt.Sprintf("< [%s] ", rid))
					buf.WriteString(sc.Text() + "\n")
				}
			}
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
