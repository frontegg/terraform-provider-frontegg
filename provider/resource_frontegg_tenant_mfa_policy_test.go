package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccTenantMFAPolicyConfig(suffix, enforce string, rememberDevice bool) string {
	return fmt.Sprintf(`
resource "frontegg_tenant" "mfa" {
  key  = "tf-acc-mfa-tenant-%[1]s"
  name = "tf-acc mfa tenant %[1]s"
}

resource "frontegg_tenant_mfa_policy" "test" {
  tenant_id                = frontegg_tenant.mfa.id
  enforce_mfa_type         = %[2]q
  allow_remember_my_device = %[3]t
  mfa_device_expiration    = 1209600
}
`, suffix, enforce, rememberDevice)
}

func TestAccFronteggTenantMFAPolicy_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantMFAPolicyConfig(suffix, "on", false),
				Check:  resource.TestCheckResourceAttr("frontegg_tenant_mfa_policy.test", "enforce_mfa_type", "on"),
			},
			{
				Config:   testAccTenantMFAPolicyConfig(suffix, "on", false),
				PlanOnly: true,
			},
			{
				Config: testAccTenantMFAPolicyConfig(suffix, "unless-saml", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_tenant_mfa_policy.test", "enforce_mfa_type", "unless-saml"),
					resource.TestCheckResourceAttr("frontegg_tenant_mfa_policy.test", "allow_remember_my_device", "true"),
				),
			},
			{
				Config:   testAccTenantMFAPolicyConfig(suffix, "unless-saml", true),
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_tenant_mfa_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
