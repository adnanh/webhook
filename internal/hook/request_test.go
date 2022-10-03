package hook

import (
	"encoding/json"
	"reflect"
	"testing"
)

var parseJSONPayloadTests = []struct {
	body    []byte
	payload map[string]interface{}
	ok      bool
}{
	{[]byte(`[1,2,3]`), map[string]interface{}{"root": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")}}, true},
	{[]byte(`  [1,2,3]`), map[string]interface{}{"root": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")}}, true},
	{[]byte(`{"key": "value"}`), map[string]interface{}{"key": "value"}, true},
	{[]byte(`[1, {"]`), map[string]interface{}(nil), false},
	{[]byte(`{"key": "value}`), map[string]interface{}(nil), false},
}

func TestParseJSONPayload(t *testing.T) {
	for _, tt := range parseJSONPayloadTests {
		r := Request{
			Body: tt.body,
		}
		err := r.ParseJSONPayload()
		if (err == nil) != tt.ok {
			t.Errorf("unexpected result given %q: %s\n", string(tt.body), err)
		}

		if !reflect.DeepEqual(tt.payload, r.Payload) {
			t.Errorf("failed to parse json %q:\nexpected %#v,\ngot %#v", string(tt.body), tt.payload, r.Payload)
		}
	}
}

var parseHeadersTests = []struct {
	headers         map[string][]string
	expectedHeaders map[string]interface{}
}{
	{
		map[string][]string{"header1": {"12"}},
		map[string]interface{}{"header1": "12"},
	},
	{
		map[string][]string{"header1": {"12", "34"}},
		map[string]interface{}{"header1": "12"},
	},
	{
		map[string][]string{"header1": {}},
		map[string]interface{}{},
	},
}

func TestParseHeaders(t *testing.T) {
	for _, tt := range parseHeadersTests {
		r := Request{}
		r.ParseHeaders(tt.headers)

		if !reflect.DeepEqual(tt.expectedHeaders, r.Headers) {
			t.Errorf("failed to parse headers %#v:\nexpected %#v,\ngot %#v", tt.headers, tt.expectedHeaders, r.Headers)
		}
	}
}

var parseQueryTests = []struct {
	query         map[string][]string
	expectedQuery map[string]interface{}
}{
	{
		map[string][]string{"query1": {"12"}},
		map[string]interface{}{"query1": "12"},
	},
	{
		map[string][]string{"query1": {"12", "34"}},
		map[string]interface{}{"query1": "12"},
	},
	{
		map[string][]string{"query1": {}},
		map[string]interface{}{},
	},
}

func TestParseQuery(t *testing.T) {
	for _, tt := range parseQueryTests {
		r := Request{}
		r.ParseQuery(tt.query)

		if !reflect.DeepEqual(tt.expectedQuery, r.Query) {
			t.Errorf("failed to parse query %#v:\nexpected %#v,\ngot %#v", tt.query, tt.expectedQuery, r.Query)
		}
	}
}

var parseFormPayloadTests = []struct {
	body            []byte
	expectedPayload map[string]interface{}
	ok              bool
}{
	{
		[]byte("x=1&y=2"),
		map[string]interface{}{"x": "1", "y": "2"},
		true,
	},
	{
		[]byte("x=1&y=2&y=3"),
		map[string]interface{}{"x": "1", "y": "2"},
		true,
	},
	{
		[]byte(";"),
		map[string]interface{}(nil),
		false,
	},
}

func TestParseFormPayload(t *testing.T) {
	for _, tt := range parseFormPayloadTests {
		r := Request{
			Body: tt.body,
		}
		err := r.ParseFormPayload()
		if (err == nil) != tt.ok {
			t.Errorf("unexpected result given %q: %s\n", string(tt.body), err)
		}

		if !reflect.DeepEqual(tt.expectedPayload, r.Payload) {
			t.Errorf("failed to parse form payload %q:\nexpected %#v,\ngot %#v", string(tt.body), tt.expectedPayload, r.Payload)
		}
	}
}

var parseXMLPayloadTests = []struct {
	body            []byte
	expectedPayload map[string]interface{}
	ok              bool
}{
	{
		[]byte("<x>1</x>"),
		map[string]interface{}{"x": "1"},
		true,
	},
	{
		[]byte("<x>1<x>"),
		map[string]interface{}(nil),
		false,
	},
}

func TestParseXMLPayload(t *testing.T) {
	for _, tt := range parseXMLPayloadTests {
		r := Request{
			Body: tt.body,
		}
		err := r.ParseXMLPayload()
		if (err == nil) != tt.ok {
			t.Errorf("unexpected result given %q: %s\n", string(tt.body), err)
		}

		if !reflect.DeepEqual(tt.expectedPayload, r.Payload) {
			t.Errorf("failed to parse xml %q:\nexpected %#v,\ngot %#v", string(tt.body), tt.expectedPayload, r.Payload)
		}
	}
}
