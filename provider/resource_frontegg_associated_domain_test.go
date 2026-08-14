package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func associatedDomainResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggAssociatedDomain().Schema, raw)
}

func TestParseAssociatedDomainsArray(t *testing.T) {
	raw := []byte(`[
		{"id": "cfg-1", "appId": "ABCDE12345.com.example.app"},
		{"id": "cfg-2", "packageName": "com.example.app", "sha256CertFingerprints": ["AA:BB"]}
	]`)
	got, err := parseAssociatedDomains(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].configurationId() != "cfg-1" || got[0].AppId != "ABCDE12345.com.example.app" {
		t.Errorf("first config not decoded: %+v", got[0])
	}
	if got[1].PackageName != "com.example.app" || len(got[1].Sha256CertFingerprints) != 1 {
		t.Errorf("second config not decoded: %+v", got[1])
	}
}

func TestParseAssociatedDomainsSingleObject(t *testing.T) {
	raw := []byte(`{"configurationId": "cfg-9", "appId": "ABCDE12345.com.example.app"}`)
	got, err := parseAssociatedDomains(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].configurationId() != "cfg-9" {
		t.Errorf("single object not decoded: %+v", got)
	}
}

func TestParseAssociatedDomainsRejectsGarbage(t *testing.T) {
	if _, err := parseAssociatedDomains([]byte(`"just a string"`)); err == nil {
		t.Error("expected an error for a non-object response")
	}
}

func TestConfigurationIdPrefersId(t *testing.T) {
	f := fronteggAssociatedDomain{Id: "a", ConfigurationId: "b"}
	if got := f.configurationId(); got != "a" {
		t.Errorf("configurationId() = %q, want %q", got, "a")
	}
	f = fronteggAssociatedDomain{ConfigurationId: "b"}
	if got := f.configurationId(); got != "b" {
		t.Errorf("configurationId() = %q, want %q", got, "b")
	}
}

func TestAssociatedDomainMatches(t *testing.T) {
	ios := associatedDomainResourceData(t, map[string]interface{}{
		"platform": "ios",
		"app_id":   "ABCDE12345.com.example.app",
	})
	if !resourceFronteggAssociatedDomainMatches(ios, fronteggAssociatedDomain{AppId: "ABCDE12345.com.example.app"}) {
		t.Error("ios config should match on app_id")
	}
	if resourceFronteggAssociatedDomainMatches(ios, fronteggAssociatedDomain{AppId: "OTHER.com.example.app"}) {
		t.Error("ios config should not match a different app_id")
	}
	if resourceFronteggAssociatedDomainMatches(ios, fronteggAssociatedDomain{PackageName: "com.example.app"}) {
		t.Error("ios config should not match an android entry")
	}

	android := associatedDomainResourceData(t, map[string]interface{}{
		"platform":     "android",
		"package_name": "com.example.app",
	})
	if !resourceFronteggAssociatedDomainMatches(android, fronteggAssociatedDomain{PackageName: "com.example.app"}) {
		t.Error("android config should match on package_name")
	}
	if resourceFronteggAssociatedDomainMatches(android, fronteggAssociatedDomain{AppId: "ABCDE12345.com.example.app"}) {
		t.Error("android config should not match an ios entry")
	}
}

func TestAssociatedDomainImportIdParsing(t *testing.T) {
	for _, tc := range []struct {
		id           string
		wantPlatform string
		wantId       string
		wantErr      bool
	}{
		{id: "ios/cfg-1", wantPlatform: "ios", wantId: "cfg-1"},
		{id: "android/cfg-2", wantPlatform: "android", wantId: "cfg-2"},
		{id: "cfg-3", wantErr: true},
		{id: "web/cfg-4", wantErr: true},
		{id: "ios/", wantErr: true},
	} {
		d := associatedDomainResourceData(t, map[string]interface{}{})
		d.SetId(tc.id)
		got, err := resourceFronteggAssociatedDomainImport(nil, d, nil)
		if tc.wantErr {
			if err == nil {
				t.Errorf("import %q: expected error", tc.id)
			}
			continue
		}
		if err != nil {
			t.Errorf("import %q: %v", tc.id, err)
			continue
		}
		if got[0].Get("platform").(string) != tc.wantPlatform || got[0].Id() != tc.wantId {
			t.Errorf("import %q: platform=%q id=%q", tc.id, got[0].Get("platform"), got[0].Id())
		}
	}
}
