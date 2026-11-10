package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Reuses the resource's own Read, which clears the ID when the object is gone.
func testAccCheckDestroyed(resourceType string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		ctx := context.Background()
		provider := New("test")()
		if diags := provider.Configure(ctx, terraform.NewResourceConfigRaw(nil)); diags.HasError() {
			return fmt.Errorf("configure provider: %v", diags)
		}
		resourceSchema := provider.ResourcesMap[resourceType]
		for _, stateResource := range state.RootModule().Resources {
			if stateResource.Type != resourceType {
				continue
			}
			data := resourceSchema.Data(stateResource.Primary)
			diags := resourceSchema.ReadContext(ctx, data, provider.Meta())
			if diags.HasError() {
				if strings.Contains(diags[0].Summary, "404") {
					continue
				}
				return fmt.Errorf("read %s %s after destroy: %s", resourceType, stateResource.Primary.ID, diags[0].Summary)
			}
			if data.Id() != "" {
				return fmt.Errorf("%s %s still exists after destroy", resourceType, stateResource.Primary.ID)
			}
		}
		return nil
	}
}

func testAccCheckAllDestroyed(resourceTypes ...string) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{}
	for _, resourceType := range resourceTypes {
		checks = append(checks, testAccCheckDestroyed(resourceType))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}
