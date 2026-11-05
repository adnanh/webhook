package network

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWebhookInProcessWithoutListener(t *testing.T) {
	conn := serveWebhookInProcess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "127.0.0.1:0" || r.Host != "public.example.test" || r.URL.RawQuery != "a=1" {
			http.Error(w, "request metadata changed", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write(body)
	}))
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	req, err := http.NewRequest(http.MethodPost, "http://public.example.test/hooks/deploy?a=1", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "payload" {
		t.Fatalf("response %d: %q", response.StatusCode, body)
	}
}
