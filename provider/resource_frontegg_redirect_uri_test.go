package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccRedirectURIConfig(path string) string {
	return fmt.Sprintf(`
resource "frontegg_redirect_uri" "test" {
  redirect_uri = "https://tf-acc-redirect.example.com/%s"
}
`, path)
}

func TestAccFronteggRedirectURI_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDestroyed("frontegg_redirect_uri"),
		Steps: []resource.TestStep{
			{
				Config: testAccRedirectURIConfig(suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_redirect_uri.test", "redirect_uri", "https://tf-acc-redirect.example.com/"+suffix),
					resource.TestCheckResourceAttrSet("frontegg_redirect_uri.test", "key"),
				),
			},
			{
				Config:   testAccRedirectURIConfig(suffix),
				PlanOnly: true,
			},
			{
				Config: testAccRedirectURIConfig(suffix + "-updated"),
				Check:  resource.TestCheckResourceAttr("frontegg_redirect_uri.test", "redirect_uri", "https://tf-acc-redirect.example.com/"+suffix+"-updated"),
			},
			{
				ResourceName:      "frontegg_redirect_uri.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
