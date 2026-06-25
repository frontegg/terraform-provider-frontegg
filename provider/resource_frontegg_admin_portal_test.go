package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const testAccAdminPortalNoPalette = `
resource "frontegg_admin_portal" "test" {
  enable_account_settings    = true
  enable_api_tokens          = true
  enable_audit_logs          = true
  enable_personal_api_tokens = true
  enable_privacy             = true
  enable_profile             = true
  enable_roles               = true
  enable_security            = true
  enable_sso                 = true
  enable_subscriptions       = true
  enable_usage               = true
  enable_users               = true
  enable_webhooks            = true
  enable_groups              = true
  enable_provisioning        = true
}
`

func TestAccFronteggAdminPortal_omittedPalettePreservesTheme(t *testing.T) {
	var before map[string]interface{}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { before = adminPortalLoginBox(t) },
				Config:    testAccAdminPortalNoPalette,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_admin_portal.test", "enable_users", "true"),
					testAccCheckAdminPortalThemePreserved(t, &before),
				),
			},
			{
				Config:   testAccAdminPortalNoPalette,
				PlanOnly: true,
			},
		},
	})
}

func testAccCheckAdminPortalThemePreserved(t *testing.T, before *map[string]interface{}) resource.TestCheckFunc {
	return func(*terraform.State) error {
		after := adminPortalLoginBox(t)
		for key := range *before {
			if _, ok := after[key]; !ok {
				return fmt.Errorf("themeV2.loginBox.%s was dropped on apply; the provider must preserve unmanaged theme fields", key)
			}
		}
		palette, ok := after["palette"].(map[string]interface{})
		if !ok {
			return nil
		}
		for category, raw := range palette {
			colors, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if main, _ := colors["main"].(string); strings.TrimSpace(main) == "" {
				return fmt.Errorf("themeV2.loginBox.palette.%s.main is blank; an empty palette crashes the React SDK (FR-25477)", category)
			}
		}
		return nil
	}
}

func adminPortalLoginBox(t *testing.T) map[string]interface{} {
	t.Helper()
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	token := fronteggVendorToken(t, base)

	req, err := http.NewRequest(http.MethodGet, base+"/metadata?entityName=adminBox", nil)
	if err != nil {
		t.Fatalf("build admin portal request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get admin portal metadata: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Rows []struct {
			Configuration struct {
				ThemeV2 struct {
					LoginBox map[string]interface{} `json:"loginBox"`
				} `json:"themeV2"`
			} `json:"configuration"`
		} `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode admin portal metadata: %v", err)
	}
	if len(out.Rows) == 0 || out.Rows[0].Configuration.ThemeV2.LoginBox == nil {
		return map[string]interface{}{}
	}
	return out.Rows[0].Configuration.ThemeV2.LoginBox
}

func fronteggVendorToken(t *testing.T, base string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"clientId": os.Getenv("FRONTEGG_CLIENT_ID"),
		"secret":   os.Getenv("FRONTEGG_SECRET_KEY"),
	})
	if err != nil {
		t.Fatalf("marshal vendor auth: %v", err)
	}
	resp, err := http.Post(base+"/auth/vendor", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("frontegg vendor auth: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode vendor auth: %v", err)
	}
	return out.Token
}
