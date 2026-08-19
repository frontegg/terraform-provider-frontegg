package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func portalUserResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggPortalUser().Schema, raw)
}

func TestPortalUserImportSplitsCompoundID(t *testing.T) {
	d := portalUserResourceData(t, map[string]interface{}{})
	d.SetId("tenant-1:user-2")

	out, err := resourceFronteggPortalUserImport(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("import returned %d resources, want 1", len(out))
	}
	if got := out[0].Id(); got != "user-2" {
		t.Errorf("id = %q, want %q", got, "user-2")
	}
	if got := out[0].Get("tenant_id").(string); got != "tenant-1" {
		t.Errorf("tenant_id = %q, want %q", got, "tenant-1")
	}
}

func TestPortalUserImportRejectsBadFormat(t *testing.T) {
	for _, id := range []string{"user-only", "tenant-1:", ":user-2", "", ":"} {
		d := portalUserResourceData(t, map[string]interface{}{})
		d.SetId(id)
		if _, err := resourceFronteggPortalUserImport(context.Background(), d, nil); err == nil {
			t.Errorf("import(%q) = nil error, want error", id)
		}
	}
}

func TestPortalUserSendsTenantHeader(t *testing.T) {
	d := portalUserResourceData(t, map[string]interface{}{
		"tenant_id": "tenant-1",
		"email":     "user@example.com",
		"role_ids":  []interface{}{"role-1"},
	})

	if got := resourceFronteggPortalUserHeaders(d).Get("frontegg-tenant-id"); got != "tenant-1" {
		t.Errorf("frontegg-tenant-id = %q, want %q", got, "tenant-1")
	}
}

func TestPortalUserTenantIDForcesNew(t *testing.T) {
	tenantID := resourceFronteggPortalUser().Schema["tenant_id"]
	if tenantID == nil {
		t.Fatal("tenant_id is missing from the schema")
	}
	if !tenantID.Required {
		t.Error("tenant_id must be required")
	}
	if !tenantID.ForceNew {
		t.Error("tenant_id must force replacement; users cannot move between tenants")
	}
}

const testAccPortalUserCreate = `
resource "frontegg_tenant" "test" {
  key  = "tf-acc-portal-user-tenant"
  name = "tf-acc portal user tenant"
}

resource "frontegg_application_tenant_assignment" "test" {
  app_id    = "%s"
  tenant_id = frontegg_tenant.test.id
}

resource "frontegg_role" "test" {
  key            = "tf-acc-portal-user-role"
  name           = "tf-acc portal user role"
  description    = "role for the portal user acceptance test"
  level          = 1
  default        = false
  permission_ids = []
}

resource "frontegg_role" "second" {
  key            = "tf-acc-portal-user-role-2"
  name           = "tf-acc portal user role 2"
  description    = "second role for the portal user acceptance test"
  level          = 2
  default        = false
  permission_ids = []
}

resource "frontegg_portal_user" "test" {
  tenant_id  = frontegg_tenant.test.id
  email      = "%s"
  role_ids   = [%s]
  depends_on = [frontegg_application_tenant_assignment.test]
}
`

func testAccPortalUserConfig(email, roles string) string {
	return fmt.Sprintf(testAccPortalUserCreate, os.Getenv("FRONTEGG_APPLICATION_ID"), email, roles)
}

const (
	testAccPortalUserEmail        = "tf-acc-portal-user@example.com"
	testAccPortalUserEmailUpdated = "tf-acc-portal-user-updated@example.com"
	testAccPortalUserOneRole      = "frontegg_role.test.id"
	testAccPortalUserTwoRoles     = "frontegg_role.test.id, frontegg_role.second.id"
)

// Creating a user requires an application unless the environment has a
// default one, and the identity API rejects the request outright otherwise.
func testAccPreCheckApplication(t *testing.T) {
	testAccPreCheck(t)
	if os.Getenv("FRONTEGG_APPLICATION_ID") == "" {
		t.Skip("FRONTEGG_APPLICATION_ID must be set for user acceptance tests")
	}
}

func TestAccFronteggPortalUser_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckApplication(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPortalUserConfig(testAccPortalUserEmail, testAccPortalUserOneRole),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_portal_user.test", "email", testAccPortalUserEmail),
					resource.TestCheckResourceAttrPair("frontegg_portal_user.test", "tenant_id", "frontegg_tenant.test", "id"),
					resource.TestCheckResourceAttr("frontegg_portal_user.test", "role_ids.#", "1"),
				),
			},
			{
				Config:   testAccPortalUserConfig(testAccPortalUserEmail, testAccPortalUserOneRole),
				PlanOnly: true,
			},
			{
				Config: testAccPortalUserConfig(testAccPortalUserEmailUpdated, testAccPortalUserTwoRoles),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_portal_user.test", "email", testAccPortalUserEmailUpdated),
					resource.TestCheckResourceAttr("frontegg_portal_user.test", "role_ids.#", "2"),
				),
			},
			{
				Config: testAccPortalUserConfig(testAccPortalUserEmailUpdated, testAccPortalUserOneRole),
				Check: resource.TestCheckResourceAttr(
					"frontegg_portal_user.test", "role_ids.#", "1"),
			},
			{
				ResourceName:      "frontegg_portal_user.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["frontegg_portal_user.test"]
					if !ok {
						return "", fmt.Errorf("frontegg_portal_user.test not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["tenant_id"], rs.Primary.ID), nil
				},
			},
		},
	})
}
