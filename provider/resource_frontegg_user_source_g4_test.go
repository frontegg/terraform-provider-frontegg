package provider

import (
	"encoding/json"
	"testing"
)

// --- Federation wire format ---

func TestFederationUserSourceWireFormatOmitsIsMigrated(t *testing.T) {
	req := fronteggFederationUserSourceRequest{
		Name: "fed-source",
		Configuration: fronteggFederationUserSourceConfig{
			SyncOnLogin:  false,
			TenantConfig: UserSourceNewTenantConfig{TenantResolverType: "new"},
			WellknownURL: "https://idp/.well-known/openid-configuration",
			ClientID:     "client-1",
			Secret:       "sec",
		},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg := m["configuration"].(map[string]interface{})
	if _, ok := cfg["isMigrated"]; ok {
		t.Errorf("federation config must NOT emit isMigrated: %s", b)
	}
	for _, key := range []string{"wellknownUrl", "clientId", "secret", "syncOnLogin", "tenantConfig"} {
		if _, ok := cfg[key]; !ok {
			t.Errorf("configuration missing key %q: %s", key, b)
		}
	}
	// oauth2Config was nil -> omitted.
	if _, ok := cfg["oauth2Config"]; ok {
		t.Errorf("oauth2Config should be omitted when nil: %s", b)
	}
}

func TestFederationOauth2ConfigWireFormat(t *testing.T) {
	req := fronteggFederationUserSourceRequest{
		Name: "fed-oauth2",
		Configuration: fronteggFederationUserSourceConfig{
			TenantConfig: UserSourceNewTenantConfig{TenantResolverType: "new"},
			Oauth2Config: &fronteggFederationOauth2Config{
				AuthorizationURL: "https://idp/authorize",
				TokenURL:         "https://idp/token",
				UserInfoURL:      "https://idp/userinfo",
				Scopes:           []string{"openid", "email"},
			},
			ClientID: "client-1",
			UsePkce:  true,
		},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg := m["configuration"].(map[string]interface{})
	oauth2, ok := cfg["oauth2Config"].(map[string]interface{})
	if !ok {
		t.Fatalf("oauth2Config missing: %s", b)
	}
	for _, key := range []string{"authorizationUrl", "tokenUrl", "userInfoUrl", "scopes"} {
		if _, ok := oauth2[key]; !ok {
			t.Errorf("oauth2Config missing key %q: %s", key, b)
		}
	}
	if _, ok := cfg["wellknownUrl"]; ok {
		t.Errorf("wellknownUrl should be omitted when empty: %s", b)
	}
	// secret empty + usePkce true -> secret omitted.
	if _, ok := cfg["secret"]; ok {
		t.Errorf("secret should be omitted when empty: %s", b)
	}
}
