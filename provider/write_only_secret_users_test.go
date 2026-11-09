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

type userPasswordCase struct {
	address  string
	config   string
	importID func(userID string) string
}

var userPasswordCases = map[string]userPasswordCase{
	"frontegg_user": {
		address: "frontegg_user.test",
		config: `
resource "frontegg_user" "test" {
  email     = "user@example.com"
  tenant_id = "tenant-1"
  role_ids  = ["role-1"]
%s}
`,
		importID: func(userID string) string { return "tenant-1:" + userID },
	},
	"frontegg_portal_user": {
		address: "frontegg_portal_user.test",
		config: `
resource "frontegg_portal_user" "test" {
  email    = "portal@example.com"
  role_ids = ["role-1"]
%s}
`,
		importID: func(userID string) string { return userID },
	},
}

func fakeUserAPI(t *testing.T) *fakeAPI {
	t.Helper()
	return newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, fronteggUserPath, func(a *fakeAPI, _ string, body fakeDoc) (int, interface{}) {
			a.nextID++
			id := fmt.Sprintf("user-%d", a.nextID)
			roles := []interface{}{}
			for _, roleID := range body["roleIds"].([]interface{}) {
				roles = append(roles, fakeDoc{"id": roleID})
			}
			return http.StatusOK, a.put(id, fakeDoc{"id": id, "email": body["email"], "roles": roles})
		}},
		fakeAPIRoute{http.MethodGet, fronteggUserPathV1 + "/{id}", fakeGet},
		fakeAPIRoute{http.MethodDelete, fronteggUserPathV1 + "/{id}", fakeDelete},
		fakeAPIRoute{http.MethodPost, fronteggUserPathV1 + "/passwords/change", func(*fakeAPI, string, fakeDoc) (int, interface{}) {
			return http.StatusOK, nil
		}},
	)
}

func (c userPasswordCase) render(arguments string) string {
	return `
variable "secret" {
  type      = string
  ephemeral = true
  default   = "ephemeral"
}
` + fmt.Sprintf(c.config, arguments)
}

func checkNoPasswordChange(api *fakeAPI) resource.TestCheckFunc {
	return func(*terraform.State) error {
		api.mu.Lock()
		defer api.mu.Unlock()
		for _, body := range api.writes {
			if _, changed := body["newPassword"]; changed {
				return fmt.Errorf("provider called the password change endpoint, which needs the current password")
			}
		}
		return nil
	}
}

func TestUserWriteOnlyPasswordLifecycle(t *testing.T) {
	for name, c := range userPasswordCases {
		t.Run(name, func(t *testing.T) {
			skipBelowWriteOnlyTerraform(t)
			api := fakeUserAPI(t)
			writeOnly := func(version int) string {
				return fmt.Sprintf("  password_wo         = \"${var.secret}-password\"\n  password_wo_version = %d\n", version)
			}

			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: testAccProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: c.render(writeOnly(1)),
						Check: resource.ComposeAggregateTestCheckFunc(
							func(*terraform.State) error {
								if got := api.lastSent("password"); got != "ephemeral-password" {
									return fmt.Errorf("API received password %q, want the write-only value", got)
								}
								return nil
							},
							resource.TestCheckNoResourceAttr(c.address, "password"),
							checkSecretOutOfState("ephemeral-password"),
						),
					},
					{
						Config:   c.render(writeOnly(1)),
						PlanOnly: true,
					},
					{
						Config:      c.render(writeOnly(2)),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(`password_wo is only sent when the user is created`),
					},
					{
						ResourceName:      c.address,
						ImportState:       true,
						ImportStateIdFunc: func(*terraform.State) (string, error) { return c.importID("user-1"), nil },
						ImportStateCheck: func(states []*terraform.InstanceState) error {
							for _, state := range states {
								for key, value := range state.Attributes {
									if value == "ephemeral-password" {
										return fmt.Errorf("imported state attribute %s holds the password", key)
									}
								}
							}
							return nil
						},
					},
				},
			})
		})
	}
}

func TestUserPasswordMovesToWriteOnly(t *testing.T) {
	for name, c := range userPasswordCases {
		t.Run(name, func(t *testing.T) {
			skipBelowWriteOnlyTerraform(t)
			api := fakeUserAPI(t)
			writeOnly := "  password_wo         = \"${var.secret}-password\"\n  password_wo_version = 1\n"

			resource.UnitTest(t, resource.TestCase{
				ProviderFactories: testAccProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: c.render("  password = \"plain-password\"\n"),
						Check:  resource.TestCheckResourceAttr(c.address, "password", "plain-password"),
					},
					{
						Config: c.render(writeOnly),
						Check: resource.ComposeAggregateTestCheckFunc(
							checkNoPasswordChange(api),
							resource.TestCheckResourceAttr(c.address, "password", ""),
							checkSecretOutOfState("plain-password", "ephemeral-password"),
						),
					},
					{
						Config:   c.render(writeOnly),
						PlanOnly: true,
					},
				},
			})
		})
	}
}

func TestUserPasswordWithoutCurrentIsRecordedNotSent(t *testing.T) {
	skipBelowWriteOnlyTerraform(t)
	api := fakeUserAPI(t)
	c := userPasswordCases["frontegg_user"]

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: c.render("  password_wo         = \"${var.secret}-password\"\n  password_wo_version = 1\n")},
			{
				Config: c.render("  password = \"plain-password\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkNoPasswordChange(api),
					resource.TestCheckResourceAttr(c.address, "password", "plain-password"),
				),
			},
			{
				Config:   c.render("  password = \"plain-password\"\n"),
				PlanOnly: true,
			},
		},
	})
}

func TestUserReplacementRejectsPasswordVersionBump(t *testing.T) {
	skipBelowWriteOnlyTerraform(t)
	api := fakeUserAPI(t)
	c := userPasswordCases["frontegg_user"]
	inTenant := func(tenantID string, arguments string) string {
		return strings.Replace(c.render(arguments), `tenant_id = "tenant-1"`, fmt.Sprintf("tenant_id = %q", tenantID), 1)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: inTenant("tenant-1", "  password_wo         = \"first-password\"\n  password_wo_version = 1\n")},
			{
				Config:      inTenant("tenant-2", "  password_wo         = \"second-password\"\n  password_wo_version = 2\n"),
				ExpectError: regexp.MustCompile(`set password_wo_version back to 1`),
			},
			{
				Config: inTenant("tenant-2", "  password_wo         = \"first-password\"\n  password_wo_version = 1\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(*terraform.State) error {
						if got := api.lastSent("password"); got != "first-password" {
							return fmt.Errorf("replacement user was created with password %q, want the current write-only value", got)
						}
						return nil
					},
					checkSecretOutOfState("first-password"),
				),
			},
		},
	})
}
