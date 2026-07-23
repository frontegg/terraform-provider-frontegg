package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func prehookResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggPrehook().Schema, raw)
}

func TestPrehookSerializeAPI(t *testing.T) {
	d := prehookResourceData(t, map[string]interface{}{
		"enabled":     true,
		"name":        "api hook",
		"description": "desc",
		"type":        fronteggPrehookTypeAPI,
		"url":         "https://example.com/hook",
		"secret":      "sh",
		"events":      []interface{}{"SIGN_UP"},
		"fail_method": "CLOSE",
	})

	got := resourceFronteggPrehookSerialize(d)
	if got.Type != fronteggPrehookTypeAPI {
		t.Errorf("type = %q, want %q", got.Type, fronteggPrehookTypeAPI)
	}
	if got.URL != "https://example.com/hook" || got.Secret != "sh" {
		t.Errorf("url/secret not serialized: %+v", got)
	}
	if got.Code != "" || got.Runtime != "" || got.Timeout != 0 || got.EventKey != "" {
		t.Errorf("API prehook must not carry custom-code fields: %+v", got)
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"url", "secret", "eventKeys", "failMethod"} {
		if _, ok := m[key]; !ok {
			t.Errorf("API wire format missing %q: %s", key, b)
		}
	}
	for _, key := range []string{"code", "runtime", "timeout", "eventKey", "executorIdentifier"} {
		if _, ok := m[key]; ok {
			t.Errorf("API wire format must omit %q: %s", key, b)
		}
	}
}

func TestPrehookSerializeCustomCodeDefaults(t *testing.T) {
	d := prehookResourceData(t, map[string]interface{}{
		"enabled":     true,
		"name":        "code hook",
		"description": "desc",
		"type":        fronteggPrehookTypeCustomCode,
		"code":        "exports.onEvent = async () => ({ verdict: 'allow' });",
		"events":      []interface{}{"USER_INVITE"},
		"fail_method": "OPEN",
	})

	got := resourceFronteggPrehookSerialize(d)
	if got.Type != fronteggPrehookTypeCustomCode {
		t.Errorf("type = %q, want %q", got.Type, fronteggPrehookTypeCustomCode)
	}
	if got.Runtime != fronteggPrehookDefaultRuntime {
		t.Errorf("runtime default = %q, want %q", got.Runtime, fronteggPrehookDefaultRuntime)
	}
	if got.Timeout != fronteggPrehookDefaultTimeout {
		t.Errorf("timeout default = %d, want %d", got.Timeout, fronteggPrehookDefaultTimeout)
	}
	if got.EventKey != "USER_INVITE" {
		t.Errorf("eventKey = %q, want single event mirror", got.EventKey)
	}
	if got.URL != "" || got.Secret != "" {
		t.Errorf("custom-code prehook must not carry url/secret: %+v", got)
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"code", "runtime", "timeout", "eventKey", "eventKeys", "failMethod"} {
		if _, ok := m[key]; !ok {
			t.Errorf("custom-code wire format missing %q: %s", key, b)
		}
	}
	for _, key := range []string{"url", "secret"} {
		if _, ok := m[key]; ok {
			t.Errorf("custom-code wire format must omit %q: %s", key, b)
		}
	}
}

func TestPrehookSerializeCustomCodeExplicit(t *testing.T) {
	d := prehookResourceData(t, map[string]interface{}{
		"enabled":     true,
		"description": "desc",
		"type":        fronteggPrehookTypeCustomCode,
		"code":        "code",
		"runtime":     "NODE_18",
		"timeout":     5,
		"events":      []interface{}{"SIGN_UP"},
		"fail_method": "OPEN",
	})

	got := resourceFronteggPrehookSerialize(d)
	if got.Runtime != "NODE_18" {
		t.Errorf("runtime = %q, want NODE_18", got.Runtime)
	}
	if got.Timeout != 5 {
		t.Errorf("timeout = %d, want 5", got.Timeout)
	}
}

func TestPrehookDefaultTypeIsAPI(t *testing.T) {
	d := prehookResourceData(t, map[string]interface{}{
		"enabled":     true,
		"description": "desc",
		"url":         "https://example.com/hook",
		"secret":      "s",
		"events":      []interface{}{"SIGN_UP"},
		"fail_method": "CLOSE",
	})

	got := resourceFronteggPrehookSerialize(d)
	if got.Type != fronteggPrehookTypeAPI {
		t.Errorf("default type = %q, want %q", got.Type, fronteggPrehookTypeAPI)
	}
}

func TestPrehookCustomizeDiffRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		raw     map[string]interface{}
		wantErr bool
	}{
		{
			name:    "api missing url",
			raw:     map[string]interface{}{"type": fronteggPrehookTypeAPI, "secret": "s"},
			wantErr: true,
		},
		{
			name:    "api missing secret",
			raw:     map[string]interface{}{"type": fronteggPrehookTypeAPI, "url": "https://x"},
			wantErr: true,
		},
		{
			name:    "api complete",
			raw:     map[string]interface{}{"type": fronteggPrehookTypeAPI, "url": "https://x", "secret": "s"},
			wantErr: false,
		},
		{
			name:    "custom code missing code",
			raw:     map[string]interface{}{"type": fronteggPrehookTypeCustomCode},
			wantErr: true,
		},
		{
			name:    "custom code complete",
			raw:     map[string]interface{}{"type": fronteggPrehookTypeCustomCode, "code": "c"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := prehookResourceData(t, tt.raw)
			err := validateFronteggPrehookFields(d)
			if (err != nil) != tt.wantErr {
				t.Errorf("customizeDiff err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
