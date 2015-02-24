package rules

import (
	"encoding/json"

	"github.com/adnanh/webhook/helpers"
)

// Rule interface
type Rule interface {
	Evaluate(params interface{}) bool
}

// AndRule type is a structure that contains list of rules (SubRules) that will be evaluated,
// and the AndRule's Evaluate method will evaluate to true if and only if all
// of the SubRules evaluate to true
type AndRule struct {
	SubRules []Rule `json:"and"`
}

// OrRule type is a structure that contains list of rules (SubRules) that will be evaluated,
// and the OrRule's Evaluate method will evaluate to true if any of the SubRules
// evaluate to true
type OrRule struct {
	SubRules []Rule `json:"or"`
}

// NotRule type is a structure that contains a single rule (SubRule) that will be evaluated,
// and the OrRule's Evaluate method will evaluate to true if any and only if
// the SubRule evaluates to false
type NotRule struct {
	SubRule Rule `json:"not"`
}

// MatchRule type is a structure that contains MatchParameter structure
type MatchRule struct {
	MatchParameter MatchParameter `json:"match"`
}

// MatchParameter type is a structure that contains Parameter and Value which are used in
// Match
type MatchParameter struct {
	Parameter string `json:"parameter"`
	Value     string `json:"value"`
}

// Evaluate AndRule will return true if and only if all of SubRules evaluate to true
func (r AndRule) Evaluate(params interface{}) bool {
	res := true

	for _, v := range r.SubRules {
		res = res && v.Evaluate(params)
		if res == false {
			return res
		}
	}

	return res
}

// Evaluate OrRule will return true if any of SubRules evaluate to true
func (r OrRule) Evaluate(params interface{}) bool {
	res := false

	for _, v := range r.SubRules {
		res = res || v.Evaluate(params)
		if res == true {
			return res
		}
	}

	return res
}

// Evaluate NotRule will return true if and only if SubRule evaluates to false
func (r NotRule) Evaluate(params interface{}) bool {
	return !r.SubRule.Evaluate(params)
}

// Evaluate MatchRule will return true if and only if the MatchParameter.Parameter
// named property value in supplied params matches the MatchParameter.Value
func (r MatchRule) Evaluate(params interface{}) bool {
	if v, ok := helpers.ExtractJSONParameter(r.MatchParameter.Parameter, params); ok {
		return v == r.MatchParameter.Value
	}

	return false
}

// UnmarshalJSON implementation for the MatchRule type
func (r *MatchRule) UnmarshalJSON(j []byte) error {
	err := json.Unmarshal(j, &r.MatchParameter)
	return err
}

// UnmarshalJSON implementation for the NotRule type
func (r *NotRule) UnmarshalJSON(j []byte) error {
	m := make(map[string]interface{})
	err := json.Unmarshal(j, &m)

	if ruleValue, ok := m["match"]; ok {
		ruleString, _ := json.Marshal(ruleValue)
		rulePtr := new(MatchRule)

		err = json.Unmarshal(ruleString, &rulePtr.MatchParameter)
		if err != nil {
			return err
		}

		r.SubRule = *rulePtr

	} else if ruleValue, ok := m["not"]; ok {
		ruleString, _ := json.Marshal(ruleValue)
		rulePtr := new(NotRule)

		err = json.Unmarshal(ruleString, rulePtr)
		if err != nil {
			return err
		}

		r.SubRule = *rulePtr

	} else if ruleValue, ok := m["and"]; ok {
		ruleString, _ := json.Marshal(ruleValue)
		rulePtr := new(AndRule)

		err = json.Unmarshal(ruleString, rulePtr)
		if err != nil {
			return err
		}

		r.SubRule = *rulePtr

	} else if ruleValue, ok := m["or"]; ok {
		ruleString, _ := json.Marshal(ruleValue)
		rulePtr := new(OrRule)

		err = json.Unmarshal(ruleString, rulePtr)
		if err != nil {
			return err
		}

		r.SubRule = *rulePtr

	}

	return err
}

// UnmarshalJSON implementation for the AndRule type
func (r *AndRule) UnmarshalJSON(j []byte) error {
	rules := new([]interface{})
	err := json.Unmarshal(j, &rules)

	for _, rulesValue := range *rules {
		m := rulesValue.(map[string]interface{})

		if ruleValue, ok := m["match"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(MatchRule)

			err = json.Unmarshal(ruleString, &rulePtr.MatchParameter)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["not"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(NotRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["and"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(AndRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["or"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(OrRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)
		}
	}

	return err
}

// UnmarshalJSON implementation for the OrRule type
func (r *OrRule) UnmarshalJSON(j []byte) error {
	rules := new([]interface{})
	err := json.Unmarshal(j, &rules)

	for _, rulesValue := range *rules {
		m := rulesValue.(map[string]interface{})

		if ruleValue, ok := m["match"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(MatchRule)

			err = json.Unmarshal(ruleString, &rulePtr.MatchParameter)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["not"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(NotRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["and"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(AndRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)

		} else if ruleValue, ok := m["or"]; ok {
			ruleString, _ := json.Marshal(ruleValue)
			rulePtr := new(OrRule)

			err = json.Unmarshal(ruleString, rulePtr)
			if err != nil {
				return err
			}

			r.SubRules = append(r.SubRules, *rulePtr)
		}
	}

	return err
}
