package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

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

func testAccPreCheckApplication(t *testing.T) {
	testAccPreCheck(t)
	if os.Getenv("FRONTEGG_APPLICATION_ID") == "" {
		t.Skip("FRONTEGG_APPLICATION_ID must be set for user acceptance tests")
	}
}

const fakeDefaultRoleID = "default"

func newFakeUserWithRolesAPI(t *testing.T, store *fakeUserRoleStore) *fakeAPI {
	return newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, "/identity/resources/users/v2", store.createUser},
		fakeAPIRoute{http.MethodGet, "/identity/resources/users/v1/{id}", store.getUser},
		fakeAPIRoute{http.MethodDelete, "/identity/resources/users/v1/{id}", store.deleteUser},
		fakeAPIRoute{http.MethodPost, "/identity/resources/users/v1/{id}/roles", store.change(true)},
		fakeAPIRoute{http.MethodDelete, "/identity/resources/users/v1/{id}/roles", store.change(false)},
		fakeAPIRoute{http.MethodGet, "/identity/resources/users/v3/roles", store.list},
	)
}

func (s *fakeUserRoleStore) createUser(api *fakeAPI, _ string, body fakeDoc) (int, interface{}) {
	api.nextID++
	userID := fmt.Sprintf("user-%d", api.nextID)
	s.assignments[userID] = map[userRoleAssignment]bool{}
	if _, hasRoleIDs := body["roleIds"]; !hasRoleIDs {
		s.assignments[userID][userRoleAssignment{RoleID: fakeDefaultRoleID}] = true
	}
	s.change(true)(api, userID, body)
	return s.getUser(api, userID, nil)
}

func (s *fakeUserRoleStore) getUser(_ *fakeAPI, userID string, _ fakeDoc) (int, interface{}) {
	assignments, ok := s.assignments[userID]
	if !ok {
		return http.StatusNotFound, nil
	}
	roles := []fronteggUserRole{}
	for assignment := range assignments {
		if assignment.EnvironmentID == "" {
			roles = append(roles, fronteggUserRole{Id: assignment.RoleID})
		}
	}
	environmentRoles := map[string][]fronteggUserRole{}
	for assignment := range assignments {
		if assignment.EnvironmentID != "" {
			environmentRoles[assignment.EnvironmentID] = append(environmentRoles[assignment.EnvironmentID], fronteggUserRole{Id: assignment.RoleID})
		}
	}
	return http.StatusOK, fakeDoc{"id": userID, "email": "user@example.com", "roles": roles, "environmentRoles": environmentRoles}
}

func (s *fakeUserRoleStore) deleteUser(_ *fakeAPI, userID string, _ fakeDoc) (int, interface{}) {
	delete(s.assignments, userID)
	return http.StatusOK, nil
}

func userWithRolesConfig(roleIDs []string, environmentRoles ...userRoleAssignment) string {
	var arguments strings.Builder
	if roleIDs != nil {
		quoted := []string{}
		for _, roleID := range roleIDs {
			quoted = append(quoted, fmt.Sprintf("%q", roleID))
		}
		fmt.Fprintf(&arguments, "  role_ids = [%s]\n", strings.Join(quoted, ", "))
	}
	for _, environmentRole := range environmentRoles {
		fmt.Fprintf(&arguments, "  environment_role {\n    environment_id = %q\n    role_id = %q\n  }\n", environmentRole.EnvironmentID, environmentRole.RoleID)
	}
	return fmt.Sprintf(`
resource "frontegg_user" "test" {
  email     = "user@example.com"
  tenant_id = "tenant-1"
%s}
`, arguments.String())
}

func TestUserEnvironmentRoles(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}}
	newFakeUserWithRolesAPI(t, store)

	defaultRole := userRoleAssignment{RoleID: fakeDefaultRoleID}
	admin := userRoleAssignment{RoleID: "admin"}
	stagingViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"}
	stagingEditor := userRoleAssignment{RoleID: "editor", EnvironmentID: "staging"}
	productionViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "production"}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: userWithRolesConfig(nil, stagingViewer, stagingEditor),
				Check: resource.ComposeAggregateTestCheckFunc(
					store.expect("user-1", defaultRole.String(), stagingViewer.String(), stagingEditor.String()),
					resource.TestCheckResourceAttr("frontegg_user.test", "role_ids.#", "1"),
					resource.TestCheckResourceAttr("frontegg_user.test", "environment_role.#", "2"),
				),
			},
			{
				Config:   userWithRolesConfig(nil, stagingViewer, stagingEditor),
				PlanOnly: true,
			},
			{
				Config: userWithRolesConfig(nil, stagingViewer, productionViewer),
				Check:  store.expect("user-1", defaultRole.String(), stagingViewer.String(), productionViewer.String()),
			},
			{
				PreConfig: func() { store.assignments["user-1"][stagingEditor] = true },
				Config:    userWithRolesConfig(nil, stagingViewer, productionViewer),
				Check:     store.expect("user-1", defaultRole.String(), stagingViewer.String(), productionViewer.String()),
			},
			{
				Config: userWithRolesConfig([]string{admin.RoleID}),
				Check:  store.expect("user-1", admin.String()),
			},
		},
	})
}

func TestUserEnvironmentRolesIgnoredFlagKeepsUser(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}, environmentRolesIgnored: true}
	api := newFakeUserWithRolesAPI(t, store)
	config := userWithRolesConfig(nil, userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if api.nextID != 1 {
				return fmt.Errorf("the provider created %d users, want 1", api.nextID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:             config,
				Check:              resource.TestCheckResourceAttr("frontegg_user.test", "id", "user-1"),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`role viewer in environment staging[\s\S]*env_role_assignment feature flag`),
			},
		},
	})
}

func TestUserEnvironmentRoleRejectsEmptyIDs(t *testing.T) {
	newFakeUserWithRolesAPI(t, &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      emptyStringVariable + userWithRolesConfig(nil, userRoleAssignment{RoleID: "viewer", EnvironmentID: "${var.empty}"}),
				ExpectError: regexp.MustCompile(`to not be an empty string`),
			},
			{
				Config:      emptyStringVariable + userWithRolesConfig(nil, userRoleAssignment{RoleID: "${var.empty}", EnvironmentID: "staging"}),
				ExpectError: regexp.MustCompile(`to not be an empty string`),
			},
		},
	})
}

func TestUserRequiresGlobalOrEnvironmentRoles(t *testing.T) {
	newFakeUserWithRolesAPI(t, &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      userWithRolesConfig(nil),
				ExpectError: regexp.MustCompile(`one of\s+` + "`environment_role,role_ids`"),
			},
		},
	})
}

func TestUserWithEnvironmentRolesInUserEnvRole(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}}
	newFakeUserWithRolesAPI(t, store)

	admin := userRoleAssignment{RoleID: "admin"}
	stagingViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"}
	productionViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "production"}
	config := func(environmentID string) string {
		return userWithRolesConfig([]string{admin.RoleID}) + fmt.Sprintf(`
resource "frontegg_user_env_role" "test" {
  tenant_id = "tenant-1"
  user_id   = frontegg_user.test.id

  assignment {
    role_id        = "viewer"
    environment_id = %q
  }
}
`, environmentID)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("staging"),
				Check:  store.expect("user-1", admin.String(), stagingViewer.String()),
			},
			{
				Config:   config("staging"),
				PlanOnly: true,
			},
			{
				Config: config("production"),
				Check:  store.expect("user-1", admin.String(), productionViewer.String()),
			},
			{
				Config:   config("production"),
				PlanOnly: true,
			},
		},
	})
}

func TestUserUnknownGlobalRoleIsReported(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}, unknownRoleIDs: map[string]bool{"ghost": true}}
	newFakeUserWithRolesAPI(t, store)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: userWithRolesConfig([]string{"admin"}),
				Check:  store.expect("user-1", "global role admin"),
			},
			{
				Config:      userWithRolesConfig([]string{"admin", "ghost"}),
				ExpectError: regexp.MustCompile(`global role ghost[\s\S]*role ID exists`),
			},
		},
	})
}

func TestUserCreatedWithUnknownGlobalRoleIsKept(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}, unknownRoleIDs: map[string]bool{"ghost": true}}
	api := newFakeUserWithRolesAPI(t, store)
	config := userWithRolesConfig([]string{"admin", "ghost"})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if api.nextID != 1 {
				return fmt.Errorf("the provider created %d users, want 1", api.nextID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:             config,
				Check:              store.expect("user-1", "global role admin"),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`global role ghost[\s\S]*role ID exists`),
			},
		},
	})
}
