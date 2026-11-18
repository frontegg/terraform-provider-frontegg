package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func testAccTenantAPITokenConfig(suffix, description string) string {
	return fmt.Sprintf(`
resource "frontegg_tenant" "token" {
  key  = "tf-acc-token-tenant-%[1]s"
  name = "tf-acc token tenant %[1]s"
}

resource "frontegg_role" "token" {
  key            = "tf-acc-token-role-%[1]s"
  name           = "tf-acc token role %[1]s"
  description    = "role for the tenant API token acceptance test"
  level          = 1
  default        = false
  permission_ids = []
}

resource "frontegg_tenant_api_token" "test" {
  tenant_id   = frontegg_tenant.token.id
  description = %[2]q
  role_ids    = [frontegg_role.token.id]
}
`, suffix, description)
}

func TestAccFronteggTenantAPIToken_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantAPITokenConfig(suffix, "first description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("frontegg_tenant_api_token.test", "client_id"),
					resource.TestCheckResourceAttrSet("frontegg_tenant_api_token.test", "secret"),
				),
			},
			{
				Config:   testAccTenantAPITokenConfig(suffix, "first description"),
				PlanOnly: true,
			},
			{
				Config: testAccTenantAPITokenConfig(suffix, "second description"),
				Check:  resource.TestCheckResourceAttr("frontegg_tenant_api_token.test", "description", "second description"),
			},
			{
				Config:   testAccTenantAPITokenConfig(suffix, "second description"),
				PlanOnly: true,
			},
			{
				ResourceName:            "frontegg_tenant_api_token.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccTenantScopedImportID("frontegg_tenant_api_token.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret", "metadata"},
			},
		},
	})
}

func testAccTenantScopedImportID(address string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		stateResource, ok := state.RootModule().Resources[address]
		if !ok {
			return "", fmt.Errorf("%s is not in state", address)
		}
		return stateResource.Primary.Attributes["tenant_id"] + ":" + stateResource.Primary.ID, nil
	}
}
