package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

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
	return http.StatusOK, fronteggUser{Key: userID, Email: "user@example.com", ReadRoleIDs: roles}
}

func (s *fakeUserRoleStore) deleteUser(_ *fakeAPI, userID string, _ fakeDoc) (int, interface{}) {
	delete(s.assignments, userID)
	return http.StatusOK, nil
}

func userWithRolesConfig(roleIDs []string, environmentRoles ...userRoleAssignment) string {
	var arguments strings.Builder
	if roleIDs != nil {
		fmt.Fprintf(&arguments, "  role_ids = [%q]\n", strings.Join(roleIDs, `", "`))
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
				Config:      emptyEnvironmentVariable + userWithRolesConfig(nil, userRoleAssignment{RoleID: "viewer", EnvironmentID: "${var.environment_id}"}),
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
