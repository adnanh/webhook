package hook

import "testing"

var extractParameterTests = []struct {
	s      string
	params map[string]interface{}
	value  string
	ok     bool
}{
	{"a", map[string]interface{}{"a": "z"}, "z", true},
	{"a.b", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "z", true},
	{"a.b.c", map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": "z"}}}, "z", true},
	{"a.1.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "z", true},
	{"a.1.b.c", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": map[string]interface{}{"c": "y"}}, map[string]interface{}{"b": map[string]interface{}{"c": "z"}}}}, "z", true},
	// failures
	{"a.X", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},
	{"a.500.b", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},
	{"a.501.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false},
}

func TestExtractParameter(t *testing.T) {
	for _, tt := range extractParameterTests {
		s, ok := ExtractParameter(tt.s, tt.params)
		if ok != tt.ok || s != tt.value {
			t.Errorf("failed to extract parameter %q:\nexpected {value:%#v, ok:%#v},\ngot {value:%#v, ok:%#v}", tt.s, tt.value, tt.ok, s, ok)
		}
	}
}
