package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestWriteOnlySecretsArePaired(t *testing.T) {
	for resourceName, resource := range New("0.0.0")().ResourcesMap {
		for key, attr := range resource.Schema {
			if !attr.WriteOnly {
				continue
			}
			secret := strings.TrimSuffix(key, writeOnlySuffix)
			if !attr.Sensitive {
				t.Errorf("%s.%s must be sensitive", resourceName, key)
			}
			if resource.Schema[secret+writeOnlyVersionSuffix] == nil {
				t.Errorf("%s.%s%s is missing", resourceName, secret, writeOnlyVersionSuffix)
			}
			if plain := resource.Schema[secret]; plain == nil || plain.Required {
				t.Errorf("%s.%s must exist and be optional so %s can replace it", resourceName, secret, key)
			}
		}
	}
}

func skipBelowWriteOnlyTerraform(t *testing.T) {
	t.Helper()
	pinned := os.Getenv("TF_ACC_TERRAFORM_VERSION")
	if pinned != "" && version.Must(version.NewVersion(pinned)).LessThan(version.Must(version.NewVersion("1.11.0"))) {
		t.Skipf("write-only arguments need Terraform 1.11 or later, TF_ACC_TERRAFORM_VERSION is %s", pinned)
	}
}

func fakeWebhookAPI(t *testing.T) *fakeWebhookHost {
	t.Helper()
	host := &fakeWebhookHost{webhooks: map[string]fronteggWebhook{}}
	portal := httptest.NewServer(http.HandlerFunc(host.serve))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
	}))
	t.Cleanup(portal.Close)
	t.Cleanup(api.Close)
	t.Setenv("FRONTEGG_API_BASE_URL", api.URL)
	t.Setenv("FRONTEGG_PORTAL_BASE_URL", portal.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")
	return host
}

func webhookSecretConfig(secretArguments string) string {
	return fmt.Sprintf(`
variable "webhook_secret" {
  type      = string
  ephemeral = true
  default   = "ephemeral-secret"
}

resource "frontegg_webhook" "test" {
  enabled     = true
  description = "write-only secret"
  url         = "https://example.com/hook"
  events      = ["frontegg.user.created"]
%s
}
`, secretArguments)
}

func checkWebhookSecretSent(host *fakeWebhookHost, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		host.mu.Lock()
		defer host.mu.Unlock()
		if got := host.webhooks["hook-1"].Secret; got != want {
			return fmt.Errorf("API received secret %q, want %q", got, want)
		}
		return nil
	}
}

func checkSecretOutOfState(secrets ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			for key, value := range rs.Primary.Attributes {
				for _, secret := range secrets {
					if value == secret {
						return fmt.Errorf("state attribute %s holds the write-only secret", key)
					}
				}
			}
		}
		return nil
	}
}

func TestWebhookWriteOnlySecret(t *testing.T) {
	skipBelowWriteOnlyTerraform(t)
	host := fakeWebhookAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: webhookSecretConfig(`  secret = "plain-secret"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkWebhookSecretSent(host, "plain-secret"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "secret", "plain-secret"),
				),
			},
			{
				Config: webhookSecretConfig(`
  secret_wo         = var.webhook_secret
  secret_wo_version = 1`),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkWebhookSecretSent(host, "ephemeral-secret"),
					checkSecretOutOfState("ephemeral-secret"),
					resource.TestCheckResourceAttr("frontegg_webhook.test", "secret", ""),
				),
			},
			{
				Config: webhookSecretConfig(`
  secret_wo         = var.webhook_secret
  secret_wo_version = 1`),
				PlanOnly: true,
			},
			{
				Config: webhookSecretConfig(`
  secret_wo         = "rotated-secret"
  secret_wo_version = 1`),
				PlanOnly: true,
			},
			{
				Config: webhookSecretConfig(`
  secret_wo         = "rotated-secret"
  secret_wo_version = 2`),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkWebhookSecretSent(host, "rotated-secret"),
					checkSecretOutOfState("ephemeral-secret", "rotated-secret"),
				),
			},
		},
	})
}

func TestWebhookImportKeepsSecretOutOfState(t *testing.T) {
	skipBelowWriteOnlyTerraform(t)
	fakeWebhookAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: webhookSecretConfig(`
  secret_wo         = var.webhook_secret
  secret_wo_version = 1`),
			},
			{
				ResourceName: "frontegg_webhook.test",
				ImportState:  true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					for _, state := range states {
						for key, value := range state.Attributes {
							if value == "ephemeral-secret" {
								return fmt.Errorf("imported state attribute %s holds the secret", key)
							}
						}
					}
					return nil
				},
			},
		},
	})
}

func TestWebhookWriteOnlySecretValidation(t *testing.T) {
	skipBelowWriteOnlyTerraform(t)
	fakeWebhookAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      webhookSecretConfig(`  secret_wo = var.webhook_secret`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("all of `secret_wo,secret_wo_version` must be specified"),
			},
			{
				Config: webhookSecretConfig(`
  secret            = "plain-secret"
  secret_wo         = var.webhook_secret
  secret_wo_version = 1`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("only one of `secret,secret_wo` can be specified"),
			},
			{
				Config:      webhookSecretConfig(``),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("one of `secret,secret_wo` must be specified"),
			},
		},
	})
}
