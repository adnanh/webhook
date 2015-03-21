package hook

import (
	"reflect"
	"testing"
)

var checkPayloadSignatureTests = []struct {
	payload   []byte
	secret    string
	signature string
	mac       string
	ok        bool
}{
	{[]byte(`{"a": "z"}`), "secret", "b17e04cbb22afa8ffbff8796fc1894ed27badd9e", "b17e04cbb22afa8ffbff8796fc1894ed27badd9e", true},
	{[]byte(`{"a": "z"}`), "secret", "sha1=b17e04cbb22afa8ffbff8796fc1894ed27badd9e", "b17e04cbb22afa8ffbff8796fc1894ed27badd9e", true},
	// failures
	{[]byte(`{"a": "z"}`), "secret", "XXXe04cbb22afa8ffbff8796fc1894ed27badd9e", "b17e04cbb22afa8ffbff8796fc1894ed27badd9e", false},
	{[]byte(`{"a": "z"}`), "secreX", "b17e04cbb22afa8ffbff8796fc1894ed27badd9e", "900225703e9342328db7307692736e2f7cc7b36e", false},
}

func TestCheckPayloadSignature(t *testing.T) {
	for _, tt := range checkPayloadSignatureTests {
		mac, ok := CheckPayloadSignature(tt.payload, tt.secret, tt.signature)
		if ok != tt.ok || mac != tt.mac {
			t.Errorf("failed to check payload signature {%q, %q, %q}:\nexpected {mac:%#v, ok:%#v},\ngot {mac:%#v, ok:%#v}", tt.payload, tt.secret, tt.signature, tt.mac, tt.ok, mac, ok)
		}
	}
}

var extractParameterTests = []struct {
	s      string
	params interface{}
	value  string
	ok     bool
}{
	{"a", map[string]interface{}{"a": "z"}, "z", true},
	{"a.b", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "z", true},
	{"a.b.c", map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": "z"}}}, "z", true},
	{"a.b.0", map[string]interface{}{"a": map[string]interface{}{"b": []interface{}{"x", "y", "z"}}}, "x", true},
	{"a.1.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "z", true},
	{"a.1.b.c", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": map[string]interface{}{"c": "y"}}, map[string]interface{}{"b": map[string]interface{}{"c": "z"}}}}, "z", true},
	// failures
	{"check_nil", nil, "", false},
	{"a.X", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},                                                      // non-existent parameter reference
	{"a.X.c", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false},   // non-integer slice index
	{"a.-1.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false},  // negative slice index
	{"a.500.b", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},                                                  // non-existent slice
	{"a.501.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false}, // non-existent slice index
	{"a.502.b", map[string]interface{}{"a": []interface{}{}}, "", false},                                                                   // non-existent slice index
}

func TestExtractParameter(t *testing.T) {
	for _, tt := range extractParameterTests {
		value, ok := ExtractParameterAsString(tt.s, tt.params)
		if ok != tt.ok || value != tt.value {
			t.Errorf("failed to extract parameter %q:\nexpected {value:%#v, ok:%#v},\ngot {value:%#v, ok:%#v}", tt.s, tt.value, tt.ok, value, ok)
		}
	}
}

var argumentGetTests = []struct {
	source, name            string
	headers, query, payload *map[string]interface{}
	value                   string
	ok                      bool
}{
	{"header", "a", &map[string]interface{}{"a": "z"}, nil, nil, "z", true},
	{"url", "a", nil, &map[string]interface{}{"a": "z"}, nil, "z", true},
	{"payload", "a", nil, nil, &map[string]interface{}{"a": "z"}, "z", true},
	// failures
	{"header", "a", nil, &map[string]interface{}{"a": "z"}, &map[string]interface{}{"a": "z"}, "", false},  // nil headers
	{"url", "a", &map[string]interface{}{"a": "z"}, nil, &map[string]interface{}{"a": "z"}, "", false},     // nil query
	{"payload", "a", &map[string]interface{}{"a": "z"}, &map[string]interface{}{"a": "z"}, nil, "", false}, // nil payload
	{"foo", "a", &map[string]interface{}{"a": "z"}, nil, nil, "", false},                                   // invalid source
}

func TestArgumentGet(t *testing.T) {
	for _, tt := range argumentGetTests {
		a := Argument{tt.source, tt.name}
		value, ok := a.Get(tt.headers, tt.query, tt.payload)
		if ok != tt.ok || value != tt.value {
			t.Errorf("failed to get {%q, %q}:\nexpected {value:%#v, ok:%#v},\ngot {value:%#v, ok:%#v}", tt.source, tt.name, tt.value, tt.ok, value, ok)
		}
	}
}

var hookParseJSONParametersTests = []struct {
	params                     []Argument
	headers, query, payload    *map[string]interface{}
	rheaders, rquery, rpayload *map[string]interface{}
}{
	{[]Argument{Argument{"header", "a"}}, &map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, nil, nil},
	{[]Argument{Argument{"url", "a"}}, nil, &map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, nil},
	{[]Argument{Argument{"payload", "a"}}, nil, nil, &map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"a": map[string]interface{}{"b": "y"}}},
	{[]Argument{Argument{"header", "z"}}, &map[string]interface{}{"z": `{}`}, nil, nil, &map[string]interface{}{"z": map[string]interface{}{}}, nil, nil},
	// failures
	{[]Argument{Argument{"header", "z"}}, &map[string]interface{}{"z": ``}, nil, nil, &map[string]interface{}{"z": ``}, nil, nil},     // empty string
	{[]Argument{Argument{"header", "y"}}, &map[string]interface{}{"X": `{}`}, nil, nil, &map[string]interface{}{"X": `{}`}, nil, nil}, // missing parameter
}

func TestHookParseJSONParameters(t *testing.T) {
	for _, tt := range hookParseJSONParametersTests {
		h := &Hook{JSONStringParameters: tt.params}
		h.ParseJSONParameters(tt.headers, tt.query, tt.payload)
		if !reflect.DeepEqual(tt.headers, tt.rheaders) {
			t.Errorf("failed to parse %v:\nexpected %#v,\ngot %#v", tt.params, *tt.rheaders, *tt.headers)
		}
	}
}

var hookExtractCommandArgumentsTests = []struct {
	exec                    string
	args                    []Argument
	headers, query, payload *map[string]interface{}
	value                   []string
}{
	{"test", []Argument{Argument{"header", "a"}}, &map[string]interface{}{"a": "z"}, nil, nil, []string{"test", "z"}},
	// failures
	{"fail", []Argument{Argument{"payload", "a"}}, &map[string]interface{}{"a": "z"}, nil, nil, []string{"fail", ""}},
}

func TestHookExtractCommandArguments(t *testing.T) {
	for _, tt := range hookExtractCommandArgumentsTests {
		h := &Hook{ExecuteCommand: tt.exec, PassArgumentsToCommand: tt.args}
		value := h.ExtractCommandArguments(tt.headers, tt.query, tt.payload)
		if !reflect.DeepEqual(value, tt.value) {
			t.Errorf("failed to extract args {cmd=%q, args=%v}:\nexpected %#v,\ngot %#v", tt.exec, tt.args, tt.value, value)
		}
	}
}

var hooksLoadFromFileTests = []struct {
	path string
	ok   bool
}{
	{"../hooks.json.example", true},
	{"", true},
	// failures
	{"missing.json", false},
}

func TestHooksLoadFromFile(t *testing.T) {
	for _, tt := range hooksLoadFromFileTests {
		h := &Hooks{}
		err := h.LoadFromFile(tt.path)
		if (err == nil) != tt.ok {
			t.Errorf(err.Error())
		}
	}
}

var hooksMatchTests = []struct {
	id    string
	hooks Hooks
	value *Hook
}{
	{"a", Hooks{Hook{ID: "a"}}, &Hook{ID: "a"}},
	{"X", Hooks{Hook{ID: "a"}}, new(Hook)},
}

func TestHooksMatch(t *testing.T) {
	for _, tt := range hooksMatchTests {
		value := tt.hooks.Match(tt.id)
		if reflect.DeepEqual(reflect.ValueOf(value), reflect.ValueOf(tt.value)) {
			t.Errorf("failed to match %q:\nexpected %#v,\ngot %#v", tt.id, tt.value, value)
		}
	}
}

var matchRuleTests = []struct {
	typ, regex, secret, value string
	param                     Argument
	headers, query, payload   *map[string]interface{}
	body                      []byte
	ok                        bool
}{
	{"value", "", "", "z", Argument{"header", "a"}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, true},
	{"regex", "^z", "", "z", Argument{"header", "a"}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, true},
	{"payload-hash-sha1", "", "secret", "", Argument{"header", "a"}, &map[string]interface{}{"a": "b17e04cbb22afa8ffbff8796fc1894ed27badd9e"}, nil, nil, []byte(`{"a": "z"}`), true},
	// failures
	{"value", "", "", "X", Argument{"header", "a"}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, false},
	{"value", "", "", "X", Argument{"header", "a"}, &map[string]interface{}{"y": "z"}, nil, nil, []byte{}, false},
	{"regex", "^X", "", "", Argument{"header", "a"}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, false},
	{"regex", "*", "", "", Argument{"header", "a"}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, false},
	{"payload-hash-sha1", "", "secret", "", Argument{"header", "a"}, &map[string]interface{}{"a": ""}, nil, nil, []byte{}, false},
}

func TestMatchRule(t *testing.T) {
	for _, tt := range matchRuleTests {
		r := MatchRule{tt.typ, tt.regex, tt.secret, tt.value, tt.param}
		ok := r.Evaluate(tt.headers, tt.query, tt.payload, &tt.body)
		if ok != tt.ok {
			t.Errorf("failed to match %#v:\nexpected %#v,\ngot %#v", r, tt.ok, ok)
		}
	}
}

var andRuleTests = []struct {
	desc                    string // description of the test case
	rule                    AndRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
}{
	{
		"(a=z, b=y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
		},
		&map[string]interface{}{"a": "z", "b": "y"}, nil, nil, []byte{},
		true,
	},
	{
		"(a=z, b=Y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
		},
		&map[string]interface{}{"a": "z", "b": "Y"}, nil, nil, []byte{},
		false,
	},
	// Complex test to cover Rules.Evaluate
	{
		"(a=z, b=y, c=x, d=w=, e=X, f=X): a=z && (b=y && c=x) && (d=w || e=v) && !f=u",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{
				And: &AndRule{
					{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
					{Match: &MatchRule{"value", "", "", "x", Argument{"header", "c"}}},
				},
			},
			{
				Or: &OrRule{
					{Match: &MatchRule{"value", "", "", "w", Argument{"header", "d"}}},
					{Match: &MatchRule{"value", "", "", "v", Argument{"header", "e"}}},
				},
			},
			{
				Not: &NotRule{
					Match: &MatchRule{"value", "", "", "u", Argument{"header", "f"}},
				},
			},
		},
		&map[string]interface{}{"a": "z", "b": "y", "c": "x", "d": "w", "e": "X", "f": "X"}, nil, nil, []byte{},
		true,
	},
	{"empty rule", AndRule{{}}, nil, nil, nil, nil, false},
}

func TestAndRule(t *testing.T) {
	for _, tt := range andRuleTests {
		ok := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body)
		if ok != tt.ok {
			t.Errorf("failed to match %#v:\nexpected %#v,\ngot %#v", tt.desc, tt.ok, ok)
		}
	}
}

var orRuleTests = []struct {
	desc                    string // description of the test case
	rule                    OrRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
}{
	{
		"(a=z, b=X): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
		},
		&map[string]interface{}{"a": "z", "b": "X"}, nil, nil, []byte{},
		true,
	},
	{
		"(a=X, b=y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
		},
		&map[string]interface{}{"a": "X", "b": "y"}, nil, nil, []byte{},
		true,
	},
	{
		"(a=Z, b=Y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b"}}},
		},
		&map[string]interface{}{"a": "Z", "b": "Y"}, nil, nil, []byte{},
		false,
	},
}

func TestOrRule(t *testing.T) {
	for _, tt := range orRuleTests {
		ok := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body)
		if ok != tt.ok {
			t.Errorf("%#v:\nexpected %#v,\ngot %#v", tt.desc, tt.ok, ok)
		}
	}
}

var notRuleTests = []struct {
	desc                    string // description of the test case
	rule                    NotRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
}{
	{"(a=z): !a=X", NotRule{Match: &MatchRule{"value", "", "", "X", Argument{"header", "a"}}}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, true},
	{"(a=z): !a=z", NotRule{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a"}}}, &map[string]interface{}{"a": "z"}, nil, nil, []byte{}, false},
}

func TestNotRule(t *testing.T) {
	for _, tt := range notRuleTests {
		ok := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body)
		if ok != tt.ok {
			t.Errorf("failed to match %#v:\nexpected %#v,\ngot %#v", tt.rule, tt.ok, ok)
		}
	}
}
