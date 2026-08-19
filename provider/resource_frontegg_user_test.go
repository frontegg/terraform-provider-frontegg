package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Verbatim errors observed from the identity API when creating a user in an
// environment whose applications include no default.
var (
	errNoApplication = errors.New(
		`restclient: request failed: POST https://api.frontegg.com/identity/resources/users/v2: ` +
			`403 Forbidden (trace 468ccbfcb42b35d6f7462a49e4be21af): ` +
			`{"errors":["Application ID is not specified"],"errorCode":"ER-00008"}`,
	)
	errTenantNotAssigned = errors.New(
		`restclient: request failed: POST https://api.frontegg.com/identity/resources/users/v2: ` +
			`404 Not Found (trace 0e1631177c43f600ed747b6392560cf6): ` +
			`{"errors":["This account/tenant is not assigned to the requested application"],"errorCode":"ER-01008"}`,
	)
)

func TestUserApplicationErrorNamesProviderSetting(t *testing.T) {
	got := fronteggUserApplicationError(errNoApplication)
	if got == nil {
		t.Fatal("got nil error, want a translated error")
	}
	for _, want := range []string{"application_id", "FRONTEGG_APPLICATION_ID"} {
		if !strings.Contains(got.Error(), want) {
			t.Errorf("error does not mention %q: %s", want, got)
		}
	}
	if !errors.Is(got, errNoApplication) {
		t.Error("translated error must wrap the original")
	}
}

func TestUserApplicationErrorNamesAssignmentResource(t *testing.T) {
	got := fronteggUserApplicationError(errTenantNotAssigned)
	if got == nil {
		t.Fatal("got nil error, want a translated error")
	}
	if !strings.Contains(got.Error(), "frontegg_application_tenant_assignment") {
		t.Errorf("error does not name the assignment resource: %s", got)
	}
	if !errors.Is(got, errTenantNotAssigned) {
		t.Error("translated error must wrap the original")
	}
}

func TestUserApplicationErrorPassesOthersThrough(t *testing.T) {
	other := errors.New(`restclient: request failed: 400 Bad Request: {"errors":["email must be an email"]}`)
	if got := fronteggUserApplicationError(other); got != other {
		t.Errorf("unrelated error was rewritten: %s", got)
	}
	if got := fronteggUserApplicationError(nil); got != nil {
		t.Errorf("nil error became %v", got)
	}
}

const testAccUserBase = `
resource "frontegg_tenant" "u" {
  key  = "tf-acc-user-tenant"
  name = "tf-acc user tenant"
}

resource "frontegg_application_tenant_assignment" "u" {
  app_id    = "%s"
  tenant_id = frontegg_tenant.u.id
}

resource "frontegg_role" "u1" {
  key            = "tf-acc-user-role-1"
  name           = "tf-acc user role 1"
  description    = "first role for the user acceptance test"
  level          = 1
  default        = false
  permission_ids = []
}

resource "frontegg_role" "u2" {
  key            = "tf-acc-user-role-2"
  name           = "tf-acc user role 2"
  description    = "second role for the user acceptance test"
  level          = 2
  default        = false
  permission_ids = []
}

resource "frontegg_user" "u" {
  tenant_id         = frontegg_tenant.u.id
  email             = "%s"
  role_ids          = [%s]
  superuser         = %t
  skip_invite_email = true
  depends_on        = [frontegg_application_tenant_assignment.u]
}
`

func testAccUserConfig(email, roles string, superuser bool) string {
	return fmt.Sprintf(testAccUserBase, os.Getenv("FRONTEGG_APPLICATION_ID"), email, roles, superuser)
}

const (
	testAccUserEmail        = "tf-acc-user@example.com"
	testAccUserEmailUpdated = "tf-acc-user-updated@example.com"
	testAccUserOneRole      = "frontegg_role.u1.id"
	testAccUserTwoRoles     = "frontegg_role.u1.id, frontegg_role.u2.id"
)

func TestAccFronteggUser_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckApplication(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig(testAccUserEmail, testAccUserOneRole, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_user.u", "email", testAccUserEmail),
					resource.TestCheckResourceAttrPair("frontegg_user.u", "tenant_id", "frontegg_tenant.u", "id"),
					resource.TestCheckResourceAttr("frontegg_user.u", "role_ids.#", "1"),
				),
			},
			{
				Config:   testAccUserConfig(testAccUserEmail, testAccUserOneRole, false),
				PlanOnly: true,
			},
			{
				Config: testAccUserConfig(testAccUserEmailUpdated, testAccUserTwoRoles, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_user.u", "email", testAccUserEmailUpdated),
					resource.TestCheckResourceAttr("frontegg_user.u", "role_ids.#", "2"),
				),
			},
			{
				Config: testAccUserConfig(testAccUserEmailUpdated, testAccUserOneRole, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_user.u", "role_ids.#", "1"),
					resource.TestCheckResourceAttr("frontegg_user.u", "superuser", "true"),
				),
			},
			{
				ResourceName:      "frontegg_user.u",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// Neither is part of the user payload; they only describe
					// how the resource was created.
					"skip_invite_email",
					"superuser",
				},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["frontegg_user.u"]
					if !ok {
						return "", fmt.Errorf("frontegg_user.u not found in state")
					}
					return fmt.Sprintf("%s:%s", rs.Primary.Attributes["tenant_id"], rs.Primary.ID), nil
				},
			},
		},
	})
}

func TestUserImportSplitsCompoundID(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggUser().Schema, map[string]interface{}{})
	d.SetId("tenant-1:user-2")

	out, err := resourceFronteggUserImport(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := out[0].Id(); got != "user-2" {
		t.Errorf("id = %q, want %q", got, "user-2")
	}
	if got := out[0].Get("tenant_id").(string); got != "tenant-1" {
		t.Errorf("tenant_id = %q, want %q", got, "tenant-1")
	}
}

func TestUserImportRejectsBadFormat(t *testing.T) {
	for _, id := range []string{"user-only", "tenant-1:", ":user-2", "", ":"} {
		d := schema.TestResourceDataRaw(t, resourceFronteggUser().Schema, map[string]interface{}{})
		d.SetId(id)
		if _, err := resourceFronteggUserImport(context.Background(), d, nil); err == nil {
			t.Errorf("import(%q) = nil error, want error", id)
		}
	}
}

// Moving a user between tenants is not something these endpoints support, so
// the change has to replace the resource rather than silently orphan the user
// in the old tenant.
func TestUserTenantIDForcesNew(t *testing.T) {
	tenantID := resourceFronteggUser().Schema["tenant_id"]
	if !tenantID.ForceNew {
		t.Error("tenant_id must force replacement")
	}
}

const testAccUserTenantSwitch = `
resource "frontegg_tenant" "first" {
  key  = "tf-acc-user-tenant-first"
  name = "tf-acc user tenant first"
}

resource "frontegg_tenant" "second" {
  key  = "tf-acc-user-tenant-second"
  name = "tf-acc user tenant second"
}

resource "frontegg_application_tenant_assignment" "first" {
  app_id    = "%[1]s"
  tenant_id = frontegg_tenant.first.id
}

resource "frontegg_application_tenant_assignment" "second" {
  app_id    = "%[1]s"
  tenant_id = frontegg_tenant.second.id
}

resource "frontegg_role" "switch" {
  key            = "tf-acc-user-switch-role"
  name           = "tf-acc user switch role"
  description    = "role for the tenant switch test"
  level          = 1
  default        = false
  permission_ids = []
}

resource "frontegg_user" "switch" {
  tenant_id         = frontegg_tenant.%[2]s.id
  email             = "tf-acc-user-switch@example.com"
  role_ids          = [frontegg_role.switch.id]
  skip_invite_email = true
  depends_on = [
    frontegg_application_tenant_assignment.first,
    frontegg_application_tenant_assignment.second,
  ]
}
`

func testAccUserTenantSwitchConfig(tenant string) string {
	return fmt.Sprintf(testAccUserTenantSwitch, os.Getenv("FRONTEGG_APPLICATION_ID"), tenant)
}

// tenant_id is ForceNew because the identity API has no way to move a user
// between tenants. Before that, changing it updated state while leaving the
// user in the original tenant, and the following plan proposed a fresh create
// on top of the orphan. This checks the replacement lands and settles.
func TestAccFronteggUser_tenantChangeReplaces(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckApplication(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserTenantSwitchConfig("first"),
				Check: resource.TestCheckResourceAttrPair(
					"frontegg_user.switch", "tenant_id", "frontegg_tenant.first", "id"),
			},
			{
				Config: testAccUserTenantSwitchConfig("second"),
				Check: resource.TestCheckResourceAttrPair(
					"frontegg_user.switch", "tenant_id", "frontegg_tenant.second", "id"),
			},
			{
				Config:   testAccUserTenantSwitchConfig("second"),
				PlanOnly: true,
			},
		},
	})
}
