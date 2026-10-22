package main

import (
	"testing"

	"github.com/adnanh/webhook/internal/hook"
)

func TestParseRequestPayloadContentTypesAreCaseInsensitive(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		payloadKey  string
		payloadVal  interface{}
	}{
		{"json", "Application/JSON; charset=UTF-8", `{"kind":"synthetic"}`, "kind", "synthetic"},
		{"form", "APPLICATION/X-WWW-FORM-URLENCODED; charset=UTF-8", "kind=synthetic", "kind", "synthetic"},
		{"xml", "APPLICATION/XML; charset=UTF-8", `<root><kind>synthetic</kind></root>`, "root", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &hook.Request{ContentType: tt.contentType, Body: []byte(tt.body)}
			parsed, err := parseRequestPayload(req)
			if err != nil {
				t.Fatalf("parseRequestPayload returned error: %v", err)
			}
			if !parsed {
				t.Fatal("valid media type was not recognized")
			}
			if req.Payload == nil {
				t.Fatal("payload was not parsed")
			}
			if tt.payloadVal != nil && req.Payload[tt.payloadKey] != tt.payloadVal {
				t.Fatalf("payload[%q] = %#v, want %#v", tt.payloadKey, req.Payload[tt.payloadKey], tt.payloadVal)
			}
		})
	}
}

func TestMultipartContentTypeIsCaseInsensitive(t *testing.T) {
	if !isMultipartContentType("Multipart/Form-Data; boundary=synthetic") {
		t.Fatal("valid multipart media type was not recognized")
	}
	if isMultipartContentType("application/json") {
		t.Fatal("non-multipart media type was recognized as multipart")
	}
}

func TestParseRequestPayloadLeavesUnsupportedTypeUnparsed(t *testing.T) {
	req := &hook.Request{ContentType: "text/plain", Body: []byte("synthetic")}
	parsed, err := parseRequestPayload(req)
	if err != nil {
		t.Fatalf("unsupported media type returned error: %v", err)
	}
	if parsed || req.Payload != nil {
		t.Fatalf("unsupported media type parsed payload: parsed=%v payload=%#v", parsed, req.Payload)
	}
}
