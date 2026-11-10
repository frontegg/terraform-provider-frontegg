package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccPlanConfig(suffix, description string) string {
	return fmt.Sprintf(`
resource "frontegg_feature" "plan" {
  name = "tf-acc plan feature %[1]s"
  key  = "tf-acc-plan-feature-%[1]s"
}

resource "frontegg_plan" "test" {
  name        = "tf-acc plan %[1]s"
  description = %[2]q
}

resource "frontegg_plan_feature" "test" {
  plan_id     = frontegg_plan.test.id
  feature_ids = [frontegg_feature.plan.id]
}
`, suffix, description)
}

func TestAccFronteggPlan_lifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAllDestroyed("frontegg_plan", "frontegg_feature"),
		Steps: []resource.TestStep{
			{
				Config: testAccPlanConfig(suffix, "first description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_plan.test", "name", "tf-acc plan "+suffix),
					resource.TestCheckResourceAttr("frontegg_plan_feature.test", "feature_ids.#", "1"),
					resource.TestCheckResourceAttrPair("frontegg_plan_feature.test", "feature_ids.0", "frontegg_feature.plan", "id"),
				),
			},
			{
				Config:   testAccPlanConfig(suffix, "first description"),
				PlanOnly: true,
			},
			{
				Config: testAccPlanConfig(suffix, "second description"),
				Check:  resource.TestCheckResourceAttr("frontegg_plan.test", "description", "second description"),
			},
			{
				Config:   testAccPlanConfig(suffix, "second description"),
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_plan.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "frontegg_plan_feature.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
