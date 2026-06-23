package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExpandFronteggPasswordTests(t *testing.T) {
	// Omitted/empty blocks must expand to nil so the field is not sent on the
	// wire, preserving behavior for configs that do not use the new blocks.
	if got := expandFronteggPasswordTests(nil); got != nil {
		t.Errorf("nil input: got %+v, want nil", got)
	}
	if got := expandFronteggPasswordTests([]interface{}{}); got != nil {
		t.Errorf("empty input: got %+v, want nil", got)
	}

	raw := []interface{}{map[string]interface{}{
		"require_lowercase":          true,
		"require_uppercase":          false,
		"require_numbers":            true,
		"require_special_chars":      false,
		"check_three_repeated_chars": true,
	}}
	got := expandFronteggPasswordTests(raw)
	if got == nil {
		t.Fatal("expected a non-nil result")
	}
	want := fronteggPasswordTests{RequireLowercase: true, RequireNumbers: true, CheckThreeRepeatedChars: true}
	if *got != want {
		t.Errorf("expand = %+v, want %+v", *got, want)
	}
}

func TestFlattenFronteggPasswordTests(t *testing.T) {
	if got := flattenFronteggPasswordTests(nil); len(got) != 0 {
		t.Errorf("nil input: got %v, want empty slice", got)
	}

	in := &fronteggPasswordTests{RequireUppercase: true, RequireSpecialChars: true}
	got := flattenFronteggPasswordTests(in)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	m := got[0].(map[string]interface{})
	if m["require_uppercase"] != true || m["require_special_chars"] != true {
		t.Errorf("flatten true fields wrong: %+v", m)
	}
	if m["require_lowercase"] != false || m["require_numbers"] != false || m["check_three_repeated_chars"] != false {
		t.Errorf("flatten false fields wrong: %+v", m)
	}
}

func TestFronteggPasswordTestsRoundTrip(t *testing.T) {
	in := &fronteggPasswordTests{
		RequireLowercase:        true,
		RequireUppercase:        true,
		RequireNumbers:          true,
		RequireSpecialChars:     true,
		CheckThreeRepeatedChars: true,
	}
	got := expandFronteggPasswordTests(flattenFronteggPasswordTests(in))
	if got == nil || *got != *in {
		t.Errorf("round trip = %+v, want %+v", got, in)
	}
}

func TestValidateFronteggPasswordTests(t *testing.T) {
	tests := []struct {
		name            string
		optional        *fronteggPasswordTests
		required        *fronteggPasswordTests
		wantErr         bool
		wantErrContains string
	}{
		{name: "both nil", optional: nil, required: nil, wantErr: false},
		{
			name:     "disjoint tests",
			optional: &fronteggPasswordTests{RequireLowercase: true, RequireUppercase: true},
			required: &fronteggPasswordTests{CheckThreeRepeatedChars: true},
			wantErr:  false,
		},
		{
			name:            "same test in both",
			optional:        &fronteggPasswordTests{RequireNumbers: true},
			required:        &fronteggPasswordTests{RequireNumbers: true},
			wantErr:         true,
			wantErrContains: "require_numbers",
		},
		{
			name:     "optional nil with required set",
			optional: nil,
			required: &fronteggPasswordTests{RequireNumbers: true},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFronteggPasswordTests(tt.optional, tt.required)
			if tt.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrContains)) {
				t.Errorf("error %v does not contain %q", err, tt.wantErrContains)
			}
		})
	}
}

// TestFronteggPasswordPolicyOmitsTestsWhenNil verifies backward compatibility:
// a policy without the new blocks must not emit optionalTests/requiredTests.
func TestFronteggPasswordPolicyOmitsTestsWhenNil(t *testing.T) {
	b, err := json.Marshal(fronteggPasswordPolicy{MinLength: 8, MaxLength: 128})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["optionalTests"]; ok {
		t.Errorf("optionalTests should be omitted when nil: %s", b)
	}
	if _, ok := m["requiredTests"]; ok {
		t.Errorf("requiredTests should be omitted when nil: %s", b)
	}
}

// TestFronteggPasswordPolicyTestsWireFormat asserts the camelCase JSON keys the
// Frontegg password configuration API expects.
func TestFronteggPasswordPolicyTestsWireFormat(t *testing.T) {
	b, err := json.Marshal(fronteggPasswordPolicy{
		OptionalTests: &fronteggPasswordTests{RequireLowercase: true, RequireSpecialChars: true},
		RequiredTests: &fronteggPasswordTests{CheckThreeRepeatedChars: true},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	opt, ok := m["optionalTests"].(map[string]interface{})
	if !ok {
		t.Fatalf("optionalTests missing or wrong type: %s", b)
	}
	if opt["requireLowercase"] != true || opt["requireSpecialChars"] != true {
		t.Errorf("optionalTests values wrong: %s", b)
	}
	for _, key := range []string{"requireLowercase", "requireUppercase", "requireNumbers", "requireSpecialChars", "checkThreeRepeatedChars"} {
		if _, ok := opt[key].(bool); !ok {
			t.Errorf("optionalTests missing expected key %q: %s", key, b)
		}
	}
	req, ok := m["requiredTests"].(map[string]interface{})
	if !ok || req["checkThreeRepeatedChars"] != true {
		t.Errorf("requiredTests wire format wrong: %s", b)
	}
}
