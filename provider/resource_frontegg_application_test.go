package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccApplicationConfig(suffix, description string) string {
	return fmt.Sprintf(`
resource "frontegg_application" "test" {
  name        = "tf-acc app %[1]s"
  app_url     = "https://tf-acc-%[1]s.example.com"
  login_url   = "https://tf-acc-%[1]s.example.com/oauth"
  access_type = "MANAGED_ACCESS"
  description = %[2]q
}
`, suffix, description)
}

func TestAccFronteggApplication_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDestroyed("frontegg_application"),
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationConfig(suffix, "first description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_application.test", "access_type", "MANAGED_ACCESS"),
					resource.TestCheckResourceAttr("frontegg_application.test", "description", "first description"),
				),
			},
			{
				Config:   testAccApplicationConfig(suffix, "first description"),
				PlanOnly: true,
			},
			{
				Config: testAccApplicationConfig(suffix, "second description"),
				Check:  resource.TestCheckResourceAttr("frontegg_application.test", "description", "second description"),
			},
			{
				Config:   testAccApplicationConfig(suffix, "second description"),
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_application.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
