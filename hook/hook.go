package hook

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Constants used to specify the parameter source
const (
	SourceHeader        string = "header"
	SourceQuery         string = "url"
	SourcePayload       string = "payload"
	SourceString        string = "string"
	SourceEntirePayload string = "entire-payload"
)

// CheckPayloadSignature calculates and verifies SHA1 signature of the given payload
func CheckPayloadSignature(payload []byte, secret string, signature string) (string, bool) {
	if strings.HasPrefix(signature, "sha1=") {
		signature = signature[5:]
	}

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return expectedMAC, hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// ReplaceParameter replaces parameter value with the passed value in the passed map
// (please note you should pass pointer to the map, because we're modifying it)
// based on the passed string
func ReplaceParameter(s string, params interface{}, value interface{}) bool {
	if params == nil {
		return false
	}

	if paramsValue := reflect.ValueOf(params); paramsValue.Kind() == reflect.Slice {
		if paramsValueSliceLength := paramsValue.Len(); paramsValueSliceLength > 0 {

			if p := strings.SplitN(s, ".", 2); len(p) > 1 {
				index, err := strconv.ParseUint(p[0], 10, 64)

				if err != nil || paramsValueSliceLength <= int(index) {
					return false
				}

				return ReplaceParameter(p[1], params.([]interface{})[index], value)
			}
		}

		return false
	}

	if p := strings.SplitN(s, ".", 2); len(p) > 1 {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return ReplaceParameter(p[1], pValue, value)
		}
	} else {
		if _, ok := (*params.(*map[string]interface{}))[p[0]]; ok {
			(*params.(*map[string]interface{}))[p[0]] = value
			return true
		}
	}

	return false
}

// GetParameter extracts interface{} value based on the passed string
func GetParameter(s string, params interface{}) (interface{}, bool) {
	if params == nil {
		return nil, false
	}

	if paramsValue := reflect.ValueOf(params); paramsValue.Kind() == reflect.Slice {
		if paramsValueSliceLength := paramsValue.Len(); paramsValueSliceLength > 0 {

			if p := strings.SplitN(s, ".", 2); len(p) > 1 {
				index, err := strconv.ParseUint(p[0], 10, 64)

				if err != nil || paramsValueSliceLength <= int(index) {
					return nil, false
				}

				return GetParameter(p[1], params.([]interface{})[index])
			}

			index, err := strconv.ParseUint(s, 10, 64)

			if err != nil || paramsValueSliceLength <= int(index) {
				return nil, false
			}

			return params.([]interface{})[index], true
		}

		return nil, false
	}

	if p := strings.SplitN(s, ".", 2); len(p) > 1 {
		if paramsValue := reflect.ValueOf(params); paramsValue.Kind() == reflect.Map {
			if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
				return GetParameter(p[1], pValue)
			}
		} else {
			return nil, false
		}
	} else {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return pValue, true
		}
	}

	return nil, false
}

// ExtractParameterAsString extracts value from interface{} as string based on the passed string
func ExtractParameterAsString(s string, params interface{}) (string, bool) {
	if pValue, ok := GetParameter(s, params); ok {
		return fmt.Sprintf("%v", pValue), true
	}
	return "", false
}

// Argument type specifies the parameter key name and the source it should
// be extracted from
type Argument struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

// Get Argument method returns the value for the Argument's key name
// based on the Argument's source
func (ha *Argument) Get(headers, query, payload *map[string]interface{}) (string, bool) {
	var source *map[string]interface{}

	switch ha.Source {
	case SourceHeader:
		source = headers
	case SourceQuery:
		source = query
	case SourcePayload:
		source = payload
	case SourceString:
		return ha.Name, true
	case SourceEntirePayload:
		r, err := json.Marshal(payload)

		if err != nil {
			return "", false
		}

		return string(r), true
	}

	if source != nil {
		return ExtractParameterAsString(ha.Name, *source)
	}

	return "", false
}

// Hook type is a structure containing details for a single hook
type Hook struct {
	ID                      string     `json:"id"`
	ExecuteCommand          string     `json:"execute-command"`
	CommandWorkingDirectory string     `json:"command-working-directory"`
	ResponseMessage         string     `json:"response-message"`
	CaptureCommandOutput    bool       `json:"include-command-output-in-response"`
	PassArgumentsToCommand  []Argument `json:"pass-arguments-to-command"`
	JSONStringParameters    []Argument `json:"parse-parameters-as-json"`
	TriggerRule             *Rules     `json:"trigger-rule"`
}

// ParseJSONParameters decodes specified arguments to JSON objects and replaces the
// string with the newly created object
func (h *Hook) ParseJSONParameters(headers, query, payload *map[string]interface{}) {
	for i := range h.JSONStringParameters {
		if arg, ok := h.JSONStringParameters[i].Get(headers, query, payload); ok {
			var newArg map[string]interface{}

			decoder := json.NewDecoder(strings.NewReader(string(arg)))
			decoder.UseNumber()

			err := decoder.Decode(&newArg)

			if err != nil {
				log.Printf("error parsing argument as JSON payload %+v\n", err)
			} else {
				var source *map[string]interface{}

				switch h.JSONStringParameters[i].Source {
				case SourceHeader:
					source = headers
				case SourcePayload:
					source = payload
				case SourceQuery:
					source = query
				}

				if source != nil {
					ReplaceParameter(h.JSONStringParameters[i].Name, source, newArg)
				} else {
					log.Printf("invalid source for argument %+v\n", h.JSONStringParameters[i])
				}
			}
		} else {
			log.Printf("couldn't retrieve argument for %+v\n", h.JSONStringParameters[i])
		}
	}
}

// ExtractCommandArguments creates a list of arguments, based on the
// PassArgumentsToCommand property that is ready to be used with exec.Command()
func (h *Hook) ExtractCommandArguments(headers, query, payload *map[string]interface{}) []string {
	var args = make([]string, 0)

	args = append(args, h.ExecuteCommand)

	for i := range h.PassArgumentsToCommand {
		if arg, ok := h.PassArgumentsToCommand[i].Get(headers, query, payload); ok {
			args = append(args, arg)
		} else {
			args = append(args, "")
			log.Printf("couldn't retrieve argument for %+v\n", h.PassArgumentsToCommand[i])
		}
	}

	return args
}

// Hooks is an array of Hook objects
type Hooks []Hook

// LoadFromFile attempts to load hooks from specified JSON file
func (h *Hooks) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	// parse hook file for hooks
	file, e := ioutil.ReadFile(path)

	if e != nil {
		return e
	}

	e = json.Unmarshal(file, h)
	return e
}

// Match iterates through Hooks and returns first one that matches the given ID,
// if no hook matches the given ID, nil is returned
func (h *Hooks) Match(id string) *Hook {
	for i := range *h {
		if (*h)[i].ID == id {
			return &(*h)[i]
		}
	}

	return nil
}

// MatchAll iterates through Hooks and returns all of the hooks that match the
// given ID, if no hook matches the given ID, nil is returned
func (h *Hooks) MatchAll(id string) []*Hook {
	matchedHooks := make([]*Hook, 0)
	for i := range *h {
		if (*h)[i].ID == id {
			matchedHooks = append(matchedHooks, &(*h)[i])
		}
	}

	if len(matchedHooks) > 0 {
		return matchedHooks
	}

	return nil
}

// Rules is a structure that contains one of the valid rule types
type Rules struct {
	And   *AndRule   `json:"and"`
	Or    *OrRule    `json:"or"`
	Not   *NotRule   `json:"not"`
	Match *MatchRule `json:"match"`
}

// Evaluate finds the first rule property that is not nil and returns the value
// it evaluates to
func (r Rules) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte) bool {
	switch {
	case r.And != nil:
		return r.And.Evaluate(headers, query, payload, body)
	case r.Or != nil:
		return r.Or.Evaluate(headers, query, payload, body)
	case r.Not != nil:
		return r.Not.Evaluate(headers, query, payload, body)
	case r.Match != nil:
		return r.Match.Evaluate(headers, query, payload, body)
	}

	return false
}

// AndRule will evaluate to true if and only if all of the ChildRules evaluate to true
type AndRule []Rules

// Evaluate AndRule will return true if and only if all of ChildRules evaluate to true
func (r AndRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte) bool {
	res := true

	for _, v := range r {
		res = res && v.Evaluate(headers, query, payload, body)
		if res == false {
			return res
		}
	}

	return res
}

// OrRule will evaluate to true if any of the ChildRules evaluate to true
type OrRule []Rules

// Evaluate OrRule will return true if any of ChildRules evaluate to true
func (r OrRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte) bool {
	res := false

	for _, v := range r {
		res = res || v.Evaluate(headers, query, payload, body)
		if res == true {
			return res
		}
	}

	return res
}

// NotRule will evaluate to true if any and only if the ChildRule evaluates to false
type NotRule Rules

// Evaluate NotRule will return true if and only if ChildRule evaluates to false
func (r NotRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte) bool {
	return !Rules(r).Evaluate(headers, query, payload, body)
}

// MatchRule will evaluate to true based on the type
type MatchRule struct {
	Type      string   `json:"type"`
	Regex     string   `json:"regex"`
	Secret    string   `json:"secret"`
	Value     string   `json:"value"`
	Parameter Argument `json:"parameter"`
}

// Constants for the MatchRule type
const (
	MatchValue    string = "value"
	MatchRegex    string = "regex"
	MatchHashSHA1 string = "payload-hash-sha1"
)

// Evaluate MatchRule will return based on the type
func (r MatchRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte) bool {
	if arg, ok := r.Parameter.Get(headers, query, payload); ok {
		switch r.Type {
		case MatchValue:
			return arg == r.Value
		case MatchRegex:
			ok, err := regexp.MatchString(r.Regex, arg)
			if err != nil {
				log.Printf("error while trying to evaluate regex: %+v", err)
			}
			return ok
		case MatchHashSHA1:
			expected, ok := CheckPayloadSignature(*body, r.Secret, arg)
			if !ok {
				log.Printf("payload signature mismatch, expected %s got %s", expected, arg)
			}

			return ok
		}
	} else {
		log.Printf("couldn't retrieve argument for %+v\n", r.Parameter)
	}
	return false
}

// CommandStatusResponse type encapsulates the executed command exit code, message, stdout and stderr
type CommandStatusResponse struct {
	ResponseMessage string `json:"message"`
	Output          string `json:"output"`
	Error           string `json:"error"`
}
