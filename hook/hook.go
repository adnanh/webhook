package hook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/textproto"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
)

// Constants used to specify the parameter source
const (
	SourceHeader        string = "header"
	SourceQuery         string = "url"
	SourceQueryAlias    string = "query"
	SourcePayload       string = "payload"
	SourceString        string = "string"
	SourceEntirePayload string = "entire-payload"
	SourceEntireQuery   string = "entire-query"
	SourceEntireHeaders string = "entire-headers"
)

const (
	// EnvNamespace is the prefix used for passing arguments into the command
	// environment.
	EnvNamespace string = "HOOK_"
)

// SignatureError describes an invalid payload signature passed to Hook.
type SignatureError struct {
	Signature string
}

func (e *SignatureError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("invalid payload signature %s", e.Signature)
}

// ArgumentError describes an invalid argument passed to Hook.
type ArgumentError struct {
	Argument Argument
}

func (e *ArgumentError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("couldn't retrieve argument for %+v", e.Argument)
}

// SourceError describes an invalid source passed to Hook.
type SourceError struct {
	Argument Argument
}

func (e *SourceError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("invalid source for argument %+v", e.Argument)
}

// ParseError describes an error parsing user input.
type ParseError struct {
	Err error
}

func (e *ParseError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Err.Error()
}

// CheckPayloadSignature calculates and verifies SHA1 signature of the given payload
func CheckPayloadSignature(payload []byte, secret string, signature string) (string, error) {
	if secret == "" {
		return "", errors.New("signature validation secret can not be empty")
	}

	signature = strings.TrimPrefix(signature, "sha1=")

	mac := hmac.New(sha1.New, []byte(secret))
	_, err := mac.Write(payload)
	if err != nil {
		return "", err
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		return expectedMAC, &SignatureError{signature}
	}
	return expectedMAC, err
}

// CheckPayloadSignature256 calculates and verifies SHA256 signature of the given payload
func CheckPayloadSignature256(payload []byte, secret string, signature string) (string, error) {
	if secret == "" {
		return "", errors.New("signature validation secret can not be empty")
	}

	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(payload)
	if err != nil {
		return "", err
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		return expectedMAC, &SignatureError{signature}
	}
	return expectedMAC, err
}

// CheckPayloadSignature512 calculates and verifies SHA512 signature of the given payload
func CheckPayloadSignature512(payload []byte, secret string, signature string) (string, error) {
	if secret == "" {
		return "", errors.New("signature validation secret can not be empty")
	}

	signature = strings.TrimPrefix(signature, "sha512=")

	mac := hmac.New(sha512.New, []byte(secret))
	_, err := mac.Write(payload)
	if err != nil {
		return "", err
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		return expectedMAC, &SignatureError{signature}
	}
	return expectedMAC, err
}

func CheckScalrSignature(headers map[string]interface{}, body []byte, signingKey string, checkDate bool) (bool, error) {
	// Check for the signature and date headers
	if _, ok := headers["X-Signature"]; !ok {
		return false, nil
	}
	if _, ok := headers["Date"]; !ok {
		return false, nil
	}
	if signingKey == "" {
		return false, errors.New("signature validation signing key can not be empty")
	}

	providedSignature := headers["X-Signature"].(string)
	dateHeader := headers["Date"].(string)
	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write(body)
	mac.Write([]byte(dateHeader))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return false, &SignatureError{providedSignature}
	}

	if !checkDate {
		return true, nil
	}
	// Example format: Fri 08 Sep 2017 11:24:32 UTC
	date, err := time.Parse("Mon 02 Jan 2006 15:04:05 MST", dateHeader)
	if err != nil {
		return false, err
	}
	now := time.Now()
	delta := math.Abs(now.Sub(date).Seconds())

	if delta > 300 {
		return false, &SignatureError{"outdated"}
	}
	return true, nil
}

// CheckIPWhitelist makes sure the provided remote address (of the form IP:port) falls within the provided IP range
// (in CIDR form or a single IP address).
func CheckIPWhitelist(remoteAddr string, ipRange string) (bool, error) {
	// Extract IP address from remote address.

	// IPv6 addresses will likely be surrounded by [].
	ip := strings.Trim(remoteAddr, " []")

	if i := strings.LastIndex(ip, ":"); i != -1 {
		ip = ip[:i]
		ip = strings.Trim(ip, " []")
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, fmt.Errorf("invalid IP address found in remote address '%s'", remoteAddr)
	}

	for _, r := range strings.Fields(ipRange) {
		// Extract IP range in CIDR form.  If a single IP address is provided, turn it into CIDR form.

		if !strings.Contains(r, "/") {
			r = r + "/32"
		}

		_, cidr, err := net.ParseCIDR(r)
		if err != nil {
			return false, err
		}

		if cidr.Contains(parsedIP) {
			return true, nil
		}
	}

	return false, nil
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

	paramsValue := reflect.ValueOf(params)

	switch paramsValue.Kind() {
	case reflect.Slice:
		paramsValueSliceLength := paramsValue.Len()
		if paramsValueSliceLength > 0 {

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

	case reflect.Map:
		// Check for raw key
		if v, ok := params.(map[string]interface{})[s]; ok {
			return v, true
		}

		// Checked for dotted references
		p := strings.SplitN(s, ".", 2)
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			if len(p) > 1 {
				return GetParameter(p[1], pValue)
			}

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
	Source       string `json:"source,omitempty"`
	Name         string `json:"name,omitempty"`
	EnvName      string `json:"envname,omitempty"`
	Base64Decode bool   `json:"base64decode,omitempty"`
}

// Get Argument method returns the value for the Argument's key name
// based on the Argument's source
func (ha *Argument) Get(headers, query, payload *map[string]interface{}) (string, bool) {
	var source *map[string]interface{}
	key := ha.Name

	switch ha.Source {
	case SourceHeader:
		source = headers
		key = textproto.CanonicalMIMEHeaderKey(ha.Name)
	case SourceQuery, SourceQueryAlias:
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
	case SourceEntireHeaders:
		r, err := json.Marshal(headers)
		if err != nil {
			return "", false
		}

		return string(r), true
	case SourceEntireQuery:
		r, err := json.Marshal(query)
		if err != nil {
			return "", false
		}

		return string(r), true
	}

	if source != nil {
		return ExtractParameterAsString(key, *source)
	}

	return "", false
}

// Header is a structure containing header name and it's value
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResponseHeaders is a slice of Header objects
type ResponseHeaders []Header

func (h *ResponseHeaders) String() string {
	// a 'hack' to display name=value in flag usage listing
	if len(*h) == 0 {
		return "name=value"
	}

	result := make([]string, len(*h))

	for idx, responseHeader := range *h {
		result[idx] = fmt.Sprintf("%s=%s", responseHeader.Name, responseHeader.Value)
	}

	return strings.Join(result, ", ")
}

// Set method appends new Header object from header=value notation
func (h *ResponseHeaders) Set(value string) error {
	splitResult := strings.SplitN(value, "=", 2)

	if len(splitResult) != 2 {
		return errors.New("header flag must be in name=value format")
	}

	*h = append(*h, Header{Name: splitResult[0], Value: splitResult[1]})
	return nil
}

// HooksFiles is a slice of String
type HooksFiles []string

func (h *HooksFiles) String() string {
	if len(*h) == 0 {
		return "hooks.json"
	}

	return strings.Join(*h, ", ")
}

// Set method appends new string
func (h *HooksFiles) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// Hook type is a structure containing details for a single hook
type Hook struct {
	ID                                  string          `json:"id,omitempty"`
	ExecuteCommand                      string          `json:"execute-command,omitempty"`
	CommandWorkingDirectory             string          `json:"command-working-directory,omitempty"`
	ResponseMessage                     string          `json:"response-message,omitempty"`
	ResponseHeaders                     ResponseHeaders `json:"response-headers,omitempty"`
	CaptureCommandOutput                bool            `json:"include-command-output-in-response,omitempty"`
	CaptureCommandOutputOnError         bool            `json:"include-command-output-in-response-on-error,omitempty"`
	PassEnvironmentToCommand            []Argument      `json:"pass-environment-to-command,omitempty"`
	PassArgumentsToCommand              []Argument      `json:"pass-arguments-to-command,omitempty"`
	PassFileToCommand                   []Argument      `json:"pass-file-to-command,omitempty"`
	JSONStringParameters                []Argument      `json:"parse-parameters-as-json,omitempty"`
	TriggerRule                         *Rules          `json:"trigger-rule,omitempty"`
	TriggerRuleMismatchHttpResponseCode int             `json:"trigger-rule-mismatch-http-response-code,omitempty"`
	IncomingPayloadContentType          string          `json:"incoming-payload-content-type,omitempty"`
	SuccessHttpResponseCode             int             `json:"success-http-response-code,omitempty"`
}

// ParseJSONParameters decodes specified arguments to JSON objects and replaces the
// string with the newly created object
func (h *Hook) ParseJSONParameters(headers, query, payload *map[string]interface{}) []error {
	errors := make([]error, 0)

	for i := range h.JSONStringParameters {
		if arg, ok := h.JSONStringParameters[i].Get(headers, query, payload); ok {
			var newArg map[string]interface{}

			decoder := json.NewDecoder(strings.NewReader(string(arg)))
			decoder.UseNumber()

			err := decoder.Decode(&newArg)
			if err != nil {
				errors = append(errors, &ParseError{err})
				continue
			}

			var source *map[string]interface{}

			switch h.JSONStringParameters[i].Source {
			case SourceHeader:
				source = headers
			case SourcePayload:
				source = payload
			case SourceQuery, SourceQueryAlias:
				source = query
			}

			if source != nil {
				key := h.JSONStringParameters[i].Name

				if h.JSONStringParameters[i].Source == SourceHeader {
					key = textproto.CanonicalMIMEHeaderKey(h.JSONStringParameters[i].Name)
				}

				ReplaceParameter(key, source, newArg)
			} else {
				errors = append(errors, &SourceError{h.JSONStringParameters[i]})
			}
		} else {
			errors = append(errors, &ArgumentError{h.JSONStringParameters[i]})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ExtractCommandArguments creates a list of arguments, based on the
// PassArgumentsToCommand property that is ready to be used with exec.Command()
func (h *Hook) ExtractCommandArguments(headers, query, payload *map[string]interface{}) ([]string, []error) {
	args := make([]string, 0)
	errors := make([]error, 0)

	args = append(args, h.ExecuteCommand)

	for i := range h.PassArgumentsToCommand {
		if arg, ok := h.PassArgumentsToCommand[i].Get(headers, query, payload); ok {
			args = append(args, arg)
		} else {
			args = append(args, "")
			errors = append(errors, &ArgumentError{h.PassArgumentsToCommand[i]})
		}
	}

	if len(errors) > 0 {
		return args, errors
	}

	return args, nil
}

// ExtractCommandArgumentsForEnv creates a list of arguments in key=value
// format, based on the PassEnvironmentToCommand property that is ready to be used
// with exec.Command().
func (h *Hook) ExtractCommandArgumentsForEnv(headers, query, payload *map[string]interface{}) ([]string, []error) {
	args := make([]string, 0)
	errors := make([]error, 0)
	for i := range h.PassEnvironmentToCommand {
		if arg, ok := h.PassEnvironmentToCommand[i].Get(headers, query, payload); ok {
			if h.PassEnvironmentToCommand[i].EnvName != "" {
				// first try to use the EnvName if specified
				args = append(args, h.PassEnvironmentToCommand[i].EnvName+"="+arg)
			} else {
				// then fallback on the name
				args = append(args, EnvNamespace+h.PassEnvironmentToCommand[i].Name+"="+arg)
			}
		} else {
			errors = append(errors, &ArgumentError{h.PassEnvironmentToCommand[i]})
		}
	}

	if len(errors) > 0 {
		return args, errors
	}

	return args, nil
}

// FileParameter describes a pass-file-to-command instance to be stored as file
type FileParameter struct {
	File    *os.File
	EnvName string
	Data    []byte
}

// ExtractCommandArgumentsForFile creates a list of arguments in key=value
// format, based on the PassFileToCommand property that is ready to be used
// with exec.Command().
func (h *Hook) ExtractCommandArgumentsForFile(headers, query, payload *map[string]interface{}) ([]FileParameter, []error) {
	args := make([]FileParameter, 0)
	errors := make([]error, 0)
	for i := range h.PassFileToCommand {
		if arg, ok := h.PassFileToCommand[i].Get(headers, query, payload); ok {

			if h.PassFileToCommand[i].EnvName == "" {
				// if no environment-variable name is set, fall-back on the name
				log.Printf("no ENVVAR name specified, falling back to [%s]", EnvNamespace+strings.ToUpper(h.PassFileToCommand[i].Name))
				h.PassFileToCommand[i].EnvName = EnvNamespace + strings.ToUpper(h.PassFileToCommand[i].Name)
			}

			var fileContent []byte
			if h.PassFileToCommand[i].Base64Decode {
				dec, err := base64.StdEncoding.DecodeString(arg)
				if err != nil {
					log.Printf("error decoding string [%s]", err)
				}
				fileContent = []byte(dec)
			} else {
				fileContent = []byte(arg)
			}

			args = append(args, FileParameter{EnvName: h.PassFileToCommand[i].EnvName, Data: fileContent})

		} else {
			errors = append(errors, &ArgumentError{h.PassFileToCommand[i]})
		}
	}

	if len(errors) > 0 {
		return args, errors
	}

	return args, nil
}

// Hooks is an array of Hook objects
type Hooks []Hook

// LoadFromFile attempts to load hooks from the specified file, which
// can be either JSON or YAML.  The asTemplate parameter causes the file
// contents to be parsed as a Go text/template prior to unmarshalling.
func (h *Hooks) LoadFromFile(path string, asTemplate bool) error {
	if path == "" {
		return nil
	}

	// parse hook file for hooks
	file, e := ioutil.ReadFile(path)

	if e != nil {
		return e
	}

	if asTemplate {
		funcMap := template.FuncMap{"getenv": getenv}

		tmpl, err := template.New("hooks").Funcs(funcMap).Parse(string(file))
		if err != nil {
			return err
		}

		var buf bytes.Buffer

		err = tmpl.Execute(&buf, nil)
		if err != nil {
			return err
		}

		file = buf.Bytes()
	}

	f, err := h.ReplaceEnv(string(file))
	if err != nil {
		return err
	}

	e = yaml.Unmarshal([]byte(f), h)
	return e
}

// ReplaceEnv replace env
func (h *Hooks) ReplaceEnv(str string) (string, error) {
	expr := `\${[A-Za-z_]*}`
	r, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("regexp.Compile error: %w", err)
	}
	if r == nil {
		return "", fmt.Errorf("r is nil")
	}
	all := r.FindAll([]byte(str), -1)
	for _, v := range all {
		envStr := string(v)
		env := os.Getenv(envStr[2 : len(envStr)-1])
		if env == "" {
			return "", errors.New("get env failed:" + envStr)
		}
		str = strings.ReplaceAll(str, envStr, env)
	}
	return str, nil
}

// Append appends hooks unless the new hooks contain a hook with an ID that already exists
func (h *Hooks) Append(other *Hooks) error {
	for _, hook := range *other {
		if h.Match(hook.ID) != nil {
			return fmt.Errorf("hook with ID %s is already defined", hook.ID)
		}

		*h = append(*h, hook)
	}

	return nil
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
	And   *AndRule   `json:"and,omitempty"`
	Or    *OrRule    `json:"or,omitempty"`
	Not   *NotRule   `json:"not,omitempty"`
	Match *MatchRule `json:"match,omitempty"`
}

// Evaluate finds the first rule property that is not nil and returns the value
// it evaluates to
func (r Rules) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte, remoteAddr string) (bool, error) {
	switch {
	case r.And != nil:
		return r.And.Evaluate(headers, query, payload, body, remoteAddr)
	case r.Or != nil:
		return r.Or.Evaluate(headers, query, payload, body, remoteAddr)
	case r.Not != nil:
		return r.Not.Evaluate(headers, query, payload, body, remoteAddr)
	case r.Match != nil:
		return r.Match.Evaluate(headers, query, payload, body, remoteAddr)
	}

	return false, nil
}

// AndRule will evaluate to true if and only if all of the ChildRules evaluate to true
type AndRule []Rules

// Evaluate AndRule will return true if and only if all of ChildRules evaluate to true
func (r AndRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte, remoteAddr string) (bool, error) {
	res := true

	for _, v := range r {
		rv, err := v.Evaluate(headers, query, payload, body, remoteAddr)
		if err != nil {
			return false, err
		}

		res = res && rv
		if !res {
			return res, nil
		}
	}

	return res, nil
}

// OrRule will evaluate to true if any of the ChildRules evaluate to true
type OrRule []Rules

// Evaluate OrRule will return true if any of ChildRules evaluate to true
func (r OrRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte, remoteAddr string) (bool, error) {
	res := false

	for _, v := range r {
		rv, err := v.Evaluate(headers, query, payload, body, remoteAddr)
		if err != nil {
			return false, err
		}

		res = res || rv
		if res {
			return res, nil
		}
	}

	return res, nil
}

// NotRule will evaluate to true if any and only if the ChildRule evaluates to false
type NotRule Rules

// Evaluate NotRule will return true if and only if ChildRule evaluates to false
func (r NotRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte, remoteAddr string) (bool, error) {
	rv, err := Rules(r).Evaluate(headers, query, payload, body, remoteAddr)
	return !rv, err
}

// MatchRule will evaluate to true based on the type
type MatchRule struct {
	Type      string   `json:"type,omitempty"`
	Regex     string   `json:"regex,omitempty"`
	Secret    string   `json:"secret,omitempty"`
	Value     string   `json:"value,omitempty"`
	Parameter Argument `json:"parameter,omitempty"`
	IPRange   string   `json:"ip-range,omitempty"`
}

// Constants for the MatchRule type
const (
	MatchValue      string = "value"
	MatchRegex      string = "regex"
	MatchHashSHA1   string = "payload-hash-sha1"
	MatchHashSHA256 string = "payload-hash-sha256"
	MatchHashSHA512 string = "payload-hash-sha512"
	IPWhitelist     string = "ip-whitelist"
	ScalrSignature  string = "scalr-signature"
)

// Evaluate MatchRule will return based on the type
func (r MatchRule) Evaluate(headers, query, payload *map[string]interface{}, body *[]byte, remoteAddr string) (bool, error) {
	if r.Type == IPWhitelist {
		return CheckIPWhitelist(remoteAddr, r.IPRange)
	}
	if r.Type == ScalrSignature {
		return CheckScalrSignature(*headers, *body, r.Secret, true)
	}

	if arg, ok := r.Parameter.Get(headers, query, payload); ok {
		switch r.Type {
		case MatchValue:
			return compare(arg, r.Value), nil
		case MatchRegex:
			return regexp.MatchString(r.Regex, arg)
		case MatchHashSHA1:
			_, err := CheckPayloadSignature(*body, r.Secret, arg)
			return err == nil, err
		case MatchHashSHA256:
			_, err := CheckPayloadSignature256(*body, r.Secret, arg)
			return err == nil, err
		case MatchHashSHA512:
			_, err := CheckPayloadSignature512(*body, r.Secret, arg)
			return err == nil, err
		}
	}
	return false, nil
}

// compare is a helper function for constant time string comparisons.
func compare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// getenv provides a template function to retrieve OS environment variables.
func getenv(s string) string {
	return os.Getenv(s)
}
