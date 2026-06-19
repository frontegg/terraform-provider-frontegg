package provider

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

func TestResourceFronteggJWTTemplateSerialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, map[string]interface{}{
		"key":         "enterprise-template",
		"name":        "Enterprise",
		"description": "An enterprise template",
		"expiration":  3600,
		"algorithm":   "RS256",
		"claims": map[string]interface{}{
			"sub":      "{{sub}}",
			"tenantId": "{{user.tenantId}}",
		},
	})

	got := resourceFronteggJWTTemplateSerialize(d)
	if got.Key != "enterprise-template" || got.Name != "Enterprise" || got.Description != "An enterprise template" {
		t.Errorf("unexpected scalar fields: %+v", got)
	}
	if got.Expiration != 3600 {
		t.Errorf("expiration = %d, want 3600", got.Expiration)
	}
	if got.Algorithm != "RS256" {
		t.Errorf("algorithm = %q, want RS256", got.Algorithm)
	}
	if got.TemplateSchema.Claims["sub"] != "{{sub}}" || got.TemplateSchema.Claims["tenantId"] != "{{user.tenantId}}" {
		t.Errorf("claims not carried into templateSchema: %+v", got.TemplateSchema.Claims)
	}
}

// TestFronteggJWTTemplateClaimsWireFormat asserts the on-the-wire JSON nests
// claims under templateSchema.claims, which is what the Frontegg API expects.
func TestFronteggJWTTemplateClaimsWireFormat(t *testing.T) {
	b, err := json.Marshal(fronteggJWTTemplate{
		Key:            "k",
		Expiration:     60,
		Algorithm:      "RS256",
		TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{"sub": "{{sub}}"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ts, ok := m["templateSchema"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected templateSchema object, got: %s", b)
	}
	claims, ok := ts["claims"].(map[string]interface{})
	if !ok || claims["sub"] != "{{sub}}" {
		t.Errorf("claims not nested under templateSchema.claims: %s", b)
	}
	// expiration must always be serialized (no omitempty), even at zero.
	if _, ok := m["expiration"]; !ok {
		t.Errorf("expiration missing from payload: %s", b)
	}
}

func TestResourceFronteggJWTTemplateDeserialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, map[string]interface{}{})

	in := fronteggJWTTemplate{
		ID:             "tpl-1",
		VendorID:       "vend-1",
		Key:            "k",
		Name:           "n",
		Description:    "d",
		Expiration:     120,
		Algorithm:      "HS256",
		TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{"sub": "{{sub}}", "tenantId": "{{user.tenantId}}"}},
		CreatedAt:      "2024-01-01T00:00:00Z",
		UpdatedAt:      "2024-01-02T00:00:00Z",
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, in); err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	if d.Id() != "tpl-1" {
		t.Errorf("id = %q, want tpl-1", d.Id())
	}
	for field, want := range map[string]string{
		"key":         "k",
		"name":        "n",
		"description": "d",
		"algorithm":   "HS256",
		"vendor_id":   "vend-1",
		"created_at":  "2024-01-01T00:00:00Z",
		"updated_at":  "2024-01-02T00:00:00Z",
	} {
		if got := d.Get(field).(string); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	if d.Get("expiration").(int) != 120 {
		t.Errorf("expiration = %d, want 120", d.Get("expiration").(int))
	}
	claims := d.Get("claims").(map[string]interface{})
	if claims["sub"] != "{{sub}}" || claims["tenantId"] != "{{user.tenantId}}" {
		t.Errorf("claims round-trip mismatch: %+v", claims)
	}
}

// TestResourceFronteggJWTTemplateDeserializeRejectsNonStringClaim ensures the
// deserializer fails loudly rather than silently dropping a non-string claim
// value returned by the API.
func TestResourceFronteggJWTTemplateDeserializeRejectsNonStringClaim(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, map[string]interface{}{})
	in := fronteggJWTTemplate{
		TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{"exp": float64(123)}},
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, in); err == nil {
		t.Fatal("expected an error for a non-string claim value, got nil")
	}
}
