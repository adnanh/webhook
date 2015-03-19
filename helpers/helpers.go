package helpers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

// ValuesToMap converts map[string][]string to a map[string]string object
func ValuesToMap(values map[string][]string) map[string]interface{} {
	ret := make(map[string]interface{})

	for key, value := range values {
		if len(value) > 0 {
			ret[key] = value[0]
		}
	}

	return ret
}

// ExtractParameter extracts value from interface{} based on the passed string
func ExtractParameter(s string, params interface{}) (string, bool) {
	if params == nil {
		return "", false
	}

	if paramsValue := reflect.ValueOf(params); paramsValue.Kind() == reflect.Slice {
		if paramsValueSliceLength := paramsValue.Len(); paramsValueSliceLength > 0 {

			if p := strings.SplitN(s, ".", 2); len(p) > 1 {
				index, err := strconv.ParseInt(p[0], 10, 64)

				if err != nil {
					return "", false
				} else if paramsValueSliceLength <= int(index) {
					return "", false
				}

				return ExtractParameter(p[1], params.([]interface{})[index])
			}
		}

		return "", false
	}

	if p := strings.SplitN(s, ".", 2); len(p) > 1 {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return ExtractParameter(p[1], pValue)
		}
	} else {
		if pValue, ok := params.(map[string]interface{})[p[0]]; ok {
			return fmt.Sprintf("%v", pValue), true
		}
	}

	return "", false
}
