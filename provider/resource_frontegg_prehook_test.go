package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

const testAccPrehookCustomCodeCreate = `
resource "frontegg_prehook" "cc" {
  enabled     = true
  name        = "tf-acc custom code"
  description = "created"
  type        = "CUSTOM_CODE"
  events      = ["USER_INVITE"]
  fail_method = "OPEN"
  code        = "async function onEvent(e){return {verdict:\"allow\"}}\nexports.onEvent = onEvent;"
}
`

const testAccPrehookCustomCodeUpdate = `
resource "frontegg_prehook" "cc" {
  enabled     = true
  name        = "tf-acc custom code"
  description = "updated"
  type        = "CUSTOM_CODE"
  timeout     = 5
  events      = ["USER_INVITE"]
  fail_method = "CLOSE"
  code        = "async function onEvent(e){return {verdict:\"block\"}}\nexports.onEvent = onEvent;"
}
`

func TestAccFronteggPrehook_customCodeLifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPrehookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPrehookCustomCodeCreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "type", "CUSTOM_CODE"),
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "description", "created"),
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "runtime", "NODE_20"),
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "timeout", "10"),
					resource.TestCheckResourceAttrSet("frontegg_prehook.cc", "executor_identifier"),
				),
			},
			{
				Config:   testAccPrehookCustomCodeCreate,
				PlanOnly: true,
			},
			{
				Config: testAccPrehookCustomCodeUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "description", "updated"),
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "fail_method", "CLOSE"),
					resource.TestCheckResourceAttr("frontegg_prehook.cc", "timeout", "5"),
				),
			},
			{
				ResourceName:      "frontegg_prehook.cc",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// code content is fetched from the executor and may be
					// normalized by the backend; not part of the list payload.
					"code",
				},
			},
		},
	})
}

const testAccPrehookAPICreate = `
resource "frontegg_prehook" "api" {
  enabled     = true
  name        = "tf-acc api"
  description = "api hook"
  type        = "API"
  url         = "https://example.com/prehook"
  secret      = "tf-acc-secret"
  events      = ["USER_INVITE"]
  fail_method = "CLOSE"
}
`

func TestAccFronteggPrehook_apiLifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPrehookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPrehookAPICreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_prehook.api", "type", "API"),
					resource.TestCheckResourceAttr("frontegg_prehook.api", "url", "https://example.com/prehook"),
				),
			},
			{
				Config:   testAccPrehookAPICreate,
				PlanOnly: true,
			},
		},
	})
}

const testAccPrehookDuplicateEvent = `
resource "frontegg_prehook" "a" {
  enabled     = true
  name        = "tf-acc dup a"
  description = "first"
  type        = "API"
  url         = "https://example.com/a"
  secret      = "s"
  events      = ["USER_INVITE"]
  fail_method = "OPEN"
}

resource "frontegg_prehook" "b" {
  enabled     = true
  name        = "tf-acc dup b"
  description = "second"
  type        = "API"
  url         = "https://example.com/b"
  secret      = "s"
  events      = ["USER_INVITE"]
  fail_method = "OPEN"
  depends_on  = [frontegg_prehook.a]
}
`

func TestAccFronteggPrehook_duplicateEventRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPrehookDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccPrehookDuplicateEvent,
				ExpectError: regexp.MustCompile(`(?i)already exists? for`),
			},
		},
	})
}

func testAccCheckPrehookDestroy(s *terraform.State) error {
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	var ids []string
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "frontegg_prehook" {
			ids = append(ids, rs.Primary.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	token := fronteggVendorTokenNoT(base)
	req, err := http.NewRequest(http.MethodGet, base+"/prehooks/resources/configurations/v1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	for _, p := range out {
		for _, id := range ids {
			if p.ID == id {
				return fmt.Errorf("prehook %s still exists after destroy", id)
			}
		}
	}
	return nil
}

func fronteggVendorTokenNoT(base string) string {
	body, _ := json.Marshal(map[string]string{
		"clientId": os.Getenv("FRONTEGG_CLIENT_ID"),
		"secret":   os.Getenv("FRONTEGG_SECRET_KEY"),
	})
	resp, err := http.Post(base+"/auth/vendor", "application/json", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Token
}
