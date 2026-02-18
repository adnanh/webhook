package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"unicode"

	"github.com/clbanning/mxj/v2"
)

// Request represents a webhook request.
type Request struct {
	// The request ID set by the RequestID middleware.
	ID string

	// The Content-Type of the request.
	ContentType string

	// The raw request body.
	Body []byte

	// Headers is a map of the parsed headers.
	Headers map[string]interface{}

	// Query is a map of the parsed URL query values.
	Query map[string]interface{}

	// Payload is a map of the parsed payload.
	Payload map[string]interface{}

	// The underlying HTTP request.
	RawRequest *http.Request

	// Treat signature errors as simple validate failures.
	AllowSignatureErrors bool
}

func (r *Request) ParseJSONPayload() error {
	decoder := json.NewDecoder(bytes.NewReader(r.Body))
	decoder.UseNumber()

	var firstChar byte
	for i := 0; i < len(r.Body); i++ {
		if unicode.IsSpace(rune(r.Body[i])) {
			continue
		}
		firstChar = r.Body[i]
		break
	}

	if firstChar == byte('[') {
		var arrayPayload interface{}
		err := decoder.Decode(&arrayPayload)
		if err != nil {
			return fmt.Errorf("error parsing JSON array payload %+v", err)
		}

		r.Payload = make(map[string]interface{}, 1)
		r.Payload["root"] = arrayPayload
	} else {
		err := decoder.Decode(&r.Payload)
		if err != nil {
			return fmt.Errorf("error parsing JSON payload %+v", err)
		}
	}

	return nil
}

func (r *Request) ParseHeaders(headers map[string][]string) {
	r.Headers = make(map[string]interface{}, len(headers))

	for k, v := range headers {
		if len(v) > 0 {
			r.Headers[k] = v[0]
		}
	}
}

func (r *Request) ParseQuery(query map[string][]string) {
	r.Query = make(map[string]interface{}, len(query))

	for k, v := range query {
		if len(v) > 0 {
			r.Query[k] = v[0]
		}
	}
}

func (r *Request) ParseFormPayload() error {
	fd, err := url.ParseQuery(string(r.Body))
	if err != nil {
		return fmt.Errorf("error parsing form payload %+v", err)
	}

	r.Payload = make(map[string]interface{}, len(fd))

	for k, v := range fd {
		if len(v) > 0 {
			r.Payload[k] = v[0]
		}
	}

	return nil
}

func (r *Request) ParseXMLPayload() error {
	var err error

	r.Payload, err = mxj.NewMapXmlReader(bytes.NewReader(r.Body))
	if err != nil {
		return fmt.Errorf("error parsing XML payload: %+v", err)
	}

	return nil
}
