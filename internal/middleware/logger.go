package middleware

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5/middleware"
)

// Logger is a middleware that logs useful data about each HTTP request.
type Logger struct {
	Logger middleware.LoggerInterface
}

// NewLogger creates a new RequestLogger Handler.
func NewLogger() func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&Logger{})
}

// NewLogEntry creates a new LogEntry for the request.
func (l *Logger) NewLogEntry(r *http.Request) middleware.LogEntry {
	e := &LogEntry{
		req: r,
		buf: &bytes.Buffer{},
	}

	return e
}

// LogEntry represents an individual log entry.
type LogEntry struct {
	*Logger
	req *http.Request
	buf *bytes.Buffer
}

// Write constructs and writes the final log entry.
func (l *LogEntry) Write(status, totalBytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	rid := GetReqID(l.req.Context())
	if rid != "" {
		fmt.Fprintf(l.buf, "[%s] ", rid)
	}

	fmt.Fprintf(l.buf, "%03d | %s | %s | ", status, humanize.IBytes(uint64(totalBytes)), elapsed)
	l.buf.WriteString(l.req.Host + " | " + l.req.Method + " " + l.req.RequestURI)
	log.Print(l.buf.String())
}

// Panic prints the call stack for a panic.
func (l *LogEntry) Panic(v interface{}, stack []byte) {
	e := l.NewLogEntry(l.req).(*LogEntry)
	fmt.Fprintf(e.buf, "panic: %#v", v)
	log.Print(e.buf.String())
	log.Print(string(stack))
}
