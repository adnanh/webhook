package hook

import (
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGetParameter(t *testing.T) {
	for _, test := range []struct {
		key    string
		val    interface{}
		expect interface{}
		ok     bool
	}{
		// True
		{"a", map[string]interface{}{"a": "1"}, "1", true},
		{"a.b", map[string]interface{}{"a.b": "1"}, "1", true},
		{"a.c", map[string]interface{}{"a": map[string]interface{}{"c": 2}}, 2, true},
		{"a.1", map[string]interface{}{"a": map[string]interface{}{"1": 3}}, 3, true},
		{"a.1", map[string]interface{}{"a": []interface{}{"a", "b"}}, "b", true},
		{"0", []interface{}{"a", "b"}, "a", true},

		// False
		{"z", map[string]interface{}{"a": "1"}, nil, false},
		{"a.z", map[string]interface{}{"a": map[string]interface{}{"b": 2}}, nil, false},
		{"z.b", map[string]interface{}{"a": map[string]interface{}{"z": 2}}, nil, false},
		{"a.2", map[string]interface{}{"a": []interface{}{"a", "b"}}, nil, false},
	} {
		res, err := GetParameter(test.key, test.val)
		if (err == nil) != test.ok {
			t.Errorf("unexpected result given {%q, %q}: %s\n", test.key, test.val, err)
		}

		if !reflect.DeepEqual(res, test.expect) {
			t.Errorf("failed given {%q, %q}:\nexpected {%#v}\ngot {%#v}\n", test.key, test.val, test.expect, res)
		}
	}
}

var checkScalrSignatureTests = []struct {
	description       string
	headers           map[string]interface{}
	body              []byte
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
	{
		"Missing signing key",
		map[string]interface{}{"Date": "Thu 07 Sep 2017 06:30:04 UTC", "X-Signature": "48e395e38ac48988929167df531eb2da00063a7d"},
		[]byte(`{"a": "b"}`), "",
		"48e395e38ac48988929167df531eb2da00063a7d", false,
	},
}

func TestCheckScalrSignature(t *testing.T) {
	for _, testCase := range checkScalrSignatureTests {
		r := &Request{
			Headers: testCase.headers,
			Body:    testCase.body,
		}
		valid, err := CheckScalrSignature(r, testCase.secret, false)
		if valid != testCase.ok {
			t.Errorf("failed to check scalr signature for test case: %s\nexpected ok:%#v, got ok:%#v}",
				testCase.description, testCase.ok, valid)
		}

		if err != nil && testCase.secret != "" && strings.Contains(err.Error(), testCase.expectedSignature) {
			t.Errorf("error message should not disclose expected mac: %s on test case %s", err, testCase.description)
		}
	}
}

var checkIPWhitelistTests = []struct {
	addr    string
	ipRange string
	expect  bool
	ok      bool
}{
	{"[ 10.0.0.1:1234 ] ", "  10.0.0.1 ", true, true},
	{"[ 10.0.0.1:1234 ] ", "  10.0.0.0 ", false, true},
	{"[ 10.0.0.1:1234 ] ", "  10.0.0.1 10.0.0.1 ", true, true},
	{"[ 10.0.0.1:1234 ] ", "  10.0.0.0/31 ", true, true},
	{" [2001:db8:1:2::1:1234] ", "  2001:db8:1::/48 ", true, true},
	{" [2001:db8:1:2::1:1234] ", "  2001:db8:1::/48 2001:db8:1::/64", true, true},
	{" [2001:db8:1:2::1:1234] ", "  2001:db8:1::/64 ", false, true},
}

func TestCheckIPWhitelist(t *testing.T) {
	for _, tt := range checkIPWhitelistTests {
		result, err := CheckIPWhitelist(tt.addr, tt.ipRange)
		if (err == nil) != tt.ok || result != tt.expect {
			t.Errorf("ip whitelist test failed {%q, %q}:\nwant {expect:%#v, ok:%#v},\ngot {result:%#v, ok:%#v}", tt.addr, tt.ipRange, tt.expect, tt.ok, result, err)
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
	{"b", map[string]interface{}{"b": map[string]interface{}{"z": 1}}, `{"z":1}`, true},
	{"c", map[string]interface{}{"c": []interface{}{"y", "z"}}, `["y","z"]`, true},
	{"d", map[string]interface{}{"d": [2]interface{}{"y", "z"}}, `["y","z"]`, true},
	// failures
	{"check_nil", nil, "", false},
	{"a.X", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},                                                      // non-existent parameter reference
	{"a.X.c", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false},   // non-integer slice index
	{"a.-1.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false},  // negative slice index
	{"a.500.b", map[string]interface{}{"a": map[string]interface{}{"b": "z"}}, "", false},                                                  // non-existent slice
	{"a.501.b", map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": "y"}, map[string]interface{}{"b": "z"}}}, "", false}, // non-existent slice index
	{"a.502.b", map[string]interface{}{"a": []interface{}{}}, "", false},                                                                   // non-existent slice index
	{"a.b.503", map[string]interface{}{"a": map[string]interface{}{"b": []interface{}{"x", "y", "z"}}}, "", false},                         // trailing, non-existent slice index
	{"a.b", interface{}("a"), "", false}, // non-map, non-slice input
}

func TestExtractParameter(t *testing.T) {
	for _, tt := range extractParameterTests {
		value, err := ExtractParameterAsString(tt.s, tt.params)
		if (err == nil) != tt.ok || value != tt.value {
			t.Errorf("failed to extract parameter %q:\nexpected {value:%#v, ok:%#v},\ngot {value:%#v, err:%v}", tt.s, tt.value, tt.ok, value, err)
		}
	}
}

var argumentGetTests = []struct {
	source, name            string
	headers, query, payload map[string]interface{}
	request                 *http.Request
	value                   string
	ok                      bool
}{
	{"header", "a", map[string]interface{}{"A": "z"}, nil, nil, nil, "z", true},
	{"url", "a", nil, map[string]interface{}{"a": "z"}, nil, nil, "z", true},
	{"payload", "a", nil, nil, map[string]interface{}{"a": "z"}, nil, "z", true},
	{"request", "METHOD", nil, nil, map[string]interface{}{"a": "z"}, &http.Request{Method: "POST", RemoteAddr: "127.0.0.1:1234"}, "POST", true},
	{"request", "remote-addr", nil, nil, map[string]interface{}{"a": "z"}, &http.Request{Method: "POST", RemoteAddr: "127.0.0.1:1234"}, "127.0.0.1:1234", true},
	{"string", "a", nil, nil, map[string]interface{}{"a": "z"}, nil, "a", true},
	// failures
	{"header", "a", nil, map[string]interface{}{"a": "z"}, map[string]interface{}{"a": "z"}, nil, "", false},  // nil headers
	{"url", "a", map[string]interface{}{"A": "z"}, nil, map[string]interface{}{"a": "z"}, nil, "", false},     // nil query
	{"payload", "a", map[string]interface{}{"A": "z"}, map[string]interface{}{"a": "z"}, nil, nil, "", false}, // nil payload
	{"foo", "a", map[string]interface{}{"A": "z"}, nil, nil, nil, "", false},                                  // invalid source
}

func TestArgumentGet(t *testing.T) {
	for _, tt := range argumentGetTests {
		a := Argument{tt.source, tt.name, "", false, nil}
		r := &Request{
			Headers:    tt.headers,
			Query:      tt.query,
			Payload:    tt.payload,
			RawRequest: tt.request,
		}
		value, err := a.Get(r)
		if (err == nil) != tt.ok || value != tt.value {
			t.Errorf("failed to get {%q, %q}:\nexpected {value:%#v, ok:%#v},\ngot {value:%#v, err:%v}", tt.source, tt.name, tt.value, tt.ok, value, err)
		}
	}
}

var hookParseJSONParametersTests = []struct {
	params                     []Argument
	headers, query, payload    map[string]interface{}
	rheaders, rquery, rpayload map[string]interface{}
	ok                         bool
}{
	{[]Argument{Argument{"header", "a", "", false, nil}}, map[string]interface{}{"A": `{"b": "y"}`}, nil, nil, map[string]interface{}{"A": map[string]interface{}{"b": "y"}}, nil, nil, true},
	{[]Argument{Argument{"url", "a", "", false, nil}}, nil, map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, nil, true},
	{[]Argument{Argument{"payload", "a", "", false, nil}}, nil, nil, map[string]interface{}{"a": `{"b": "y"}`}, nil, nil, map[string]interface{}{"a": map[string]interface{}{"b": "y"}}, true},
	{[]Argument{Argument{"header", "z", "", false, nil}}, map[string]interface{}{"Z": `{}`}, nil, nil, map[string]interface{}{"Z": map[string]interface{}{}}, nil, nil, true},
	// failures
	{[]Argument{Argument{"header", "z", "", false, nil}}, map[string]interface{}{"Z": ``}, nil, nil, map[string]interface{}{"Z": ``}, nil, nil, false},     // empty string
	{[]Argument{Argument{"header", "y", "", false, nil}}, map[string]interface{}{"X": `{}`}, nil, nil, map[string]interface{}{"X": `{}`}, nil, nil, false}, // missing parameter
	{[]Argument{Argument{"string", "z", "", false, nil}}, map[string]interface{}{"Z": ``}, nil, nil, map[string]interface{}{"Z": ``}, nil, nil, false},     // invalid argument source
}

func TestHookParseJSONParameters(t *testing.T) {
	for _, tt := range hookParseJSONParametersTests {
		h := &Hook{JSONStringParameters: tt.params}
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
		}
		err := h.ParseJSONParameters(r)
		if (err == nil) != tt.ok || !reflect.DeepEqual(tt.headers, tt.rheaders) {
			t.Errorf("failed to parse %v:\nexpected %#v, ok: %v\ngot %#v, ok: %v", tt.params, tt.rheaders, tt.ok, tt.headers, (err == nil))
		}
	}
}

var hookExtractCommandArgumentsTests = []struct {
	exec                    string
	args                    []Argument
	headers, query, payload map[string]interface{}
	value                   []string
	ok                      bool
}{
	{"test", []Argument{Argument{"header", "a", "", false, nil}}, map[string]interface{}{"A": "z"}, nil, nil, []string{"test", "z"}, true},
	// failures
	{"fail", []Argument{Argument{"payload", "a", "", false, nil}}, map[string]interface{}{"A": "z"}, nil, nil, []string{"fail", ""}, false},
}

func TestHookExtractCommandArguments(t *testing.T) {
	for _, tt := range hookExtractCommandArgumentsTests {
		h := &Hook{ExecuteCommand: tt.exec, PassArgumentsToCommand: tt.args}
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
		}
		value, err := h.ExtractCommandArguments(r)
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
//
//	[
//	  {
//	    "id": "push",
//	    "execute-command": "bb2mm",
//	    "command-working-directory": "/tmp",
//	    "pass-environment-to-command":
//	    [
//	      {
//	        "source": "entire-payload",
//	        "envname": "PAYLOAD"
//	      },
//	    ]
//	  }
//	]
var hookExtractCommandArgumentsForEnvTests = []struct {
	exec                    string
	args                    []Argument
	headers, query, payload map[string]interface{}
	value                   []string
	ok                      bool
}{
	// successes
	{
		"test",
		[]Argument{Argument{"header", "a", "", false, nil}},
		map[string]interface{}{"A": "z"}, nil, nil,
		[]string{"HOOK_a=z"},
		true,
	},
	{
		"test",
		[]Argument{Argument{"header", "a", "MYKEY", false, nil}},
		map[string]interface{}{"A": "z"}, nil, nil,
		[]string{"MYKEY=z"},
		true,
	},
	// failures
	{
		"fail",
		[]Argument{Argument{"payload", "a", "", false, nil}},
		map[string]interface{}{"A": "z"}, nil, nil,
		[]string{},
		false,
	},
}

func TestHookExtractCommandArgumentsForEnv(t *testing.T) {
	for _, tt := range hookExtractCommandArgumentsForEnvTests {
		h := &Hook{ExecuteCommand: tt.exec, PassEnvironmentToCommand: tt.args}
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
		}
		value, err := h.ExtractCommandArgumentsForEnv(r)
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
	{"../../hooks.json.example", false, true},
	{"../../hooks.yaml.example", false, true},
	{"../../hooks.json.tmpl.example", true, true},
	{"../../hooks.yaml.tmpl.example", true, true},
	{"", false, true},
	// failures
	{"missing.json", false, false},
}

func TestHooksLoadFromFile(t *testing.T) {
	secret := `foo"123`
	os.Setenv("XXXTEST_SECRET", secret)

	for _, tt := range hooksLoadFromFileTests {
		h := &Hooks{}
		err := h.LoadFromFile(tt.path, tt.asTemplate, "")
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
		err := h.LoadFromFile(tt.path, tt.asTemplate, "")
		if (err == nil) != tt.ok {
			t.Errorf(err.Error())
			continue
		}

		s := (*h.Match("webhook").TriggerRule.And)[0].Signature.Secret
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
	headers, query, payload            map[string]interface{}
	body                               []byte
	remoteAddr                         string
	ok                                 bool
	err                                bool
}{
	{"value", "", "", "z", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", true, false},
	{"regex", "^z", "", "z", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", true, false},
	// failures
	{"value", "", "", "X", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, false},
	{"regex", "^X", "", "", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, false},
	{"value", "", "2", "X", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"Y": "z"}, nil, nil, []byte{}, "", false, true}, // reference invalid header
	// errors
	{"regex", "*", "", "", "", Argument{"header", "a", "", false, nil}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, "", false, true}, // invalid regex
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
		req := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
			Body:    tt.body,
			RawRequest: &http.Request{
				RemoteAddr: tt.remoteAddr,
			},
		}
		ok, err := r.Evaluate(req)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("%d failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", i, r, tt.ok, tt.err, ok, err)
		}
	}
}

var signatureRuleTests = []struct {
	algorithm, secret       string
	sigSource               Argument
	stringToSign            *Argument
	headers, query, payload map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{"sha1", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": "b17e04cbb22afa8ffbff8796fc1894ed27badd9e"}, nil, nil, []byte(`{"a": "z"}`), true, false},
	{"sha1", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": "b17e04cbb22afa8ffbff8796fc1894ed27badd9e"}, nil, nil, []byte(`{"a": "z"}`), true, false},
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89"}, nil, nil, []byte(`{"a": "z"}`), true, false},
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": "f417af3a21bd70379b5796d5f013915e7029f62c580fb0f500f59a35a6f04c89"}, nil, nil, []byte(`{"a": "z"}`), true, false},
	// errors
	{"sha1", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true},   // invalid hmac
	{"sha1", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true},   // invalid hmac
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true}, // invalid hmac
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true}, // invalid hmac
	{"sha512", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true}, // invalid hmac
	{"sha512", "secret", Argument{"header", "a", "", false, nil}, nil, map[string]interface{}{"A": ""}, nil, nil, []byte{}, false, true}, // invalid hmac

	// template to build custom string-to-sign
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, &Argument{"template", "{{ printf \"%s\\n%s\" .BodyText (.GetHeader \"x-id\") }}", "", false, nil}, map[string]interface{}{"A": "sha256=4f1d62e6e6de1e31537a5faefabeffd7dce115bc499584feefbf8db6d2da4027", "X-Id": "test"}, nil, nil, []byte(`{"a": "z"}`), true, false},
	{"sha256", "secret", Argument{"header", "a", "", false, nil}, &Argument{"template", "{{ printf \"%s\\n%s\" .BodyText (.GetHeader \"x-id\") }}", "", false, nil}, map[string]interface{}{"A": "sha256=4f1d62e6e6de1e31537a5faefabeffd7dce115bc499584feefbf8db6d2da4027", "X-Id": "unexpected"}, nil, nil, []byte(`{"a": "z"}`), false, true},
}

func TestSignatureRule(t *testing.T) {
	for i, tt := range signatureRuleTests {
		if tt.stringToSign != nil {
			// post process the argument, as it would have been if it were loaded from a hooks file
			tt.stringToSign.postProcess()
		}
		r := SignatureRule{tt.algorithm, tt.secret, tt.sigSource, "", tt.stringToSign}
		req := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
			Body:    tt.body,
			RawRequest: &http.Request{
				RemoteAddr: "",
			},
		}
		ok, err := r.Evaluate(req)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("%d failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", i, r, tt.ok, tt.err, ok, err)
		}
	}
}

var andRuleTests = []struct {
	desc                    string // description of the test case
	rule                    AndRule
	headers, query, payload map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{
		"(a=z, b=y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
		},
		map[string]interface{}{"A": "z", "B": "y"}, nil, nil,
		[]byte{},
		true, false,
	},
	{
		"(a=z, b=Y): a=z && b=y",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
		},
		map[string]interface{}{"A": "z", "B": "Y"}, nil, nil,
		[]byte{},
		false, false,
	},
	// Complex test to cover Rules.Evaluate
	{
		"(a=z, b=y, c=x, d=w=, e=X, f=X): a=z && (b=y && c=x) && (d=w || e=v) && !f=u",
		AndRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{
				And: &AndRule{
					{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
					{Match: &MatchRule{"value", "", "", "x", Argument{"header", "c", "", false, nil}, ""}},
				},
			},
			{
				Or: &OrRule{
					{Match: &MatchRule{"value", "", "", "w", Argument{"header", "d", "", false, nil}, ""}},
					{Match: &MatchRule{"value", "", "", "v", Argument{"header", "e", "", false, nil}, ""}},
				},
			},
			{
				Not: &NotRule{
					Match: &MatchRule{"value", "", "", "u", Argument{"header", "f", "", false, nil}, ""},
				},
			},
		},
		map[string]interface{}{"A": "z", "B": "y", "C": "x", "D": "w", "E": "X", "F": "X"}, nil, nil,
		[]byte{},
		true, false,
	},
	{"empty rule", AndRule{{}}, nil, nil, nil, nil, false, false},
	// failures
	{
		"invalid rule",
		AndRule{{Match: &MatchRule{"value", "", "", "X", Argument{"header", "a", "", false, nil}, ""}}},
		map[string]interface{}{"Y": "z"}, nil, nil, nil,
		false, true,
	},
}

func TestAndRule(t *testing.T) {
	for _, tt := range andRuleTests {
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
			Body:    tt.body,
		}
		ok, err := tt.rule.Evaluate(r)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", tt.desc, tt.ok, tt.err, ok, err)
		}
	}
}

var orRuleTests = []struct {
	desc                    string // description of the test case
	rule                    OrRule
	headers, query, payload map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{
		"(a=z, b=X): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
		},
		map[string]interface{}{"A": "z", "B": "X"}, nil, nil,
		[]byte{},
		true, false,
	},
	{
		"(a=X, b=y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
		},
		map[string]interface{}{"A": "X", "B": "y"}, nil, nil,
		[]byte{},
		true, false,
	},
	{
		"(a=Z, b=Y): a=z || b=y",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
			{Match: &MatchRule{"value", "", "", "y", Argument{"header", "b", "", false, nil}, ""}},
		},
		map[string]interface{}{"A": "Z", "B": "Y"}, nil, nil,
		[]byte{},
		false, false,
	},
	// failures
	{
		"missing parameter node",
		OrRule{
			{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}},
		},
		map[string]interface{}{"Y": "Z"}, nil, nil,
		[]byte{},
		false, false,
	},
}

func TestOrRule(t *testing.T) {
	for _, tt := range orRuleTests {
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
			Body:    tt.body,
		}
		ok, err := tt.rule.Evaluate(r)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("%#v:\nexpected ok: %#v, err: %v\ngot ok: %#v err: %v", tt.desc, tt.ok, tt.err, ok, err)
		}
	}
}

var notRuleTests = []struct {
	desc                    string // description of the test case
	rule                    NotRule
	headers, query, payload map[string]interface{}
	body                    []byte
	ok                      bool
	err                     bool
}{
	{"(a=z): !a=X", NotRule{Match: &MatchRule{"value", "", "", "X", Argument{"header", "a", "", false, nil}, ""}}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, true, false},
	{"(a=z): !a=z", NotRule{Match: &MatchRule{"value", "", "", "z", Argument{"header", "a", "", false, nil}, ""}}, map[string]interface{}{"A": "z"}, nil, nil, []byte{}, false, false},
}

func TestNotRule(t *testing.T) {
	for _, tt := range notRuleTests {
		r := &Request{
			Headers: tt.headers,
			Query:   tt.query,
			Payload: tt.payload,
			Body:    tt.body,
		}
		ok, err := tt.rule.Evaluate(r)
		if ok != tt.ok || (err != nil) != tt.err {
			t.Errorf("failed to match %#v:\nexpected ok: %#v, err: %v\ngot ok: %#v, err: %v", tt.rule, tt.ok, tt.err, ok, err)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		ok   bool
	}{
		{"abcd", "abcd", true},
		{"zyxw", "abcd", false},
	} {
		if ok := compare(tt.a, tt.b); ok != tt.ok {
			t.Errorf("compare failed for %q and %q: got %v\n", tt.a, tt.b, ok)
		}
	}
}
