package helpers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// CheckPayloadSignature calculates and verifies SHA1 signature of the given payload
func CheckPayloadSignature(payload []byte, secret string, signature string) (string, bool) {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return expectedMAC, hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// FormValuesToMap converts url.Values to a map[string]interface{} object
func FormValuesToMap(formValues url.Values) map[string]interface{} {
	ret := make(map[string]interface{})

	for key, value := range formValues {
		if len(value) > 0 {
			ret[key] = value[0]
		}
	}

	return ret
}

// ExtractJSONParameter extracts value from payload based on the passed string
func ExtractJSONParameter(s string, params interface{}) (string, bool) {
	var p []string

	if paramsValue := reflect.ValueOf(params); paramsValue.Kind() == reflect.Slice {
		if paramsValueSliceLength := paramsValue.Len(); paramsValueSliceLength > 0 {

			if p = strings.SplitN(s, ".", 3); len(p) > 3 {
				index, err := strconv.ParseInt(p[1], 10, 64)

				if err != nil {
					return "", false
				} else if paramsValueSliceLength <= int(index) {
					return "", false
				}

				return ExtractJSONParameter(p[2], params.([]map[string]interface{})[index])
			}
		}

		return "", false
	}

	if p = strings.SplitN(s, ".", 2); len(p) > 1 {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return ExtractJSONParameter(p[1], pValue)
		}
	} else {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return fmt.Sprintf("%v", pValue), true
		}
	}

	return "", false
}
