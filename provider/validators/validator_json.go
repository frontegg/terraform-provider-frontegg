package validators

import (
	"encoding/json"
	"fmt"
)

// ValidateJSON validates that the value is a valid JSON object.
// Arrays and other JSON primitives are rejected since this validator
// is intended for JWT claim metadata fields that must be key-value objects.
func ValidateJSON(v interface{}, k string) (warns []string, errs []error) {
	val := v.(string)
	if val == "" {
		return
	}
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(val), &js); err != nil {
		errs = append(errs, fmt.Errorf("%q must be a valid JSON object: %s", k, err))
	}
	return
}
