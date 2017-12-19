package hook

import (
	"os"
	"reflect"
	"strings"
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
		mac, err := CheckPayloadSignature(tt.payload, tt.secret, tt.signature)
		if (err == nil) != tt.ok || mac != tt.mac {
			t.Errorf("failed to check payload signature {%q, %q, %q}:\nexpected {mac:%#v, ok:%#v},\ngot {mac:%#v, ok:%#v}", tt.payload, tt.secret, tt.signature, tt.mac, tt.ok, mac, (err == nil))
		}

		if err != nil && strings.Contains(err.Error(), tt.mac) {
			t.Errorf("error message should not disclose expected mac: %s", err)
		}
	}
}

var checkPayloadSignature256Tests = []struct {
	payload   []byte
	secret    string
	signature string
	mac       string
	ok        bool
}{
	{[]byte(`{"a": "z"}`), "secret", "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", true},
	{[]byte(`{"a": "z"}`), "secret", "sha256=f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", true},
	// failures
	{[]byte(`{"a": "z"}`), "secret", "XXX7af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89", false},
}

func TestCheckPayloadSignature256(t *testing.T) {
	for _, tt := range checkPayloadSignature256Tests {
		mac, err := CheckPayloadSignature256(tt.payload, tt.secret, tt.signature)
		if (err == nil) != tt.ok || mac != tt.mac {
			t.Errorf("failed to check payload signature {%q, %q, %q}:\nexpected {mac:%#v, ok:%#v},\ngot {mac:%#v, ok:%#v}", tt.payload, tt.secret, tt.signature, tt.mac, tt.ok, mac, (err == nil))
		}

		if err != nil && strings.Contains(err.Error(), tt.mac) {
			t.Errorf("error message should not disclose expected mac: %s", err)
		}
	}
}

var checkScalrSignatureTests = []struct {
	description       string
	headers           map[string]interface{}
	payload           []byte
	secret            string
	expectedSignature string
	ok                bool
}{
	{
		"Valid signature",
		map[string]interface{}{"Date": "Thu 07 Sep 2017 06:30:04 UTC", "X-Signature": "48e395e38ac48988929167df531eb2da00063a7d"},
		[]byte(`{"a": "b"}`), "bilFGi4ZVZUdG+C6r0NIM9tuRq6PaG33R3eBUVhLwMAErGBaazvXe4Gq2DcJs5q+",
		"48e395e38ac48988929167df531eb2da00063a7d", true,
	},
	{
		"Wrong signature",
		map[string]interface{}{"Date": "Thu 07 Sep 2017 06:30:04 UTC", "X-Signature": "999395e38ac48988929167df531eb2da00063a7d"},
		[]byte(`{"a": "b"}`), "bilFGi4ZVZUdG+C6r0NIM9tuRq6PaG33R3eBUVhLwMAErGBaazvXe4Gq2DcJs5q+",
		"48e395e38ac48988929167df531eb2da00063a7d", false,
	},
	{
		"Missing Date header",
		map[string]interface{}{"X-Signature": "999395e38ac48988929167df531eb2da00063a7d"},
		[]byte(`{"a": "b"}`), "bilFGi4ZVZUdG+C6r0NIM9tuRq6PaG33R3eBUVhLwMAErGBaazvXe4Gq2DcJs5q+",
		"48e395e38ac48988929167df531eb2da00063a7d", false,
	},
	{
		"Missing X-Signature header",
		map[string]interface{}{"Date": "Thu 07 Sep 2017 06:30:04 UTC"},
		[]byte(`{"a": "b"}`), "bilFGi4ZVZUdG+C6r0NIM9tuRq6PaG33R3eBUVhLwMAErGBaazvXe4Gq2DcJs5q+",
		"48e395e38ac48988929167df531eb2da00063a7d", false,
	},
}

func TestCheckScalrSignature(t *testing.T) {
	for _, testCase := range checkScalrSignatureTests {
		valid, err := CheckScalrSignature(testCase.headers, testCase.payload, testCase.secret, false)
		if valid != testCase.ok {
			t.Errorf("failed to check scalr signature fot test case: %s\nexpected ok:%#v, got ok:%#v}",
				testCase.description, testCase.ok, valid)
		}

		if err != nil && strings.Contains(err.Error(), testCase.expectedSignature) {
			t.Errorf("error message should not disclose expected mac: %s on test case %s", err, testCase.description)
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
	{"a.b.503", map[string]interface{}{"a": map[string]interface{}{"b": []interface{}{"x", "y", "z"}}}, "", false},                         // trailing, non-existent slice index
	{"a.b", interface{}("a"), "", false},                                                                                                   // non-map, non-slice input
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
	{"header", "a", &map[string]interface{}{"A": "z"}, nil, nil, "z", true},
	{"url", "a", nil, &map[string]interface{}{"a": "z"}, nil, "z", true},
	{"payload", "a", nil, nil, &map[string]interface{}{"a": "z"}, "z", true},
	{"string", "a", nil, nil, &map[string]interface{}{"a": "z"}, "a", true},
	// failures
	{"header", "a", nil, &map[string]interface{}{"a": "z"}, &map[string]interface{}{"a": "z"}, "", false},  // nil headers
	{"url", "a", &map[string]interface{}{"A": "z"}, nil, &map[string]interface{}{"a": "z"}, "", false},     // nil query
	{"payload", "a", &map[string]interface{}{"A": "z"}, &map[string]interface{}{"a": "z"}, nil, "", false}, // nil payload
	{"foo", "a", &map[string]interface{}{"A": "z"}, nil, nil, "", false},                                   // invalid source
}

func TestArgumentGet(t *testing.T) {
	for _, tt := range argumentGetTests {
		a := Argument{tt.source, tt.name, "", false}
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
	ok                         bool
}{
	{[]Argument{Argument{"header", "a", "", false}}, &map[string]interface{}{"A": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"A": map[string]interface{}{"b": "y"}}, nil, nil, true},
	{[]Argument{Argument{"url", "a", "", false}}, nil, &map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, nil, true},
	{[]Argument{Argument{"payload", "a", "", false}}, nil, nil, &map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, &map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, true},
	{[]Argument{Argument{"header", "z", "", false}}, &map[string]interface{}{"Z": `{}`}, nil, nil, &map[string]interface{}{"Z": map[string]interface{}{}}, nil, nil, true},
	// failures
	{[]Argument{Argument{"header", "z", "", false}}, &map[string]interface{}{"Z": ``}, nil, nil, &map[string]interface{}{"Z": ``}, nil, nil, false},     // empty string
	{[]Argument{Argument{"header", "y", "", false}}, &map[string]interface{}{"X": `{}`}, nil, nil, &map[string]interface{}{"X": `{}`}, nil, nil, false}, // missing parameter
	{[]Argument{Argument{"string", "z", "", false}}, &map[string]interface{}{"Z": ``}, nil, nil, &map[string]interface{}{"Z": ``}, nil, nil, false},     // invalid argument source
}

func TestHookParseJSONParameters(t *testing.T) {
	for _, tt := range hookParseJSONParametersTests {
		h := &Hook{JSONStringParameters: tt.params}
		err := h.ParseJSONParameters(tt.headers, tt.query, tt.payload)
		if (err == nil) != tt.ok || !reflect.DeepEqual(tt.headers, tt.rheaders) {
			t.Errorf("failed to parse %v:\nexpected %#v, ok: %v\ngot %#v, ok: %v", tt.params, *tt.rheaders, tt.ok, *tt.headers, (err == nil))
		}
	}
}

var hookExtractCommandArgumentsTests = []struct {
	exec                    string
	args                    []Argument
	headers, query, payload *map[string]interface{}
	value                   []string
	ok                      bool
}{
	{"test", []Argument{Argument{"header", "a", "", false}}, &map[string]interface{}{"A": "z"}, nil, nil, []string{"test", "z"}, true},
	// failures
	{"fail", []Argument{Argument{"payload", "a", "", false}}, &map[string]interface{}{"A": "z"}, nil, nil, []string{"fail", ""}, false},
}

func TestHookExtractCommandArguments(t *testing.T) {
	for _, tt := range hookExtractCommandArgumentsTests {
		h := &Hook{ExecuteCommand: tt.exec, PassArgumentsToCommand: tt.args}
		value, err := h.ExtractCommandArguments(tt.headers, tt.query, tt.payload)
		if (err == nil) != tt.ok || !reflect.DeepEqual(value, tt.value) {
			t.Errorf("failed to extract args {cmd=%q, args=%v}:\nexpected %#v, ok: %v\ngot %#v, ok: %v", tt.exec, tt.args, tt.value, tt.ok, value, (err == nil))
		}
	}
}

// Here we test the extraction of env variables when the user defined a hook
// with the "pass-environment-to-command" directive
// we test both cases where the name of the data is used as the name of the
// env key & the case where the hook definition sets the env var name to a
// fixed value using the envname construct like so::
//    [
//      {
//        "id": "push",
//        "execute-command": "bb2mm",
//        "command-working-directory": "/tmp",
//        "pass-environment-to-command":
//        [
//          {
//            "source": "entire-payload",
//            "envname": "PAYLOAD"
//          },
//        ]
//      }
//    ]
var hookExtractCommandArgumentsForEnvTests = []struct {
	exec                    string
	args                    []Argument
	headers, query, payload *map[string]interface{}
	value                   []string
	ok                      bool
}{
	// successes
	{
		"test",
		[]Argument{Argument{"header", "a", "", false}},
		&map[string]interface{}{"A": "z"}, nil, nil,
		[]string{"HOOK_a=z"},
		true,
	},
	{
		"test",
		[]Argument{Argument{"header", "a", "MYKEY", false}},
		&map[string]interface{}{"A": "z"}, nil, nil,
		[]string{"MYKEY=z"},
		true,
	},
	// failures
	{
		"fail",
		[]Argument{Argument{"payload", "a", "", false}},
		&map[string]interface{}{"A": "z"}, nil, nil,
		[]string{},
		false,
	},
}

func TestHookExtractCommandArgumentsForEnv(t *testing.T) {
	for _, tt := range hookExtractCommandArgumentsForEnvTests {
		h := &Hook{ExecuteCommand: tt.exec, PassEnvironmentToCommand: tt.args}
		value, err := h.ExtractCommandArgumentsForEnv(tt.headers, tt.query, tt.payload)
		if (err == nil) != tt.ok || !reflect.DeepEqual(value, tt.value) {
			t.Errorf("failed to extract args for env {cmd=%q, args=%v}:\nexpected %#v, ok: %v\ngot %#v, ok: %v", tt.exec, tt.args, tt.value, tt.ok, value, (err == nil))
		}
	}
}

var hooksLoadFromFileTests = []struct {
	path       string
	asTemplate bool
	ok         bool
}{
	{"../hooks.json.example", false, true},
	{"../hooks.yaml.example", false, true},
	{"../hooks.json.tmpl.example", true, true},
	{"../hooks.yaml.tmpl.example", true, true},
	{"", false, true},
	// failures
	{"missing.json", false, false},
}

func TestHooksLoadFromFile(t *testing.T) {
	secret := `foo"123`
	os.Setenv("XXXTEST_SECRET", secret)

	for _, tt := range hooksLoadFromFileTests {
		h := &Hooks{}
		err := h.LoadFromFile(tt.path, tt.asTemplate)
		if (err == nil) != tt.ok {
			t.Errorf(err.Error())
		}
	}
}

func TestHooksTemplateLoadFromFile(t *testing.T) {
	secret := `foo"123`
	os.Setenv("XXXTEST_SECRET", secret)

	for _, tt := range hooksLoadFromFileTests {
		if !tt.asTemplate {
			continue
		}

		h := &Hooks{}
		err := h.LoadFromFile(tt.path, tt.asTemplate)
		if (err == nil) != tt.ok {
			t.Errorf(err.Error())
			continue
		}

		s := (*h.Match("webhook").TriggerRule.And)[0].Match.Secret
		if s != secret {
			t.Errorf("Expected secret of %q, got %q", secret, s)
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
	typ, regex, secret, value, ipRange string
	param                              Argument
	headers, query, payload            *map[string]interface{}
	body                               []byte
	remoteAddr                         string
	ok                                 bool
	err                                bool
}{
	{"value", "", "", "z", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", true, false},
	{"regex", "^z", "", "z", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", true, false},
	{"payload-hash-sha1", "", "secret", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "b17e04cbb22afa8ffbff8796fc1894ed27badd9e"}, nil, nil, []byte(`{"a": "z"}`), "", true, false},
	{"payload-hash-sha256", "", "secret", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89"}, nil, nil, []byte(`{"a": "z"}`), "", true, false},
	// failures
	{"value", "", "", "X", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, false},
	{"regex", "^X", "", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, false},
	{"value", "", "2", "X", "", Argument{"header", "a", "", false}, &map[string]interface{}{"Y": "z"}, nil, nil, []byte{}, "", false, false}, // reference invalid header
	// errors
	{"regex", "*", "", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, true},                   // invalid regex
	{"payload-hash-sha1", "", "secret", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": ""}, nil, nil, []byte{}, "", false, true},   // invalid hmac
	{"payload-hash-sha256", "", "secret", "", "", Argument{"header", "a", "", false}, &map[string]interface{}{"A": ""}, nil, nil, []byte{}, "", false, true}, // invalid hmac
	// IP whitelisting, valid cases
	{"ip-whitelist", "", "", "", "192.168.0.1/24", Argument{}, nil, nil, nil, []byte{}, "192.168.0.2:9000", true, false}, // valid IPv4, with range
	{"ip-whitelist", "", "", "", "192.168.0.1/24", Argument{}, nil, nil, nil, []byte{}, "192.168.0.2:9000", true, false}, // valid IPv4, with range
	{"ip-whitelist", "", "", "", "192.168.0.1", Argument{}, nil, nil, nil, []byte{}, "192.168.0.1:9000", true, false},    // valid IPv4, no range
	{"ip-whitelist", "", "", "", "::1/24", Argument{}, nil, nil, nil, []byte{}, "[::1]:9000", true, false},               // valid IPv6, with range
	{"ip-whitelist", "", "", "", "::1", Argument{}, nil, nil, nil, []byte{}, "[::1]:9000", true, false},                  // valid IPv6, no range
	// IP whitelisting, invalid cases
	{"ip-whitelist", "", "", "", "192.168.0.1/a", Argument{}, nil, nil, nil, []byte{}, "192.168.0.2:9000", false, true},  // invalid IPv4, with range
	{"ip-whitelist", "", "", "", "192.168.0.a", Argument{}, nil, nil, nil, []byte{}, "192.168.0.2:9000", false, true},    // invalid IPv4, no range
	{"ip-whitelist", "", "", "", "192.168.0.1/24", Argument{}, nil, nil, nil, []byte{}, "192.168.0.a:9000", false, true}, // invalid IPv4 address
	{"ip-whitelist", "", "", "", "::1/a", Argument{}, nil, nil, nil, []byte{}, "[::1]:9000", false, true},                // invalid IPv6, with range
	{"ip-whitelist", "", "", "", "::z", Argument{}, nil, nil, nil, []byte{}, "[::1]:9000", false, true},                  // invalid IPv6, no range
	{"ip-whitelist", "", "", "", "::1/24", Argument{}, nil, nil, nil, []byte{}, "[::z]:9000", false, true},               // invalid IPv6 address
}

func TestMatchRule(t *testing.T) {
	for i, tt := range matchRuleTests {
		r := MatchRule{tt.typ, tt.regex, tt.secret, tt.value, tt.param, tt.ipRange}
		ok, err := r.Evaluate(tt.headers, tt.query, tt.payload, &tt.body, tt.remoteAddr)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("%d failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", i, r, tt.ok, tt.err, ok, (err != nil))
		}
	}
}

var andRuleTests = []struct {
	desc                    string // description of the test case
	rule                    AndRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{
		"(a=z, b=y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
		},
		&map[string]interface{}{"A": "z", "B": "y"}, nil, nil, []byte{},
		true, false,
	},
	{
		"(a=z, b=Y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
		},
		&map[string]interface{}{"A": "z", "B": "Y"}, nil, nil, []byte{},
		false, false,
	},
	// Complex test to cover Rules.Evaluate
	{
		"(a=z, b=y, c=x, d=w=, e=X, f=X): a=z && (b=y && c=x) && (d=w || e=v) && !f=u",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{
				And: &AndRule{
					{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
					{Match: &MatchRule{"value", "", "", "x", Argument{"header", "c", "", false}, ""}},
				},
			},
			{
				Or: &OrRule{
					{Match: &MatchRule{"value", "", "", "w", Argument{"header", "d", "", false}, ""}},
					{Match: &MatchRule{"value", "", "", "v", Argument{"header", "e", "", false}, ""}},
				},
			},
			{
				Not: &NotRule{
					Match: &MatchRule{"value", "", "", "u", Argument{"header", "f", "", false}, ""},
				},
			},
		},
		&map[string]interface{}{"A": "z", "B": "y", "C": "x", "D": "w", "E": "X", "F": "X"}, nil, nil, []byte{},
		true, false,
	},
	{"empty rule", AndRule{{}}, nil, nil, nil, nil, false, false},
	// failures
	{
		"invalid rule",
		AndRule{{Match: &MatchRule{"value", "", "", "X", Argument{"header", "a", "", false}, ""}}},
		&map[string]interface{}{"Y": "z"}, nil, nil, nil,
		false, false,
	},
}

func TestAndRule(t *testing.T) {
	for _, tt := range andRuleTests {
		ok, err := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body, "")
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", tt.desc, tt.ok, tt.err, ok, err)
		}
	}
}

var orRuleTests = []struct {
	desc                    string // description of the test case
	rule                    OrRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{
		"(a=z, b=X): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
		},
		&map[string]interface{}{"A": "z", "B": "X"}, nil, nil, []byte{},
		true, false,
	},
	{
		"(a=X, b=y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
		},
		&map[string]interface{}{"A": "X", "B": "y"}, nil, nil, []byte{},
		true, false,
	},
	{
		"(a=Z, b=Y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false}, ""}},
		},
		&map[string]interface{}{"A": "Z", "B": "Y"}, nil, nil, []byte{},
		false, false,
	},
	// failures
	{
		"invalid rule",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}},
		},
		&map[string]interface{}{"Y": "Z"}, nil, nil, []byte{},
		false, false,
	},
}

func TestOrRule(t *testing.T) {
	for _, tt := range orRuleTests {
		ok, err := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body, "")
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("%#v:\nexpected ok: %#v, err: %v\ngot ok: %#v err: %v", tt.desc, tt.ok, tt.err, ok, err)
		}
	}
}

var notRuleTests = []struct {
	desc                    string // description of the test case
	rule                    NotRule
	headers, query, payload *map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{"(a=z): !a=X", NotRule{Match: &MatchRule{"value", "", "", "X", Argument{"header", "a", "", false}, ""}}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, true, false},
	{"(a=z): !a=z", NotRule{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false}, ""}}, &map[string]interface{}{"A": "z"}, nil, nil, []byte{}, false, false},
}

func TestNotRule(t *testing.T) {
	for _, tt := range notRuleTests {
		ok, err := tt.rule.Evaluate(tt.headers, tt.query, tt.payload, &tt.body, "")
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", tt.rule, tt.ok, tt.err, ok, err)
		}
	}
}
