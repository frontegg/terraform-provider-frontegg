package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccSecretConfig(key, value string) string {
	return fmt.Sprintf(`
resource "frontegg_secret" "test" {
  key   = %q
  value = %q
}
`, key, value)
}

func TestAccFronteggSecret_lifecycle(t *testing.T) {
	key := "TF_ACC_" + strings.ToUpper(acctest.RandString(8))
	renamedKey := key + "_NEW"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDestroyed("frontegg_secret"),
		Steps: []resource.TestStep{
			{
				Config: testAccSecretConfig(key, "first-value"),
				Check:  resource.TestCheckResourceAttr("frontegg_secret.test", "key", key),
			},
			{
				Config:   testAccSecretConfig(key, "first-value"),
				PlanOnly: true,
			},
			{
				Config: testAccSecretConfig(key, "second-value"),
				Check:  resource.TestCheckResourceAttr("frontegg_secret.test", "value", "second-value"),
			},
			{
				Config:   testAccSecretConfig(key, "second-value"),
				PlanOnly: true,
			},
			{
				Config: testAccSecretConfig(renamedKey, "second-value"),
				Check:  resource.TestCheckResourceAttr("frontegg_secret.test", "key", renamedKey),
			},
			{
				Config:   testAccSecretConfig(renamedKey, "second-value"),
				PlanOnly: true,
			},
			{
				ResourceName:            "frontegg_secret.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}
