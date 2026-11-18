package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccFeatureConfig(suffix, description string) string {
	return fmt.Sprintf(`
resource "frontegg_feature" "test" {
  name        = "tf-acc feature %[1]s"
  key         = "tf-acc-feature-%[1]s"
  description = %[2]q
}
`, suffix, description)
}

func TestAccFronteggFeature_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDestroyed("frontegg_feature"),
		Steps: []resource.TestStep{
			{
				Config: testAccFeatureConfig(suffix, "first description"),
				Check:  resource.TestCheckResourceAttr("frontegg_feature.test", "key", "tf-acc-feature-"+suffix),
			},
			{
				Config:   testAccFeatureConfig(suffix, "first description"),
				PlanOnly: true,
			},
			{
				Config: testAccFeatureConfig(suffix, "second description"),
				Check:  resource.TestCheckResourceAttr("frontegg_feature.test", "description", "second description"),
			},
			{
				Config:   testAccFeatureConfig(suffix, "second description"),
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_feature.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
