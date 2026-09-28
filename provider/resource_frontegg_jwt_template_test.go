package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func jwtTemplateConfig(claimsAttributes map[string]interface{}) map[string]interface{} {
	config := map[string]interface{}{
		"key":        "k",
		"name":       "n",
		"expiration": 3600,
		"algorithm":  "RS256",
	}
	maps.Copy(config, claimsAttributes)
	return config
}

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
			claims: asClaims("iss", "sub", "aud", "exp", "iat"),
			want:   nil,
		},
		{
			name:   "required claims plus custom extras present",
			claims: asClaims("iss", "sub", "aud", "exp", "iat", "email", "roles"),
			want:   nil,
		},
		{
			name:   "empty claims map missing everything",
			claims: map[string]interface{}{},
			want:   []string{"iss", "sub", "aud", "exp", "iat"},
		},
		{
			name:   "missing reported in canonical order",
			claims: asClaims("sub", "exp"),
			want:   []string{"iss", "aud", "iat"},
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

func TestResourceFronteggJWTTemplateValidateClaims(t *testing.T) {
	requiredClaims := map[string]interface{}{
		"iss": "{{iss}}",
		"sub": "{{sub}}",
		"aud": "{{clientId}}",
		"exp": "{{exp}}",
		"iat": "{{iat}}",
	}
	withClaims := func(extra map[string]string) map[string]interface{} {
		claims := map[string]interface{}{}
		for k, v := range requiredClaims {
			claims[k] = v
		}
		for k, v := range extra {
			claims[k] = v
		}
		return jwtTemplateConfig(map[string]interface{}{"claims": claims})
	}

	claimsState := jwtTemplateState(map[string]string{"claims.%": "1", "claims.sub": "{{sub}}"})
	tests := []struct {
		name    string
		state   *terraform.InstanceState
		raw     map[string]interface{}
		wantErr string
	}{
		{
			name: "tenantId is accepted",
			raw:  withClaims(map[string]string{"tenantId": "{{user.tenantId}}"}),
		},
		{
			name: "type is left to the API",
			raw:  withClaims(map[string]string{"type": "userToken"}),
		},
		{
			name: "custom claims are accepted",
			raw:  withClaims(map[string]string{"email": "{{user.email}}"}),
		},
		{
			name: "missing required claims are rejected at plan time",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims": map[string]interface{}{"email": "{{user.email}}"},
			}),
			wantErr: "missing: iss, sub, aud, exp, iat",
		},
		{
			name: "claims_json with required claims and a nested object is accepted",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims_json": `{"iss":"{{iss}}","sub":"{{sub}}","aud":"{{clientId}}","exp":"{{exp}}","iat":"{{iat}}","org":{"id":"{{user.tenantId}}"}}`,
			}),
		},
		{
			name: "claims_json missing required claims is rejected at plan time",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims_json": `{"iss":"{{iss}}","org":{"id":"{{user.tenantId}}"}}`,
			}),
			wantErr: "claims_json must include the required OIDC claims (iss, sub, aud, exp, iat); missing: sub, aud, exp, iat",
		},
		{
			name: "claims_json null is rejected",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims_json": "null",
			}),
			wantErr: "must be a JSON object, not null",
		},
		{
			name: "unknown claims_json is left to apply",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims_json": unknownValuePlaceholder,
			}),
		},
		{
			name: "unknown claims map is left to apply",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims": unknownValuePlaceholder,
			}),
		},
		{
			name: "known claims_json is checked even when claims is unknown",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims":      unknownValuePlaceholder,
				"claims_json": `{"iss":"{{iss}}"}`,
			}),
			wantErr: "claims_json must include the required OIDC claims (iss, sub, aud, exp, iat); missing: sub, aud, exp, iat",
		},
		{
			name: "known claims is checked even when claims_json is unknown",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims":      map[string]interface{}{"email": "{{user.email}}"},
				"claims_json": unknownValuePlaceholder,
			}),
			wantErr: "claims must include the required OIDC claims (iss, sub, aud, exp, iat); missing: iss, sub, aud, exp, iat",
		},
		{
			name: "both claims and claims_json known are rejected",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims":      map[string]interface{}{"sub": "{{sub}}"},
				"claims_json": `{"sub":"{{sub}}"}`,
			}),
			wantErr: "only one of `claims,claims_json` can be specified",
		},
		{
			name:    "empty claims map reports the missing required claims",
			raw:     jwtTemplateConfig(map[string]interface{}{"claims": map[string]interface{}{}}),
			wantErr: "claims must include the required OIDC claims (iss, sub, aud, exp, iat); missing: iss, sub, aud, exp, iat",
		},
		{
			name:  "update with known claims_json and unknown claims over claims state",
			state: claimsState,
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims":      unknownValuePlaceholder,
				"claims_json": `{"iss":"{{iss}}","sub":"{{sub}}","aud":"{{clientId}}","exp":"{{exp}}","iat":"{{iat}}"}`,
			}),
		},
		{
			name:  "update with unknown claims over claims state",
			state: claimsState,
			raw:   jwtTemplateConfig(map[string]interface{}{"claims": unknownValuePlaceholder}),
		},
		{
			name:  "update with unknown claims_json over claims state",
			state: claimsState,
			raw:   jwtTemplateConfig(map[string]interface{}{"claims_json": unknownValuePlaceholder}),
		},
		{
			name:    "claims_json array resolved at apply is rejected as a non-object",
			raw:     jwtTemplateConfig(map[string]interface{}{"claims_json": `["sub"]`}),
			wantErr: "claims_json must be a valid JSON object",
		},
		{
			name: "claims map with an unknown value is left to apply",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims": map[string]interface{}{"iss": "{{iss}}", "appId": unknownValuePlaceholder},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resourceFronteggJWTTemplate()
			_, err := r.Diff(context.Background(), tt.state, terraform.NewResourceConfigRaw(tt.raw), nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Diff() returned an unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Diff() returned no error, want one")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Diff() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestResourceFronteggJWTTemplateValidateClaimsAttributes(t *testing.T) {
	stringClaims := map[string]interface{}{"sub": "{{sub}}"}
	tests := []struct {
		name    string
		raw     map[string]interface{}
		wantErr string
	}{
		{
			name: "claims only",
			raw:  jwtTemplateConfig(map[string]interface{}{"claims": stringClaims}),
		},
		{
			name: "claims_json only",
			raw:  jwtTemplateConfig(map[string]interface{}{"claims_json": `{"sub":"{{sub}}"}`}),
		},
		{
			name:    "neither",
			raw:     jwtTemplateConfig(nil),
			wantErr: "one of `claims,claims_json` must be specified",
		},
		{
			name: "both",
			raw: jwtTemplateConfig(map[string]interface{}{
				"claims":      stringClaims,
				"claims_json": `{"sub":"{{sub}}"}`,
			}),
			wantErr: "only one of `claims,claims_json` can be specified",
		},
		{
			name:    "claims_json array",
			raw:     jwtTemplateConfig(map[string]interface{}{"claims_json": `["sub"]`}),
			wantErr: "must be a valid JSON object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := resourceFronteggJWTTemplate().Validate(terraform.NewResourceConfigRaw(tt.raw))
			if tt.wantErr == "" {
				if diags.HasError() {
					t.Fatalf("Validate() returned unexpected errors: %v", diags)
				}
				return
			}
			var messages []string
			for _, diagnostic := range diags {
				messages = append(messages, diagnostic.Summary+": "+diagnostic.Detail)
			}
			if joined := strings.Join(messages, "\n"); !strings.Contains(joined, tt.wantErr) {
				t.Errorf("Validate() errors = %q, want one containing %q", joined, tt.wantErr)
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
			"sub":   "{{sub}}",
			"email": "{{user.email}}",
		},
	})

	got, err := resourceFronteggJWTTemplateSerialize(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if got.Key != "enterprise-template" || got.Name != "Enterprise" || got.Description != "An enterprise template" {
		t.Errorf("unexpected scalar fields: %+v", got)
	}
	if got.Expiration != 3600 {
		t.Errorf("expiration = %d, want 3600", got.Expiration)
	}
	if got.Algorithm != "RS256" {
		t.Errorf("algorithm = %q, want RS256", got.Algorithm)
	}
	if got.TemplateSchema.Claims["sub"] != "{{sub}}" || got.TemplateSchema.Claims["email"] != "{{user.email}}" {
		t.Errorf("claims not carried into templateSchema: %+v", got.TemplateSchema.Claims)
	}
}

func TestResourceFronteggJWTTemplateSerializeClaimsJSON(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, jwtTemplateConfig(map[string]interface{}{
		"claims_json": `{"sub":"{{sub}}","accountNumber":42,"org":{"id":"{{user.tenantId}}","roles":["admin"]}}`,
	}))

	got, err := resourceFronteggJWTTemplateSerialize(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want := map[string]interface{}{
		"sub":           "{{sub}}",
		"accountNumber": float64(42),
		"org":           map[string]interface{}{"id": "{{user.tenantId}}", "roles": []interface{}{"admin"}},
	}
	if !reflect.DeepEqual(got.TemplateSchema.Claims, want) {
		t.Errorf("claims = %+v, want %+v", got.TemplateSchema.Claims, want)
	}
}

func TestResourceFronteggJWTTemplateSerializeRejectsNullClaimsJSON(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, jwtTemplateConfig(map[string]interface{}{
		"claims_json": "null",
	}))
	if _, err := resourceFronteggJWTTemplateSerialize(d); err == nil || !strings.Contains(err.Error(), "not null") {
		t.Errorf("serialize error = %v, want a null claims_json error", err)
	}
}

func jwtTemplateState(stateAttributes map[string]string) *terraform.InstanceState {
	attributes := map[string]string{"id": "tpl-1", "key": "k", "name": "n", "expiration": "3600", "algorithm": "RS256"}
	maps.Copy(attributes, stateAttributes)
	return &terraform.InstanceState{ID: "tpl-1", Attributes: attributes}
}

func jwtTemplateUpdateData(t *testing.T, stateAttributes map[string]string, config map[string]interface{}) *schema.ResourceData {
	jwtTemplateResource := resourceFronteggJWTTemplate()
	state := jwtTemplateState(stateAttributes)
	diff, err := jwtTemplateResource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(jwtTemplateConfig(config)), nil)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	data, err := schema.InternalMap(jwtTemplateResource.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	return data
}

func TestResourceFronteggJWTTemplateUpdateSwitchesClaimsAttribute(t *testing.T) {
	claimsMap := map[string]interface{}{"aud": "{{clientId}}", "exp": "{{exp}}", "iat": "{{iat}}", "iss": "{{iss}}", "sub": "{{sub}}"}
	claimsJSON, err := structure.FlattenJsonToString(claimsMap)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	claimsMapState := map[string]string{"claims.%": "5"}
	for claim, value := range claimsMap {
		claimsMapState["claims."+claim] = value.(string)
	}
	response := fronteggJWTTemplate{ID: "tpl-1", TemplateSchema: fronteggJWTTemplateSchema{Claims: claimsMap}}

	tests := []struct {
		name           string
		state          map[string]string
		config         map[string]interface{}
		wantClaimsJSON string
		wantClaims     map[string]interface{}
	}{
		{
			name:           "claims to claims_json",
			state:          claimsMapState,
			config:         map[string]interface{}{"claims_json": claimsJSON},
			wantClaimsJSON: claimsJSON,
			wantClaims:     map[string]interface{}{},
		},
		{
			name:           "claims_json to claims",
			state:          map[string]string{"claims_json": claimsJSON},
			config:         map[string]interface{}{"claims": claimsMap},
			wantClaimsJSON: "",
			wantClaims:     claimsMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := jwtTemplateUpdateData(t, tt.state, tt.config)
			request, err := resourceFronteggJWTTemplateSerialize(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if !reflect.DeepEqual(request.TemplateSchema.Claims, claimsMap) {
				t.Errorf("claims sent = %+v, want %+v", request.TemplateSchema.Claims, claimsMap)
			}
			if err := resourceFronteggJWTTemplateDeserialize(d, response); err != nil {
				t.Fatalf("deserialize: %v", err)
			}
			if got := d.Get("claims_json").(string); got != tt.wantClaimsJSON {
				t.Errorf("claims_json = %q, want %q", got, tt.wantClaimsJSON)
			}
			if claims := d.Get("claims").(map[string]interface{}); !reflect.DeepEqual(claims, tt.wantClaims) {
				t.Errorf("claims = %+v, want %+v", claims, tt.wantClaims)
			}
		})
	}
}

func TestResourceFronteggJWTTemplateClaimsJSONPlannedValueIsNormalized(t *testing.T) {
	config := jwtTemplateConfig(map[string]interface{}{
		"claims_json": `{ "sub": "{{sub}}", "iss": "{{iss}}", "aud": "{{clientId}}", "exp": "{{exp}}", "iat": "{{iat}}" }`,
	})
	diff, err := resourceFronteggJWTTemplate().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	want := `{"aud":"{{clientId}}","exp":"{{exp}}","iat":"{{iat}}","iss":"{{iss}}","sub":"{{sub}}"}`
	if got := diff.Attributes["claims_json"].New; got != want {
		t.Errorf("planned claims_json = %s, want %s", got, want)
	}
}

func TestResourceFronteggJWTTemplateReformattedClaimsJSONHasNoDiff(t *testing.T) {
	jwtTemplateResource := resourceFronteggJWTTemplate()
	storedClaims, err := structure.FlattenJsonToString(map[string]interface{}{
		"iss": "{{iss}}", "sub": "{{sub}}", "aud": "{{clientId}}", "exp": "{{exp}}", "iat": "{{iat}}",
		"org": map[string]interface{}{"id": "{{user.tenantId}}"},
	})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	state := jwtTemplateState(map[string]string{"claims_json": storedClaims})
	config := jwtTemplateConfig(map[string]interface{}{
		"claims_json": "{\n  \"org\": {\"id\": \"{{user.tenantId}}\"},\n  \"sub\": \"{{sub}}\", \"iss\": \"{{iss}}\",\n  \"aud\": \"{{clientId}}\", \"iat\": \"{{iat}}\", \"exp\": \"{{exp}}\"\n}",
	})
	diff, err := jwtTemplateResource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff != nil && len(diff.Attributes) > 0 {
		t.Errorf("unexpected diff: %+v", diff.Attributes)
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
		TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{"sub": "{{sub}}", "email": "{{user.email}}"}},
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
	if claims["sub"] != "{{sub}}" || claims["email"] != "{{user.email}}" {
		t.Errorf("claims round-trip mismatch: %+v", claims)
	}
	if claimsJSON := d.Get("claims_json").(string); claimsJSON != "" {
		t.Errorf("claims_json = %q, want empty", claimsJSON)
	}
}

// TestResourceFronteggJWTTemplateDeserializeNonStringClaim covers object claims saved from the portal.
func TestResourceFronteggJWTTemplateDeserializeNonStringClaim(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, map[string]interface{}{})
	in := fronteggJWTTemplate{
		TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{
			"sub":      "{{sub}}",
			"org":      map[string]interface{}{"id": "{{user.tenantId}}"},
			"level":    float64(3),
			"verified": true,
		}},
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, in); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if got, want := d.Get("claims_json").(string), `{"level":3,"org":{"id":"{{user.tenantId}}"},"sub":"{{sub}}","verified":true}`; got != want {
		t.Errorf("claims_json = %s, want %s", got, want)
	}
	if claims := d.Get("claims").(map[string]interface{}); len(claims) != 0 {
		t.Errorf("claims = %+v, want empty", claims)
	}
}

func TestResourceFronteggJWTTemplateDeserializeMissingClaimsClearsClaimsJSON(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplate().Schema, map[string]interface{}{
		"claims_json": `{"sub":"{{sub}}"}`,
	})
	if err := resourceFronteggJWTTemplateDeserialize(d, fronteggJWTTemplate{}); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if got := d.Get("claims_json").(string); got != "" {
		t.Errorf("claims_json = %q, want empty", got)
	}
}

func TestResourceFronteggJWTTemplateDeserializeClearsClaimsStateOnNonStringDrift(t *testing.T) {
	state := jwtTemplateState(map[string]string{
		"claims.%":   "2",
		"claims.sub": "{{sub}}",
		"claims.iss": "{{iss}}",
	})
	d := resourceFronteggJWTTemplate().Data(state)
	response := fronteggJWTTemplate{ID: "tpl-1", TemplateSchema: fronteggJWTTemplateSchema{Claims: map[string]interface{}{
		"sub": "{{sub}}",
		"org": map[string]interface{}{"id": "{{user.tenantId}}"},
	}}}
	if err := resourceFronteggJWTTemplateDeserialize(d, response); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if got, want := d.Get("claims_json").(string), `{"org":{"id":"{{user.tenantId}}"},"sub":"{{sub}}"}`; got != want {
		t.Errorf("claims_json = %s, want %s", got, want)
	}
	if claims := d.Get("claims").(map[string]interface{}); len(claims) != 0 {
		t.Errorf("claims = %+v, want cleared", claims)
	}
	if count := d.State().Attributes["claims.%"]; count != "" && count != "0" {
		t.Errorf("claims.%% in state = %q, want cleared", count)
	}
}

const testAccJWTTemplateWithTenantID = `
resource "frontegg_jwt_template" "test" {
  key         = "tf-acc-tenant-id"
  name        = "TF acceptance tenantId"
  description = "FR-26867 regression coverage"
  expiration  = 3600
  algorithm   = "RS256"

  claims = {
    iss      = "{{iss}}"
    sub      = "{{sub}}"
    aud      = "{{clientId}}"
    exp      = "{{exp}}"
    iat      = "{{iat}}"
    tenantId = "{{user.tenantId}}"
    email    = "{{user.email}}"
  }
}
`

func TestAccFronteggJWTTemplate_tenantIDClaimIsAccepted(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckJWTTemplateDestroyed(t),
		Steps: []resource.TestStep{
			{
				Config: testAccJWTTemplateWithTenantID,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_jwt_template.test", "claims.tenantId", "{{user.tenantId}}"),
					resource.TestCheckResourceAttrSet("frontegg_jwt_template.test", "id"),
					resource.TestCheckResourceAttrSet("frontegg_jwt_template.test", "vendor_id"),
				),
			},
			{
				Config:   testAccJWTTemplateWithTenantID,
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_jwt_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testAccJWTTemplateWithStringClaims = `
resource "frontegg_jwt_template" "test" {
  key        = "tf-acc-nested-claim"
  name       = "TF acceptance nested claim"
  expiration = 3600
  algorithm  = "RS256"

  claims = {
    iss = "{{iss}}"
    sub = "{{sub}}"
    aud = "{{clientId}}"
    exp = "{{exp}}"
    iat = "{{iat}}"
  }
}
`

const testAccJWTTemplateWithNestedClaim = `
resource "frontegg_jwt_template" "test" {
  key        = "tf-acc-nested-claim"
  name       = "TF acceptance nested claim"
  expiration = 3600
  algorithm  = "RS256"

  claims_json = jsonencode({
    iss = "{{iss}}"
    sub = "{{sub}}"
    aud = "{{clientId}}"
    exp = "{{exp}}"
    iat = "{{iat}}"
    org = {
      id   = "{{user.tenantId}}"
      name = "static"
    }
    level    = 3
    verified = true
    roles    = ["admin", "viewer"]
  })
}
`

func TestAccFronteggJWTTemplate_nestedObjectClaim(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckJWTTemplateDestroyed(t),
		Steps: []resource.TestStep{
			{
				Config: testAccJWTTemplateWithStringClaims,
				Check:  resource.TestCheckResourceAttr("frontegg_jwt_template.test", "claims.aud", "{{clientId}}"),
			},
			{
				Config: testAccJWTTemplateWithNestedClaim,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("frontegg_jwt_template.test", "id"),
					resource.TestCheckNoResourceAttr("frontegg_jwt_template.test", "claims.%"),
					resource.TestCheckResourceAttr("frontegg_jwt_template.test", "claims_json",
						`{"aud":"{{clientId}}","exp":"{{exp}}","iat":"{{iat}}","iss":"{{iss}}","level":3,"org":{"id":"{{user.tenantId}}","name":"static"},"roles":["admin","viewer"],"sub":"{{sub}}","verified":true}`),
				),
			},
			{
				Config:   testAccJWTTemplateWithNestedClaim,
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_jwt_template.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccJWTTemplateWithStringClaims,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_jwt_template.test", "claims.aud", "{{clientId}}"),
					resource.TestCheckResourceAttr("frontegg_jwt_template.test", "claims_json", ""),
				),
			},
		},
	})
}

func testAccCheckJWTTemplateDestroyed(t *testing.T) func(*terraform.State) error {
	return func(s *terraform.State) error {
		base := os.Getenv("FRONTEGG_API_BASE_URL")
		if base == "" {
			base = "https://api.frontegg.com"
		}
		token := fronteggVendorToken(t, base)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "frontegg_jwt_template" {
				continue
			}
			req, err := http.NewRequest(http.MethodGet, base+fronteggJWTTemplatePath+"/"+rs.Primary.ID, nil)
			if err != nil {
				return fmt.Errorf("build jwt template lookup: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("look up jwt template %s: %w", rs.Primary.ID, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				return fmt.Errorf("jwt template %s still exists after destroy (HTTP %d)", rs.Primary.ID, resp.StatusCode)
			}
		}
		return nil
	}
}
