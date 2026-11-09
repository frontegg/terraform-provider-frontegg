package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type fakeSSOBackend struct {
	mu      sync.Mutex
	nextID  int
	config  map[string]interface{}
	groups  []map[string]interface{}
	domains []map[string]interface{}
}

func newFakeSSOBackend(t *testing.T) *fakeSSOBackend {
	t.Helper()
	b := &fakeSSOBackend{}
	srv := httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(srv.Close)
	t.Setenv("FRONTEGG_API_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_PORTAL_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")
	return b
}

func (b *fakeSSOBackend) newID(kind string) string {
	b.nextID++
	return fmt.Sprintf("%s-%d", kind, b.nextID)
}

func (b *fakeSSOBackend) groupIDs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ids := []string{}
	for _, g := range b.groups {
		ids = append(ids, g["id"].(string))
	}
	return ids
}

func (b *fakeSSOBackend) recreateChildren() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, g := range b.groups {
		g["id"] = b.newID("group")
	}
	for _, dom := range b.domains {
		dom["id"] = b.newID("domain")
	}
}

func (b *fakeSSOBackend) setDefaultRoles(roleIDs ...string) func() {
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		roles := []interface{}{}
		for _, id := range roleIDs {
			roles = append(roles, id)
		}
		b.config["roleIds"] = roles
	}
}

func (b *fakeSSOBackend) defaultRoles() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fmt.Sprint(b.config["roleIds"])
}

func (b *fakeSSOBackend) serve(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.URL.Path == "/auth/vendor" {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
		return
	}
	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	status, out := b.route(r.Method, r.URL.Path, body)
	w.WriteHeader(status)
	if out != nil {
		_ = json.NewEncoder(w).Encode(out)
	}
}

func (b *fakeSSOBackend) route(method, path string, body map[string]interface{}) (int, interface{}) {
	switch {
	case method == http.MethodPost && path == fronteggTenantSSOConfigV2Path:
		b.config = body
		b.config["id"] = b.newID("config")
		return http.StatusCreated, b.config
	case method == http.MethodGet && path == fronteggTenantSSOConfigPath:
		if b.config == nil {
			return http.StatusOK, []interface{}{}
		}
		out := map[string]interface{}{"groups": b.groups, "domains": b.domains}
		for k, v := range b.config {
			out[k] = v
		}
		return http.StatusOK, []interface{}{out}
	case method == http.MethodPatch && path == fronteggTenantSSOConfigV2Path+"/"+b.configID():
		b.groups = []map[string]interface{}{}
		groups, _ := body["groups"].([]interface{})
		for _, g := range groups {
			group := g.(map[string]interface{})
			group["id"] = b.newID("group")
			b.groups = append(b.groups, group)
		}
		delete(body, "groups")
		b.config["roleIds"] = []interface{}{}
		for k, v := range body {
			b.config[k] = v
		}
		return http.StatusOK, nil
	case method == http.MethodDelete && path == fronteggTenantSSOConfigPath+"/"+b.configID():
		b.config, b.groups, b.domains = nil, nil, nil
		return http.StatusOK, nil
	}
	return b.routeChildren(method, path, body)
}

func (b *fakeSSOBackend) routeChildren(method, path string, body map[string]interface{}) (int, interface{}) {
	groupsPath := fmt.Sprintf(fronteggSSOGroupPath, b.configID())
	domainsPath := fronteggTenantSSOConfigPath + "/" + b.configID() + "/domains"
	switch {
	case method == http.MethodGet && path == groupsPath:
		return http.StatusOK, b.groups
	case method == http.MethodPost && path == groupsPath:
		body["id"] = b.newID("group")
		body["enabled"] = true
		b.groups = append(b.groups, body)
		return http.StatusCreated, body
	case method == http.MethodPatch && strings.HasPrefix(path, groupsPath+"/"):
		group := findByID(b.groups, strings.TrimPrefix(path, groupsPath+"/"))
		if group == nil {
			return http.StatusNotFound, nil
		}
		group["roleIds"] = body["roleIds"]
		return http.StatusOK, nil
	case method == http.MethodDelete && strings.HasPrefix(path, groupsPath+"/"):
		id := strings.TrimPrefix(path, groupsPath+"/")
		if findByID(b.groups, id) == nil {
			return http.StatusNotFound, nil
		}
		b.groups = removeByID(b.groups, id)
		return http.StatusOK, nil
	case method == http.MethodPost && path == domainsPath:
		body["id"] = b.newID("domain")
		body["validated"] = false
		b.domains = append(b.domains, body)
		return http.StatusCreated, body
	case method == http.MethodDelete && strings.HasPrefix(path, domainsPath+"/"):
		b.domains = removeByID(b.domains, strings.TrimPrefix(path, domainsPath+"/"))
		return http.StatusOK, nil
	}
	return http.StatusNotFound, nil
}

func (b *fakeSSOBackend) configID() string {
	if b.config == nil {
		return ""
	}
	return b.config["id"].(string)
}

func findByID(items []map[string]interface{}, id string) map[string]interface{} {
	for _, item := range items {
		if item["id"] == id {
			return item
		}
	}
	return nil
}

func removeByID(items []map[string]interface{}, id string) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, item := range items {
		if item["id"] != id {
			out = append(out, item)
		}
	}
	return out
}

func tenantSSOConfigWithChildren(endpoint string, skipEmailDomainValidation bool) string {
	return tenantSSOConfigWithMapping(endpoint, skipEmailDomainValidation, "role-1")
}

func tenantSSOConfigWithMapping(endpoint string, skipEmailDomainValidation bool, roleID string) string {
	config := fmt.Sprintf(`
resource "frontegg_tenant_oidc_config" "test" {
  tenant_id                    = "tenant-1"
  enabled                      = false
  sso_endpoint                 = %q
  oidc_client_id               = "oidc-client"
  oidc_secret                  = "oidc-secret"
  skip_email_domain_validation = %t
}

resource "frontegg_tenant_sso_domain" "test" {
  tenant_id     = "tenant-1"
  sso_config_id = frontegg_tenant_oidc_config.test.id
  domain        = "example.com"
}
`, endpoint, skipEmailDomainValidation)
	if roleID == "" {
		return config
	}
	return config + fmt.Sprintf(`
resource "frontegg_tenant_sso_group_mapping" "test" {
  tenant_id     = "tenant-1"
  sso_config_id = frontegg_tenant_oidc_config.test.id
  group         = "admins"
  role_ids      = [%q]
}
`, roleID)
}

func checkGroupMappingMatchesBackend(b *fakeSSOBackend) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ids := b.groupIDs()
		if len(ids) != 1 {
			return fmt.Errorf("backend has %d group mappings, want 1", len(ids))
		}
		if got := s.RootModule().Resources["frontegg_tenant_sso_group_mapping.test"].Primary.ID; got != ids[0] {
			return fmt.Errorf("group mapping ID in state = %q, backend has %q", got, ids[0])
		}
		return nil
	}
}

func checkBackendHasOneGroupMapping(b *fakeSSOBackend) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if ids := b.groupIDs(); len(ids) != 1 {
			return fmt.Errorf("backend has %d group mappings, want 1", len(ids))
		}
		return nil
	}
}

func TestTenantSSOConfigUpdateKeepsGroupMappings(t *testing.T) {
	b := newFakeSSOBackend(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tenantSSOConfigWithChildren("https://idp-a.example.com", false),
				Check:  checkGroupMappingMatchesBackend(b),
			},
			{
				Config: tenantSSOConfigWithChildren("https://idp-b.example.com", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_tenant_oidc_config.test", "sso_endpoint", "https://idp-b.example.com"),
					checkBackendHasOneGroupMapping(b),
				),
			},
			{
				Config: tenantSSOConfigWithChildren("https://idp-b.example.com", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_tenant_oidc_config.test", "skip_email_domain_validation", "true"),
					checkBackendHasOneGroupMapping(b),
				),
			},
			{
				Config: tenantSSOConfigWithChildren("https://idp-b.example.com", true),
				Check:  checkGroupMappingMatchesBackend(b),
			},
		},
	})
}

func TestTenantSSOChildrenAdoptRecreatedIDs(t *testing.T) {
	b := newFakeSSOBackend(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tenantSSOConfigWithChildren("https://idp-a.example.com", false),
			},
			{
				PreConfig: b.recreateChildren,
				Config:    tenantSSOConfigWithChildren("https://idp-a.example.com", false),
				PlanOnly:  true,
			},
			{
				Config: tenantSSOConfigWithChildren("https://idp-a.example.com", false),
				Check:  checkGroupMappingMatchesBackend(b),
			},
		},
	})
}

func TestTenantSSOConfigUpdateKeepsDefaultRoles(t *testing.T) {
	b := newFakeSSOBackend(t)
	checkDefaultRoles := func(*terraform.State) error {
		if got := b.defaultRoles(); got != "[role-default]" {
			return fmt.Errorf("backend default roles = %s, want [role-default]", got)
		}
		return nil
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tenantSSOConfigWithChildren("https://idp-a.example.com", false),
			},
			{
				PreConfig: b.setDefaultRoles("role-default"),
				Config:    tenantSSOConfigWithChildren("https://idp-b.example.com", false),
				Check:     checkDefaultRoles,
			},
			{
				Config: tenantSSOConfigWithChildren("https://idp-b.example.com", true),
				Check:  checkDefaultRoles,
			},
		},
	})
}

func checkBackendGroupRoles(b *fakeSSOBackend, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		got := []string{}
		for _, g := range b.groups {
			roles, _ := g["roleIds"].([]interface{})
			for _, r := range roles {
				got = append(got, r.(string))
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return fmt.Errorf("backend group roles = %v, want %v", got, want)
		}
		return nil
	}
}

func TestTenantSSOGroupMappingChangesInSameApplyAsConfigUpdate(t *testing.T) {
	b := newFakeSSOBackend(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tenantSSOConfigWithMapping("https://idp-a.example.com", false, "role-1"),
			},
			{
				Config: tenantSSOConfigWithMapping("https://idp-b.example.com", false, "role-2"),
				Check:  checkBackendGroupRoles(b, "role-2"),
			},
			{
				Config: tenantSSOConfigWithMapping("https://idp-c.example.com", false, ""),
				Check:  checkBackendGroupRoles(b),
			},
		},
	})
}
