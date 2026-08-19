package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// The API stores every origin with a trailing slash, so a configuration that
// omits it must still match what comes back.
func TestContainsAllowedOriginIgnoresTrailingSlash(t *testing.T) {
	stored := &fronteggAllowedOrigins{AllowedOrigins: []string{
		"http://localhost:3000/",
		"https://app.example.com/",
	}}

	for _, origin := range []string{
		"https://app.example.com",
		"https://app.example.com/",
		"http://localhost:3000",
	} {
		if !containsAllowedOrigin(stored, origin) {
			t.Errorf("containsAllowedOrigin(%q) = false, want true", origin)
		}
	}

	for _, origin := range []string{
		"https://other.example.com",
		"https://app.example.com/path",
		"",
	} {
		if containsAllowedOrigin(stored, origin) {
			t.Errorf("containsAllowedOrigin(%q) = true, want false", origin)
		}
	}
}

const testAccAllowedOrigin = `
resource "frontegg_allowed_origin" "test" {
  allowed_origin = "https://tf-acc-origin.example.com"
}
`

func TestAccFronteggAllowedOrigin_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAllowedOrigin,
				Check: resource.TestCheckResourceAttr(
					"frontegg_allowed_origin.test", "allowed_origin", "https://tf-acc-origin.example.com"),
			},
			{
				Config:   testAccAllowedOrigin,
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_allowed_origin.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "https://tf-acc-origin.example.com",
			},
		},
	})
}
