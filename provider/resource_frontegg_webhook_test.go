package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestVendorIDFromToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"type":"vendor","vendorId":"abc-123"}`))
	token := "header." + payload + ".signature"

	got, err := vendorIDFromToken(token)
	if err != nil {
		t.Fatalf("vendorIDFromToken: %v", err)
	}
	if got != "abc-123" {
		t.Errorf("vendorId = %q, want %q", got, "abc-123")
	}
}

func TestVendorIDFromTokenRejectsNonJWT(t *testing.T) {
	for _, token := range []string{"", "not-a-jwt", "only.two"} {
		if _, err := vendorIDFromToken(token); err == nil {
			t.Errorf("vendorIDFromToken(%q) = nil error, want error", token)
		}
	}
}

const testAccWebhookCreate = `
resource "frontegg_webhook" "test" {
  enabled     = false
  name        = "tf-acc webhook"
  description = "created"
  url         = "https://example.com/webhook"
  secret      = "tf-acc-webhook-secret"
  events      = ["frontegg.user.created"]
}
`

const testAccWebhookUpdate = `
resource "frontegg_webhook" "test" {
  enabled     = true
  name        = "tf-acc webhook"
  description = "updated"
  url         = "https://example.com/webhook-updated"
  secret      = "tf-acc-webhook-secret"
  events      = ["frontegg.user.created", "frontegg.user.deleted"]
}
`

func TestAccFronteggWebhook_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookCreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_webhook.test", "description", "created"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "enabled", "false"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "type", "CUSTOM"),
					resource.TestCheckResourceAttrSet("frontegg_webhook.test", "vendor_id"),
				),
			},
			{
				Config:   testAccWebhookCreate,
				PlanOnly: true,
			},
			{
				Config: testAccWebhookUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_webhook.test", "description", "updated"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "enabled", "true"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "events.#", "2"),
				),
			},
			{
				ResourceName:      "frontegg_webhook.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckWebhookDestroy(s *terraform.State) error {
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	var ids []string
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "frontegg_webhook" {
			ids = append(ids, rs.Primary.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	token := fronteggVendorTokenNoT(base)
	vendorID, err := vendorIDFromToken(token)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, base+"/webhook", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("frontegg-tenant-id", vendorID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out []struct {
		ID string `json:"_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	for _, w := range out {
		for _, id := range ids {
			if w.ID == id {
				return fmt.Errorf("webhook %s still exists after destroy", id)
			}
		}
	}
	return nil
}
