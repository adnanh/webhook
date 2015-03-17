package hook

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"regexp"

	"github.com/adnanh/webhook/helpers"
)

// Constants used to specify the parameter source
const (
	SourceHeader  string = "header"
	SourceQuery   string = "url"
	SourcePayload string = "payload"
)

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
	}

	if source != nil {
		return helpers.ExtractParameter(ha.Name, *source)
	}

	return "", false
}

// Hook type is a structure containing details for a single hook
type Hook struct {
	ID                      string     `json:"id"`
	ExecuteCommand          string     `json:"execute-command"`
	CommandWorkingDirectory string     `json:"command-working-directory"`
	ResponseMessage         string     `json:"response-message"`
	PassArgumentsToCommand  []Argument `json:"pass-arguments-to-command"`
	TriggerRule             *Rules     `json:"trigger-rule"`
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
	return !r.Evaluate(headers, query, payload, body)
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
			expected, ok := helpers.CheckPayloadSignature(*body, r.Secret, arg)
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
