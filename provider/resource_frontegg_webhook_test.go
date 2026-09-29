package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type fakeWebhookHost struct {
	mu       sync.Mutex
	requests []string
	webhooks map[string]fronteggWebhook
	deleted  int
}

func (h *fakeWebhookHost) serve(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requests = append(h.requests, r.Method+" "+r.URL.Path)
	switch {
	case r.Method == http.MethodPost && r.URL.Path == fronteggWebhookPath+"/custom":
		var in fronteggWebhook
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.ID = "hook-1"
		in.Type = "CUSTOM"
		h.webhooks[in.ID] = in
		_ = json.NewEncoder(w).Encode(in)
	case r.Method == http.MethodGet && r.URL.Path == fronteggWebhookPath:
		out := []fronteggWebhook{}
		for _, hook := range h.webhooks {
			out = append(out, hook)
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, fronteggWebhookPath+"/"):
		id := strings.TrimPrefix(r.URL.Path, fronteggWebhookPath+"/")
		var in fronteggWebhook
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.ID = id
		in.Type = "CUSTOM"
		h.webhooks[id] = in
		_ = json.NewEncoder(w).Encode(in)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, fronteggWebhookPath+"/"):
		id := strings.TrimPrefix(r.URL.Path, fronteggWebhookPath+"/")
		if _, ok := h.webhooks[id]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(h.webhooks, id)
		h.deleted++
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func webhookTestHolder(t *testing.T) (*restclient.ClientHolder, *fakeWebhookHost, *fakeWebhookHost) {
	t.Helper()
	portal := &fakeWebhookHost{webhooks: map[string]fronteggWebhook{}}
	api := &fakeWebhookHost{webhooks: map[string]fronteggWebhook{}}
	portalSrv := httptest.NewServer(http.HandlerFunc(portal.serve))
	apiSrv := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(portalSrv.Close)
	t.Cleanup(apiSrv.Close)
	holder := &restclient.ClientHolder{
		ApiClient:    restclient.MakeRestClient(apiSrv.URL, "env-1", ""),
		PortalClient: restclient.MakeRestClient(portalSrv.URL, "env-1", ""),
	}
	holder.ApiClient.Authenticate("test-token")
	holder.PortalClient.Authenticate("test-token")
	return holder, portal, api
}

func webhookResourceData(t *testing.T, description string) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggWebhook().Schema, map[string]interface{}{
		"enabled":     true,
		"name":        "probe",
		"description": description,
		"url":         "https://example.com/hook",
		"secret":      "probe-secret",
		"events":      []interface{}{"frontegg.user.created"},
	})
}

func TestWebhookCRUDUsesPortalHost(t *testing.T) {
	holder, portal, api := webhookTestHolder(t)
	ctx := context.Background()
	d := webhookResourceData(t, "created")

	if diags := resourceFronteggWebhookCreate(ctx, d, holder); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if diags := resourceFronteggWebhookRead(ctx, d, holder); diags.HasError() {
		t.Fatalf("read: %v", diags)
	}
	if err := d.Set("description", "updated"); err != nil {
		t.Fatal(err)
	}
	if diags := resourceFronteggWebhookUpdate(ctx, d, holder); diags.HasError() {
		t.Fatalf("update: %v", diags)
	}
	if diags := resourceFronteggWebhookDelete(ctx, d, holder); diags.HasError() {
		t.Fatalf("delete: %v", diags)
	}

	want := []string{"POST /webhook/custom", "GET /webhook", "PATCH /webhook/hook-1", "DELETE /webhook/hook-1"}
	if got := strings.Join(portal.requests, ", "); got != strings.Join(want, ", ") {
		t.Errorf("portal host requests = %q, want %q", got, strings.Join(want, ", "))
	}
	if len(api.requests) != 0 {
		t.Errorf("API host requests = %q, want none: webhooks the portal shows only exist on the portal host", api.requests)
	}
}

func TestWebhookReadClearsIDWhenMissing(t *testing.T) {
	holder, _, _ := webhookTestHolder(t)
	d := webhookResourceData(t, "probe")
	d.SetId("deleted-elsewhere")

	if diags := resourceFronteggWebhookRead(context.Background(), d, holder); diags.HasError() {
		t.Fatalf("read: %v", diags)
	}
	if d.Id() != "" {
		t.Errorf("id = %q, want empty so a webhook deleted outside Terraform shows as drift", d.Id())
	}
}

func TestWebhookDeleteToleratesAlreadyDeleted(t *testing.T) {
	holder, _, _ := webhookTestHolder(t)
	d := webhookResourceData(t, "probe")
	d.SetId("already-gone")

	if diags := resourceFronteggWebhookDelete(context.Background(), d, holder); diags.HasError() {
		t.Errorf("delete of a missing webhook failed: %v", diags)
	}
}

func testAccPreCheckPortalCredentials(t *testing.T) {
	testAccPreCheck(t)
	if os.Getenv("FRONTEGG_ENVIRONMENT_ID") == "" {
		t.Skip("FRONTEGG_ENVIRONMENT_ID and an admin-portal API key must be set for webhook acceptance tests")
	}
}

func testAccWebhookConfig(name, description string, enabled bool, events string) string {
	return fmt.Sprintf(`
provider "frontegg" {
  environment_id = %q
}

resource "frontegg_webhook" "test" {
  enabled     = %t
  name        = %q
  description = %q
  url         = "https://example.com/webhook"
  secret      = "tf-acc-webhook-secret"
  events      = [%s]
}
`, os.Getenv("FRONTEGG_ENVIRONMENT_ID"), enabled, name, description, events)
}

func TestAccFronteggWebhook_lifecycle(t *testing.T) {
	name := "tf-acc webhook " + acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckPortalCredentials(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookConfig(name, "created", false, `"frontegg.user.created"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_webhook.test", "description", "created"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "type", "CUSTOM"),
					testAccCheckWebhookListedInPortal("frontegg_webhook.test"),
				),
			},
			{
				Config:   testAccWebhookConfig(name, "created", false, `"frontegg.user.created"`),
				PlanOnly: true,
			},
			{
				Config: testAccWebhookConfig(name, "updated", true, `"frontegg.user.created", "frontegg.user.deleted"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_webhook.test", "description", "updated"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "enabled", "true"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "events.#", "2"),
					testAccCheckWebhookListedInPortal("frontegg_webhook.test"),
				),
			},
			{
				Config:                  testAccWebhookConfig(name, "updated", true, `"frontegg.user.created", "frontegg.user.deleted"`),
				ResourceName:            "frontegg_webhook.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func TestAccFronteggWebhook_importPortalCreated(t *testing.T) {
	name := "tf-acc portal webhook " + acctest.RandString(8)
	var id string
	config := testAccWebhookConfig(name, "created in the portal", false, `"frontegg.user.created"`)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckPortalCredentials(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					created, err := testAccCreatePortalWebhook(name)
					if err != nil {
						t.Fatalf("create webhook the way the portal does: %v", err)
					}
					id = created
				},
				Config:             config,
				ResourceName:       "frontegg_webhook.test",
				ImportState:        true,
				ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
				ImportStatePersist: true,
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func testAccPortalWebhookRequest(method, path string, body interface{}) (*http.Response, error) {
	apiBase := os.Getenv("FRONTEGG_API_BASE_URL")
	if apiBase == "" {
		apiBase = "https://api.frontegg.com"
	}
	portalBase := os.Getenv("FRONTEGG_PORTAL_BASE_URL")
	if portalBase == "" {
		portalBase = "https://frontegg-prod.frontegg.com"
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, portalBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+fronteggVendorTokenNoT(apiBase))
	req.Header.Set("frontegg-environment-id", os.Getenv("FRONTEGG_ENVIRONMENT_ID"))
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func testAccCreatePortalWebhook(name string) (string, error) {
	resp, err := testAccPortalWebhookRequest(http.MethodPost, fronteggWebhookPath+"/custom", fronteggWebhook{
		DisplayName: name,
		Description: "created in the portal",
		URL:         "https://example.com/webhook",
		Secret:      "tf-acc-webhook-secret",
		EventKeys:   []string{"frontegg.user.created"},
		IsActive:    false,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, raw)
	}
	var out fronteggWebhook
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func testAccPortalWebhookIDs() (map[string]bool, error) {
	resp, err := testAccPortalWebhookRequest(http.MethodGet, fronteggWebhookPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("listing portal webhooks: status %d: %s", resp.StatusCode, raw)
	}
	var out []fronteggWebhook
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, w := range out {
		ids[w.ID] = true
	}
	return ids, nil
}

func testAccCheckWebhookListedInPortal(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not found in state", name)
		}
		ids, err := testAccPortalWebhookIDs()
		if err != nil {
			return err
		}
		if !ids[rs.Primary.ID] {
			return fmt.Errorf("webhook %s is not in the list the portal shows", rs.Primary.ID)
		}
		return nil
	}
}

func testAccCheckWebhookDestroy(s *terraform.State) error {
	ids, err := testAccPortalWebhookIDs()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "frontegg_webhook" && ids[rs.Primary.ID] {
			return fmt.Errorf("webhook %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}
