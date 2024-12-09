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
	"hash"
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
	SourceHeader         string = "header"
	SourceQuery          string = "url"
	SourceQueryAlias     string = "query"
	SourcePayload        string = "payload"
	SourceRawRequestBody string = "raw-request-body"
	SourceRequest        string = "request"
	SourceString         string = "string"
	SourceEntirePayload  string = "entire-payload"
	SourceEntireQuery    string = "entire-query"
	SourceEntireHeaders  string = "entire-headers"
	SourceTemplate       string = "template"
)

const (
	// EnvNamespace is the prefix used for passing arguments into the command
	// environment.
	EnvNamespace string = "HOOK_"
)

// ParameterNodeError describes an error walking a parameter node.
type ParameterNodeError struct {
	key string
}

func (e *ParameterNodeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("parameter node not found: %s", e.key)
}

// IsParameterNodeError returns whether err is of type ParameterNodeError.
func IsParameterNodeError(err error) bool {
	switch err.(type) {
	case *ParameterNodeError:
		return true
	default:
		return false
	}
}

// SignatureError describes an invalid payload signature passed to Hook.
type SignatureError struct {
	Signature  string
	Signatures []string

	emptyPayload bool
}

func (e *SignatureError) Error() string {
	if e == nil {
		return "<nil>"
	}

	var empty string
	if e.emptyPayload {
		empty = " on empty payload"
	}

	if e.Signatures != nil {
		return fmt.Sprintf("invalid payload signatures %s%s", e.Signatures, empty)
	}

	return fmt.Sprintf("invalid payload signature %s%s", e.Signature, empty)
}

// IsSignatureError returns whether err is of type SignatureError.
func IsSignatureError(err error) bool {
	switch err.(type) {
	case *SignatureError:
		return true
	default:
		return false
	}
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

// ExtractCommaSeparatedValues will extract the values matching the key.
func ExtractCommaSeparatedValues(source, prefix string) []string {
	parts := strings.Split(source, ",")
	values := make([]string, 0)
	for _, part := range parts {
		if strings.HasPrefix(part, prefix) {
			values = append(values, strings.TrimPrefix(part, prefix))
		}
	}

	return values
}

// ExtractSignatures will extract all the signatures from the source.
func ExtractSignatures(source, prefix string) []string {
	// If there are multiple possible matches, let the comma separated extractor
	// do it's work.
	if strings.Contains(source, ",") {
		return ExtractCommaSeparatedValues(source, prefix)
	}

	// There were no commas, so just trim the prefix (if it even exists) and
	// pass it back.
	return []string{
		strings.TrimPrefix(source, prefix),
	}
}

// ValidateMAC will verify that the expected mac for the given hash will match
// the one provided.
func ValidateMAC(payload []byte, mac hash.Hash, signatures []string) (string, error) {
	// Write the payload to the provided hash.
	_, err := mac.Write(payload)
	if err != nil {
		return "", err
	}

	actualMAC := hex.EncodeToString(mac.Sum(nil))

	for _, signature := range signatures {
		if hmac.Equal([]byte(signature), []byte(actualMAC)) {
			return actualMAC, err
		}
	}

	e := &SignatureError{Signatures: signatures}
	if len(payload) == 0 {
		e.emptyPayload = true
	}

	return actualMAC, e
}

func CheckScalrSignature(r *Request, signingKey string, checkDate bool) (bool, error) {
	if r.Headers == nil {
		return false, nil
	}

	// Check for the signature and date headers
	if _, ok := r.Headers["X-Signature"]; !ok {
		return false, nil
	}
	if _, ok := r.Headers["Date"]; !ok {
		return false, nil
	}
	if signingKey == "" {
		return false, errors.New("signature validation signing key can not be empty")
	}

	providedSignature := r.Headers["X-Signature"].(string)
	dateHeader := r.Headers["Date"].(string)
	mac := hmac.New(sha1.New, []byte(signingKey))
	mac.Write(r.Body)
	mac.Write([]byte(dateHeader))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return false, &SignatureError{Signature: providedSignature}
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
		return false, &SignatureError{Signature: "outdated"}
	}
	return true, nil
}

// CheckIPWhitelist makes sure the provided remote address (of the form IP:port) falls within the provided IP range
// (in CIDR form or a single IP address).
func CheckIPWhitelist(remoteAddr, ipRange string) (bool, error) {
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
func ReplaceParameter(s string, params, value interface{}) bool {
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
func GetParameter(s string, params interface{}) (interface{}, error) {
	if params == nil {
		return nil, errors.New("no parameters")
	}

	paramsValue := reflect.ValueOf(params)

	switch paramsValue.Kind() {
	case reflect.Slice:
		paramsValueSliceLength := paramsValue.Len()
		if paramsValueSliceLength > 0 {

			if p := strings.SplitN(s, ".", 2); len(p) > 1 {
				index, err := strconv.ParseUint(p[0], 10, 64)

				if err != nil || paramsValueSliceLength <= int(index) {
					return nil, &ParameterNodeError{s}
				}

				return GetParameter(p[1], params.([]interface{})[index])
			}

			index, err := strconv.ParseUint(s, 10, 64)

			if err != nil || paramsValueSliceLength <= int(index) {
				return nil, &ParameterNodeError{s}
			}

			return params.([]interface{})[index], nil
		}

		return nil, &ParameterNodeError{s}

	case reflect.Map:
		// Check for raw key
		if v, ok := params.(map[string]interface{})[s]; ok {
			return v, nil
		}

		// Checked for dotted references
		p := strings.SplitN(s, ".", 2)
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			if len(p) > 1 {
				return GetParameter(p[1], pValue)
			}

			return pValue, nil
		}
	}

	return nil, &ParameterNodeError{s}
}

// ExtractParameterAsString extracts value from interface{} as string based on
// the passed string.  Complex data types are rendered as JSON instead of the Go
// Stringer format.
func ExtractParameterAsString(s string, params interface{}) (string, error) {
	pValue, err := GetParameter(s, params)
	if err != nil {
		return "", err
	}

	switch v := reflect.ValueOf(pValue); v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		r, err := json.Marshal(pValue)
		if err != nil {
			return "", err
		}

		return string(r), nil

	default:
		return fmt.Sprintf("%v", pValue), nil
	}
}

// Argument type specifies the parameter key name and the source it should
// be extracted from
type Argument struct {
	Source       string `json:"source,omitempty"`
	Name         string `json:"name,omitempty"`
	EnvName      string `json:"envname,omitempty"`
	Base64Decode bool   `json:"base64decode,omitempty"`

	// if the Argument is SourceTemplate, this will be the compiled template,
	// otherwise it will be nil
	template *template.Template
}

// UnmarshalJSON parses an Argument in the normal way, and then allows the
// newly-loaded Argument to do any necessary post-processing.
func (ha *Argument) UnmarshalJSON(text []byte) error {
	// First unmarshal as normal, skipping the custom unmarshaller
	type jsonArgument Argument
	if err := json.Unmarshal(text, (*jsonArgument)(ha)); err != nil {
		return err
	}

	return ha.postProcess()
}

// postProcess does the necessary post-unmarshal processing for this argument.
// If the argument is a SourceTemplate it compiles the template string into an
// executable template.  This method is idempotent, i.e. it is safe to call
// more than once on the same Argument
func (ha *Argument) postProcess() error {
	if ha.Source == SourceTemplate && ha.template == nil {
		// now compile the template
		var err error
		ha.template, err = template.New("argument").Option("missingkey=zero").Parse(ha.Name)
		return err
	}

	return nil
}

// templateContext is the context passed as "." to the template executed when
// getting an Argument of type SourceTemplate
type templateContext struct {
	ID          string
	ContentType string
	Body        []byte
	Headers     map[string]interface{}
	Query       map[string]interface{}
	Payload     map[string]interface{}
	Method      string
	RemoteAddr  string
}

// BodyText is a convenience to access the request Body as a string.  This means
// you can just say {{ .BodyText }} instead of having to do a trick like
// {{ printf "%s" .Body }}
func (ctx *templateContext) BodyText() string {
	return string(ctx.Body)
}

// GetHeader is a function to fetch a specific item out of the headers map
// by its case insensitive name.  The header name is converted to canonical form
// before being looked up in the header map, e.g. {{ .GetHeader "x-request-id" }}
func (ctx *templateContext) GetHeader(name string) interface{} {
	return ctx.Headers[textproto.CanonicalMIMEHeaderKey(name)]
}

func (ha *Argument) runTemplate(r *Request) (string, error) {
	w := &strings.Builder{}
	ctx := &templateContext{
		r.ID, r.ContentType, r.Body, r.Headers, r.Query, r.Payload, r.RawRequest.Method, r.RawRequest.RemoteAddr,
	}
	err := ha.template.Execute(w, ctx)
	if err == nil {
		return w.String(), nil
	}
	return "", err
}

// Get Argument method returns the value for the Argument's key name
// based on the Argument's source
func (ha *Argument) Get(r *Request) (string, error) {
	var source *map[string]interface{}
	key := ha.Name

	switch ha.Source {
	case SourceHeader:
		source = &r.Headers
		key = textproto.CanonicalMIMEHeaderKey(ha.Name)

	case SourceQuery, SourceQueryAlias:
		source = &r.Query

	case SourcePayload:
		source = &r.Payload

	case SourceString:
		return ha.Name, nil

	case SourceRawRequestBody:
		return string(r.Body), nil

	case SourceRequest:
		if r == nil || r.RawRequest == nil {
			return "", errors.New("request is nil")
		}

		switch strings.ToLower(ha.Name) {
		case "remote-addr":
			return r.RawRequest.RemoteAddr, nil
		case "method":
			return r.RawRequest.Method, nil
		default:
			return "", fmt.Errorf("unsupported request key: %q", ha.Name)
		}

	case SourceEntirePayload:
		res, err := json.Marshal(&r.Payload)
		if err != nil {
			return "", err
		}

		return string(res), nil

	case SourceEntireHeaders:
		res, err := json.Marshal(&r.Headers)
		if err != nil {
			return "", err
		}

		return string(res), nil

	case SourceEntireQuery:
		res, err := json.Marshal(&r.Query)
		if err != nil {
			return "", err
		}

		return string(res), nil

	case SourceTemplate:
		return ha.runTemplate(r)
	}

	if source != nil {
		return ExtractParameterAsString(key, *source)
	}

	return "", errors.New("no source for value retrieval")
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
	TriggerSignatureSoftFailures        bool            `json:"trigger-signature-soft-failures,omitempty"`
	IncomingPayloadContentType          string          `json:"incoming-payload-content-type,omitempty"`
	SuccessHttpResponseCode             int             `json:"success-http-response-code,omitempty"`
	HTTPMethods                         []string        `json:"http-methods"`
}

// ParseJSONParameters decodes specified arguments to JSON objects and replaces the
// string with the newly created object
func (h *Hook) ParseJSONParameters(r *Request) []error {
	errors := make([]error, 0)

	for i := range h.JSONStringParameters {
		arg, err := h.JSONStringParameters[i].Get(r)
		if err != nil {
			errors = append(errors, &ArgumentError{h.JSONStringParameters[i]})
		} else {
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
				source = &r.Headers
			case SourcePayload:
				source = &r.Payload
			case SourceQuery, SourceQueryAlias:
				source = &r.Query
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
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ExtractCommandArguments creates a list of arguments, based on the
// PassArgumentsToCommand property that is ready to be used with exec.Command()
func (h *Hook) ExtractCommandArguments(r *Request) ([]string, []error) {
	args := make([]string, 0)
	errors := make([]error, 0)

	args = append(args, h.ExecuteCommand)

	for i := range h.PassArgumentsToCommand {
		arg, err := h.PassArgumentsToCommand[i].Get(r)
		if err != nil {
			args = append(args, "")
			errors = append(errors, &ArgumentError{h.PassArgumentsToCommand[i]})
			continue
		}

		args = append(args, arg)
	}

	if len(errors) > 0 {
		return args, errors
	}

	return args, nil
}

// ExtractCommandArgumentsForEnv creates a list of arguments in key=value
// format, based on the PassEnvironmentToCommand property that is ready to be used
// with exec.Command().
func (h *Hook) ExtractCommandArgumentsForEnv(r *Request) ([]string, []error) {
	args := make([]string, 0)
	errors := make([]error, 0)
	for i := range h.PassEnvironmentToCommand {
		arg, err := h.PassEnvironmentToCommand[i].Get(r)
		if err != nil {
			errors = append(errors, &ArgumentError{h.PassEnvironmentToCommand[i]})
			continue
		}

		if h.PassEnvironmentToCommand[i].EnvName != "" {
			// first try to use the EnvName if specified
			args = append(args, h.PassEnvironmentToCommand[i].EnvName+"="+arg)
		} else {
			// then fallback on the name
			args = append(args, EnvNamespace+h.PassEnvironmentToCommand[i].Name+"="+arg)
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
func (h *Hook) ExtractCommandArgumentsForFile(r *Request) ([]FileParameter, []error) {
	args := make([]FileParameter, 0)
	errors := make([]error, 0)
	for i := range h.PassFileToCommand {
		arg, err := h.PassFileToCommand[i].Get(r)
		if err != nil {
			errors = append(errors, &ArgumentError{h.PassFileToCommand[i]})
			continue
		}

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
// The delimsStr parameter is a comma-separated pair of the left and right
// template delimiters, or an empty string to use the default '{{,}}'.
func (h *Hooks) LoadFromFile(path string, asTemplate bool, delimsStr string) error {
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
		left, right, found := strings.Cut(delimsStr, ",")
		if !found && delimsStr != "" {
			return fmt.Errorf("invalid delimiters %q - should be left and right delimiters separated by a comma", delimsStr)
		}

		tmpl, err := template.New("hooks").Funcs(funcMap).Delims(strings.TrimSpace(left), strings.TrimSpace(right)).Parse(string(file))
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

	err := yaml.Unmarshal(file, h)
	if err != nil {
		return err
	}

	return h.postProcess()
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

func (h *Hooks) postProcess() error {
	for i := range *h {
		rules := (*h)[i].TriggerRule
		if rules != nil {
			if err := postProcess(rules); err != nil {
				return err
			}
		}
	}
	return nil
}

// Rules is a structure that contains one of the valid rule types
type Rules struct {
	And       *AndRule       `json:"and,omitempty"`
	Or        *OrRule        `json:"or,omitempty"`
	Not       *NotRule       `json:"not,omitempty"`
	Match     *MatchRule     `json:"match,omitempty"`
	Signature *SignatureRule `json:"check-signature,omitempty"`
}

// postProcess is called on each Rules instance after loading it from JSON/YAML,
// to replace any legacy constructs with their modern equivalents.
func postProcess(r *Rules) error {
	if r.And != nil {
		for i := range *(r.And) {
			if err := postProcess(&(*r.And)[i]); err != nil {
				return err
			}
		}
	}
	if r.Or != nil {
		for i := range *(r.Or) {
			if err := postProcess(&(*r.Or)[i]); err != nil {
				return err
			}
		}
	}
	if r.Not != nil {
		return postProcess((*Rules)(r.Not))
	}
	if r.Match != nil {
		// convert any signature matching rules to the equivalent SignatureRule
		if r.Match.Type == MatchHashSHA1 || r.Match.Type == MatchHMACSHA1 {
			log.Printf(`warn: use of deprecated match type %s; use a check-signature rule instead`, r.Match.Type)
			r.Signature = &SignatureRule{
				Algorithm: AlgorithmSHA1,
				Secret:    r.Match.Secret,
				Signature: r.Match.Parameter,
			}
			r.Match = nil
			return nil
		}
		if r.Match.Type == MatchHashSHA256 || r.Match.Type == MatchHMACSHA256 {
			log.Printf(`warn: use of deprecated match type %s; use a check-signature rule instead`, r.Match.Type)
			r.Signature = &SignatureRule{
				Algorithm: AlgorithmSHA256,
				Secret:    r.Match.Secret,
				Signature: r.Match.Parameter,
			}
			r.Match = nil
			return nil
		}
		if r.Match.Type == MatchHashSHA512 || r.Match.Type == MatchHMACSHA512 {
			log.Printf(`warn: use of deprecated match type %s; use a check-signature rule instead`, r.Match.Type)
			r.Signature = &SignatureRule{
				Algorithm: AlgorithmSHA512,
				Secret:    r.Match.Secret,
				Signature: r.Match.Parameter,
			}
			r.Match = nil
			return nil
		}
	}
	return nil
}

// Evaluate finds the first rule property that is not nil and returns the value
// it evaluates to
func (r Rules) Evaluate(req *Request) (bool, error) {
	switch {
	case r.And != nil:
		return r.And.Evaluate(req)
	case r.Or != nil:
		return r.Or.Evaluate(req)
	case r.Not != nil:
		return r.Not.Evaluate(req)
	case r.Match != nil:
		return r.Match.Evaluate(req)
	case r.Signature != nil:
		return r.Signature.Evaluate(req)
	}

	return false, nil
}

// AndRule will evaluate to true if and only if all of the ChildRules evaluate to true
type AndRule []Rules

// Evaluate AndRule will return true if and only if all of ChildRules evaluate to true
func (r AndRule) Evaluate(req *Request) (bool, error) {
	res := true

	for _, v := range r {
		rv, err := v.Evaluate(req)
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
func (r OrRule) Evaluate(req *Request) (bool, error) {
	res := false

	for _, v := range r {
		rv, err := v.Evaluate(req)
		if err != nil {
			if !IsParameterNodeError(err) {
				if !req.AllowSignatureErrors || (req.AllowSignatureErrors && !IsSignatureError(err)) {
					return false, err
				}
			}
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
func (r NotRule) Evaluate(req *Request) (bool, error) {
	rv, err := Rules(r).Evaluate(req)
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
	MatchValue     string = "value"
	MatchRegex     string = "regex"
	IPWhitelist    string = "ip-whitelist"
	ScalrSignature string = "scalr-signature"

	// legacy match types that have migrated to SignatureRule

	MatchHMACSHA1   string = "payload-hmac-sha1"
	MatchHMACSHA256 string = "payload-hmac-sha256"
	MatchHMACSHA512 string = "payload-hmac-sha512"
	MatchHashSHA1   string = "payload-hash-sha1"
	MatchHashSHA256 string = "payload-hash-sha256"
	MatchHashSHA512 string = "payload-hash-sha512"
)

// Evaluate MatchRule will return based on the type
func (r MatchRule) Evaluate(req *Request) (bool, error) {
	if r.Type == IPWhitelist {
		return CheckIPWhitelist(req.RawRequest.RemoteAddr, r.IPRange)
	}
	if r.Type == ScalrSignature {
		return CheckScalrSignature(req, r.Secret, true)
	}

	arg, err := r.Parameter.Get(req)
	if err == nil {
		switch r.Type {
		case MatchValue:
			return compare(arg, r.Value), nil
		case MatchRegex:
			return regexp.MatchString(r.Regex, arg)
		}
	}
	return false, err
}

type SignatureRule struct {
	Algorithm    string    `json:"algorithm,omitempty"`
	Secret       string    `json:"secret,omitempty"`
	Signature    Argument  `json:"signature,omitempty"`
	Prefix       string    `json:"prefix,omitempty"`
	StringToSign *Argument `json:"string-to-sign,omitempty"`
}

// Constants for the SignatureRule type
const (
	AlgorithmSHA1   string = "sha1"
	AlgorithmSHA256 string = "sha256"
	AlgorithmSHA512 string = "sha512"
)

// Evaluate extracts the signature payload and signature value from the request
// and checks whether the signature matches
func (r SignatureRule) Evaluate(req *Request) (bool, error) {
	if r.Secret == "" {
		return false, errors.New("signature validation secret can not be empty")
	}

	var hashConstructor func() hash.Hash
	switch r.Algorithm {
	case AlgorithmSHA1:
		hashConstructor = sha1.New
	case AlgorithmSHA256:
		hashConstructor = sha256.New
	case AlgorithmSHA512:
		hashConstructor = sha512.New
	default:
		return false, fmt.Errorf("unknown hash algorithm %s", r.Algorithm)
	}

	prefix := r.Prefix
	if prefix == "" {
		// default prefix is "sha1=" for SHA1, etc.
		prefix = fmt.Sprintf("%s=", r.Algorithm)
	}

	// find the signature
	sig, err := r.Signature.Get(req)
	if err != nil {
		return false, err
	}

	// determine the payload that is signed
	payload := req.Body
	if r.StringToSign != nil {
		payloadStr, err := r.StringToSign.Get(req)
		if err != nil {
			return false, fmt.Errorf("could not build string-to-sign: %w", err)
		}
		payload = []byte(payloadStr)
	}

	// check the signature
	signatures := ExtractSignatures(sig, prefix)
	_, err = ValidateMAC(payload, hmac.New(hashConstructor, []byte(r.Secret)), signatures)

	return err == nil, err
}

// compare is a helper function for constant time string comparisons.
func compare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// getenv provides a template function to retrieve OS environment variables.
func getenv(s string) string {
	return os.Getenv(s)
}
