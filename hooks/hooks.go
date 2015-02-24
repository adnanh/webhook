package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/adnanh/webhook/helpers"
	"github.com/adnanh/webhook/rules"
)

// Hook is a structure that contains command to be executed
// and the current working directory name where that command should be executed
type Hook struct {
	ID      string     `json:"id"`
	Command string     `json:"command"`
	Cwd     string     `json:"cwd"`
	Secret  string     `json:"secret"`
	Args    []string   `json:"args"`
	Rule    rules.Rule `json:"trigger-rule"`
}

// Hooks represents structure that contains list of Hook objects
// and the name of file which is correspondingly mapped to it
type Hooks struct {
	fileName string
	list     []Hook
}

// ParseFormArgs gets arguments from the Form payload that should be passed to the command
func (h *Hook) ParseFormArgs(form url.Values) []string {
	var args = make([]string, 0)

	args = append(args, h.Command)

	for i := range h.Args {
		if arg := form[h.Args[i]]; len(arg) > 0 {
			args = append(args, arg[0])
		} else {
			args = append(args, "")
		}
	}

	fmt.Printf("parse form args %+v\n", args)

	return args
}

// ParseJSONArgs gets arguments from the JSON payload that should be passed to the command
func (h *Hook) ParseJSONArgs(payload interface{}) []string {
	var args = make([]string, 0)

	args = append(args, h.Command)

	for i := range h.Args {
		if arg, ok := helpers.ExtractJSONParameter(h.Args[i], payload); ok {
			args = append(args, arg)
		} else {
			args = append(args, "")
		}
	}

	fmt.Printf("parse json args %+v\n", args)

	return args
}

// UnmarshalJSON implementation for a single hook
func (h *Hook) UnmarshalJSON(j []byte) error {
	m := make(map[string]interface{})
	err := json.Unmarshal(j, &m)

	if err != nil {
		return err
	}

	if v, ok := m["id"]; ok {
		h.ID = v.(string)
	}

	if v, ok := m["command"]; ok {
		h.Command = v.(string)
	}

	if v, ok := m["cwd"]; ok {
		h.Cwd = v.(string)
	}

	if v, ok := m["secret"]; ok {
		h.Secret = v.(string)
	}

	if v, ok := m["args"]; ok {
		h.Args = make([]string, 0)

		for i := range v.([]interface{}) {
			h.Args = append(h.Args, v.([]interface{})[i].(string))
		}
	}

	if v, ok := m["trigger-rule"]; ok {
		rule := v.(map[string]interface{})

		if ruleValue, ok := rule["match"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(rules.MatchRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			h.Rule = *rulePtr

		} else if ruleValue, ok := rule["not"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(rules.NotRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			h.Rule = *rulePtr

		} else if ruleValue, ok := rule["and"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(rules.AndRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			h.Rule = *rulePtr

		} else if ruleValue, ok := rule["or"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(rules.OrRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			h.Rule = *rulePtr
		}

	}

	return nil
}

// New creates an instance of Hooks, tries to unmarshal contents of hookFile
// and returns a pointer to the newly created instance
func New(hookFile string) (*Hooks, error) {
	h := &Hooks{fileName: hookFile}

	if hookFile == "" {
		return h, nil
	}

	// parse hook file for hooks
	file, e := ioutil.ReadFile(hookFile)

	if e != nil {
		return h, e
	}

	e = json.Unmarshal(file, &(h.list))

	h.SetDefaults()

	return h, e
}

// Match looks for the hook with the given id in the list of hooks
// and returns the pointer to the hook if it exists, or nil if it doesn't exist
func (h *Hooks) Match(id string, params interface{}) *Hook {
	for i := range h.list {
		if h.list[i].ID == id {
			if h.list[i].Rule == nil || (h.list[i].Rule != nil && h.list[i].Rule.Evaluate(params)) {
				return &h.list[i]
			}
		}
	}

	return nil
}

// Count returns number of hooks in the list
func (h *Hooks) Count() int {
	return len(h.list)
}

// SetDefaults sets default values that were ommited for hooks in JSON file
func (h *Hooks) SetDefaults() {
	for i := range h.list {
		if h.list[i].Cwd == "" {
			h.list[i].Cwd = "."
		}

		if h.list[i].Args == nil {
			h.list[i].Args = make([]string, 1)
		}
	}
}
