package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceFronteggSessionManagementPolicySerialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggSessionManagementPolicy().Schema, map[string]interface{}{
		"idle_session_timeout_enabled":    true,
		"idle_session_timeout":            3600,
		"force_relogin_enabled":           false,
		"force_relogin_timeout":           604800,
		"max_concurrent_sessions_enabled": true,
		"max_concurrent_sessions":         5,
	})

	got := resourceFronteggSessionManagementPolicySerialize(d)
	if !got.SessionIdleTimeoutConfiguration.IsActive || got.SessionIdleTimeoutConfiguration.Timeout != 3600 {
		t.Errorf("idle = %+v", got.SessionIdleTimeoutConfiguration)
	}
	if got.SessionTimeoutConfiguration.IsActive || got.SessionTimeoutConfiguration.Timeout != 604800 {
		t.Errorf("force relogin = %+v", got.SessionTimeoutConfiguration)
	}
	if !got.SessionConcurrentConfiguration.IsActive || got.SessionConcurrentConfiguration.MaxSessions != 5 {
		t.Errorf("concurrent = %+v", got.SessionConcurrentConfiguration)
	}
}

func TestResourceFronteggSessionManagementPolicyDeserialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggSessionManagementPolicy().Schema, map[string]interface{}{})
	out := fronteggSessionManagementPolicy{
		SessionIdleTimeoutConfiguration: fronteggSessionTimeoutConfig{IsActive: true, Timeout: 7200},
		SessionTimeoutConfiguration:     fronteggSessionTimeoutConfig{IsActive: true, Timeout: 86400},
		SessionConcurrentConfiguration:  fronteggSessionConcurrentConfig{IsActive: true, MaxSessions: 3},
	}
	if err := resourceFronteggSessionManagementPolicyDeserialize(d, out); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if d.Id() != sessionManagementPolicyID {
		t.Errorf("id = %q, want %q", d.Id(), sessionManagementPolicyID)
	}
	if d.Get("idle_session_timeout_enabled") != true || d.Get("idle_session_timeout") != 7200 {
		t.Errorf("idle = %v/%v", d.Get("idle_session_timeout_enabled"), d.Get("idle_session_timeout"))
	}
	if d.Get("force_relogin_enabled") != true || d.Get("force_relogin_timeout") != 86400 {
		t.Errorf("force relogin = %v/%v", d.Get("force_relogin_enabled"), d.Get("force_relogin_timeout"))
	}
	if d.Get("max_concurrent_sessions_enabled") != true || d.Get("max_concurrent_sessions") != 3 {
		t.Errorf("concurrent = %v/%v", d.Get("max_concurrent_sessions_enabled"), d.Get("max_concurrent_sessions"))
	}
}

// TestResourceFronteggSessionManagementPolicyDeserializeSkipsInactiveValues
// verifies that the value of a disabled feature is not overwritten from the API
// response, which would otherwise produce perpetual plan diffs.
func TestResourceFronteggSessionManagementPolicyDeserializeSkipsInactiveValues(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggSessionManagementPolicy().Schema, map[string]interface{}{
		"idle_session_timeout":    3600,
		"max_concurrent_sessions": 7,
	})
	out := fronteggSessionManagementPolicy{
		SessionIdleTimeoutConfiguration: fronteggSessionTimeoutConfig{IsActive: false, Timeout: 9999},
		SessionConcurrentConfiguration:  fronteggSessionConcurrentConfig{IsActive: false, MaxSessions: 99},
	}
	if err := resourceFronteggSessionManagementPolicyDeserialize(d, out); err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if d.Get("idle_session_timeout_enabled") != false {
		t.Error("idle_session_timeout_enabled should be false")
	}
	if d.Get("idle_session_timeout") != 3600 {
		t.Errorf("idle_session_timeout should remain 3600 (not synced while inactive), got %v", d.Get("idle_session_timeout"))
	}
	if d.Get("max_concurrent_sessions") != 7 {
		t.Errorf("max_concurrent_sessions should remain 7 (not synced while inactive), got %v", d.Get("max_concurrent_sessions"))
	}
}

// TestFronteggSessionManagementPolicyWireFormat asserts the camelCase JSON keys
// and that all three configuration objects are always present in the payload.
func TestFronteggSessionManagementPolicyWireFormat(t *testing.T) {
	b, err := json.Marshal(fronteggSessionManagementPolicy{
		SessionIdleTimeoutConfiguration: fronteggSessionTimeoutConfig{IsActive: true, Timeout: 3600},
		SessionConcurrentConfiguration:  fronteggSessionConcurrentConfig{IsActive: true, MaxSessions: 5},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	idle, ok := m["sessionIdleTimeoutConfiguration"].(map[string]interface{})
	if !ok || idle["isActive"] != true || idle["timeout"] != float64(3600) {
		t.Errorf("sessionIdleTimeoutConfiguration wire format wrong: %s", b)
	}
	// All three objects must always be present so the policy is fully declared.
	if _, ok := m["sessionTimeoutConfiguration"]; !ok {
		t.Errorf("sessionTimeoutConfiguration must always be present: %s", b)
	}
	conc, ok := m["sessionConcurrentConfiguration"].(map[string]interface{})
	if !ok || conc["isActive"] != true || conc["maxSessions"] != float64(5) {
		t.Errorf("sessionConcurrentConfiguration wire format wrong: %s", b)
	}
}
