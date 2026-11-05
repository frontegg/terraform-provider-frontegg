package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type fakeUserRoleStore struct {
	assignments              map[string]map[userRoleAssignment]bool
	environmentRolesIgnored  bool
	environmentRolesRejected bool
}

func newFakeUserRoleAPI(t *testing.T, store *fakeUserRoleStore) *fakeAPI {
	return newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, "/identity/resources/users/v1/{id}/roles", store.change(true)},
		fakeAPIRoute{http.MethodDelete, "/identity/resources/users/v1/{id}/roles", store.change(false)},
		fakeAPIRoute{http.MethodGet, "/identity/resources/users/v3/roles", store.list},
	)
}

func (s *fakeUserRoleStore) change(assign bool) func(*fakeAPI, string, fakeDoc) (int, interface{}) {
	return func(_ *fakeAPI, userID string, body fakeDoc) (int, interface{}) {
		user, ok := s.assignments[userID]
		if !ok {
			return http.StatusNotFound, nil
		}
		changed := []userRoleAssignment{}
		roleIDs, _ := body["roleIds"].([]interface{})
		for _, roleID := range roleIDs {
			changed = append(changed, userRoleAssignment{RoleID: roleID.(string)})
		}
		environmentRoles, _ := body["environmentRoles"].([]interface{})
		if assign && s.environmentRolesRejected && len(environmentRoles) > 0 {
			return http.StatusBadRequest, fakeDoc{"errors": []string{"Environment-scoped roles are not enabled"}, "errorCode": "ER-01262"}
		}
		for _, item := range environmentRoles {
			if s.environmentRolesIgnored {
				break
			}
			environment := item.(map[string]interface{})
			for _, roleID := range environment["roleIds"].([]interface{}) {
				changed = append(changed, userRoleAssignment{RoleID: roleID.(string), EnvironmentID: environment["environmentId"].(string)})
			}
		}
		for _, assignment := range changed {
			if assign {
				user[assignment] = true
			} else {
				delete(user, assignment)
			}
		}
		return http.StatusCreated, nil
	}
}

func (s *fakeUserRoleStore) list(_ *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
	users := []fronteggUserRoles{}
	for userID, assignments := range s.assignments {
		user := fronteggUserRoles{UserID: userID, RoleIDs: []string{}}
		roleIDsByEnvironment := map[string][]string{}
		for assignment := range assignments {
			if assignment.EnvironmentID == "" {
				user.RoleIDs = append(user.RoleIDs, assignment.RoleID)
				continue
			}
			roleIDsByEnvironment[assignment.EnvironmentID] = append(roleIDsByEnvironment[assignment.EnvironmentID], assignment.RoleID)
		}
		for environmentID, roleIDs := range roleIDsByEnvironment {
			user.EnvironmentRoles = append(user.EnvironmentRoles, fronteggEnvironmentRoles{EnvironmentID: environmentID, RoleIDs: roleIDs})
		}
		users = append(users, user)
	}
	return http.StatusOK, users
}

func (s *fakeUserRoleStore) expect(userID string, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		got := []string{}
		for assignment := range s.assignments[userID] {
			got = append(got, assignment.String())
		}
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, "; ") != strings.Join(want, "; ") {
			return fmt.Errorf("user %s has %v, want %v", userID, got, want)
		}
		return nil
	}
}

func userEnvRoleConfig(assignments ...userRoleAssignment) string {
	var blocks strings.Builder
	for _, assignment := range assignments {
		blocks.WriteString("  assignment {\n")
		fmt.Fprintf(&blocks, "    role_id = %q\n", assignment.RoleID)
		if assignment.EnvironmentID != "" {
			fmt.Fprintf(&blocks, "    environment_id = %q\n", assignment.EnvironmentID)
		}
		blocks.WriteString("  }\n")
	}
	return fmt.Sprintf(`
resource "frontegg_user_env_role" "test" {
  tenant_id = "tenant-1"
  user_id   = "user-1"
%s}
`, blocks.String())
}

func TestUserEnvRoleLifecycle(t *testing.T) {
	unmanaged := userRoleAssignment{RoleID: "unmanaged"}
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{
		"user-1": {unmanaged: true},
	}}
	newFakeUserRoleAPI(t, store)

	global := userRoleAssignment{RoleID: "admin"}
	stagingViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"}
	stagingEditor := userRoleAssignment{RoleID: "editor", EnvironmentID: "staging"}
	productionViewer := userRoleAssignment{RoleID: "viewer", EnvironmentID: "production"}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      store.expect("user-1", unmanaged.String()),
		Steps: []resource.TestStep{
			{
				Config: userEnvRoleConfig(global, stagingViewer, stagingEditor),
				Check: resource.ComposeAggregateTestCheckFunc(
					store.expect("user-1", unmanaged.String(), global.String(), stagingViewer.String(), stagingEditor.String()),
					resource.TestCheckResourceAttr("frontegg_user_env_role.test", "id", "tenant-1:user-1"),
					resource.TestCheckResourceAttr("frontegg_user_env_role.test", "assignment.#", "3"),
				),
			},
			{
				Config:   userEnvRoleConfig(global, stagingViewer, stagingEditor),
				PlanOnly: true,
			},
			{
				Config: userEnvRoleConfig(global, stagingViewer, productionViewer),
				Check:  store.expect("user-1", unmanaged.String(), global.String(), stagingViewer.String(), productionViewer.String()),
			},
			{
				PreConfig: func() { delete(store.assignments["user-1"], productionViewer) },
				Config:    userEnvRoleConfig(global, stagingViewer, productionViewer),
				Check:     store.expect("user-1", unmanaged.String(), global.String(), stagingViewer.String(), productionViewer.String()),
			},
			{
				ResourceName:  "frontegg_user_env_role.test",
				ImportState:   true,
				ImportStateId: "tenant-1:user-1",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if got := states[0].Attributes["assignment.#"]; got != "4" {
						return fmt.Errorf("imported %s assignments, want 4", got)
					}
					return nil
				},
			},
		},
	})
}

func TestUserEnvRoleReportsIgnoredEnvironmentRoles(t *testing.T) {
	store := &fakeUserRoleStore{
		assignments:             map[string]map[userRoleAssignment]bool{"user-1": {}},
		environmentRolesIgnored: true,
	}
	newFakeUserRoleAPI(t, store)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      userEnvRoleConfig(userRoleAssignment{RoleID: "admin"}, userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"}),
				ExpectError: regexp.MustCompile(`role viewer in environment staging[\s\S]*env_role_assignment feature flag`),
			},
		},
	})
}

func TestUserEnvRoleRejectedCreateKeepsExistingRoles(t *testing.T) {
	admin := userRoleAssignment{RoleID: "admin"}
	store := &fakeUserRoleStore{
		assignments:              map[string]map[userRoleAssignment]bool{"user-1": {admin: true}},
		environmentRolesRejected: true,
	}
	newFakeUserRoleAPI(t, store)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      userEnvRoleConfig(admin, userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"}),
				ExpectError: regexp.MustCompile(`Environment-scoped roles are not enabled`),
			},
			{
				Config: "locals {}",
				Check:  store.expect("user-1", admin.String()),
			},
		},
	})
}

const emptyStringVariable = `
variable "empty" {
  default = ""
}
`

func TestUserEnvRoleRejectsEmptyEnvironmentID(t *testing.T) {
	newFakeUserRoleAPI(t, &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{"user-1": {}}})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      emptyStringVariable + userEnvRoleConfig(userRoleAssignment{RoleID: "admin", EnvironmentID: "${var.empty}"}),
				ExpectError: regexp.MustCompile(`to not be an empty string`),
			},
		},
	})
}

func TestMissingUserRoleAssignmentsErrorNamesFlagOnlyForEnvironmentRoles(t *testing.T) {
	globalOnly := missingUserRoleAssignmentsError("user-1", []userRoleAssignment{{RoleID: "admin"}})
	if strings.Contains(globalOnly.Error(), "env_role_assignment") {
		t.Errorf("a missing global role should not blame the feature flag: %s", globalOnly)
	}
	if !strings.Contains(globalOnly.Error(), "role ID exists") {
		t.Errorf("a missing global role should suggest checking the role ID: %s", globalOnly)
	}
	scoped := missingUserRoleAssignmentsError("user-1", []userRoleAssignment{{RoleID: "admin"}, {RoleID: "viewer", EnvironmentID: "staging"}})
	if !strings.Contains(scoped.Error(), "env_role_assignment") {
		t.Errorf("a missing environment role should name the feature flag: %s", scoped)
	}
	if !strings.Contains(scoped.Error(), "role ID exists") {
		t.Errorf("a missing environment role should also suggest checking the role ID, since an unknown role is dropped even with the flag on: %s", scoped)
	}
	if missingUserRoleAssignmentsError("user-1", nil) != nil {
		t.Error("nothing missing should not be an error")
	}
}

func TestUserEnvRoleRemovedWithUser(t *testing.T) {
	store := &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{"user-1": {}}}
	newFakeUserRoleAPI(t, store)
	config := userEnvRoleConfig(userRoleAssignment{RoleID: "viewer", EnvironmentID: "staging"})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { delete(store.assignments, "user-1") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestUserEnvRoleImportRejectsMalformedID(t *testing.T) {
	newFakeUserRoleAPI(t, &fakeUserRoleStore{assignments: map[string]map[userRoleAssignment]bool{}})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        userEnvRoleConfig(userRoleAssignment{RoleID: "admin"}),
				ResourceName:  "frontegg_user_env_role.test",
				ImportState:   true,
				ImportStateId: "user-1",
				ExpectError:   regexp.MustCompile(`expected tenant_id:user_id`),
			},
		},
	})
}
