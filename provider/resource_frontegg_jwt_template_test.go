package provider

import (
	"reflect"
	"testing"
)

func TestMissingRequiredClaims(t *testing.T) {
	asClaims := func(keys ...string) map[string]interface{} {
		m := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			m[k] = "{{" + k + "}}"
		}
		return m
	}

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   []string
	}{
		{
			name:   "all required claims present",
			claims: asClaims("iss", "sub", "aud", "exp", "iat", "type", "tenantId"),
			want:   nil,
		},
		{
			name:   "required claims plus extras present",
			claims: asClaims("iss", "sub", "aud", "exp", "iat", "type", "tenantId", "email", "roles"),
			want:   nil,
		},
		{
			name:   "empty claims map missing everything",
			claims: map[string]interface{}{},
			want:   []string{"iss", "sub", "aud", "exp", "iat", "type", "tenantId"},
		},
		{
			name:   "missing only Frontegg claims",
			claims: asClaims("iss", "sub", "aud", "exp", "iat"),
			want:   []string{"type", "tenantId"},
		},
		{
			name:   "missing reported in canonical order",
			claims: asClaims("sub", "exp", "type"),
			want:   []string{"iss", "aud", "iat", "tenantId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingRequiredClaims(tt.claims); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("missingRequiredClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}
