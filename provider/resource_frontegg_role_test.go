package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func roleResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggRole().Schema, raw)
}

func TestRoleImportIdParsing(t *testing.T) {
	for _, tc := range []struct {
		id         string
		wantTenant string
		wantId     string
		wantErr    bool
	}{
		{id: "tenant-1:role-1", wantTenant: "tenant-1", wantId: "role-1"},
		{id: "role-1", wantId: "role-1"},
		{id: ":role-1", wantErr: true},
		{id: "tenant-1:", wantErr: true},
	} {
		d := roleResourceData(t, map[string]interface{}{})
		d.SetId(tc.id)
		got, err := resourceFronteggRoleImport(context.Background(), d, nil)
		if tc.wantErr {
			if err == nil || !strings.Contains(err.Error(), "tenant_id:role_id") {
				t.Errorf("import %q: err = %v, want an error naming the tenant_id:role_id format", tc.id, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("import %q: %v", tc.id, err)
			continue
		}
		if got[0].Get("tenant_id").(string) != tc.wantTenant || got[0].Id() != tc.wantId {
			t.Errorf("import %q: tenant_id=%q id=%q", tc.id, got[0].Get("tenant_id"), got[0].Id())
		}
	}
}

// fakeRoleAPI scopes roles the way identity-service does: the v1 list returns vendor roles
// plus the header tenant's roles, PATCH and PUT permissions find only roles of the header's
// scope, PUT /tenant is vendor-level and moves a role to any tenant ID, and the v2 list
// without a tenant header pages the roles of every tenant. The tenants API answers an
// unknown tenant with an empty list.
type fakeRoleAPI struct {
	mu       sync.Mutex
	roles    []fronteggRole
	tenants  map[string]bool
	failMove bool
	requests []string
	nextID   int
}

func newFakeRoleAPI(t *testing.T, roles ...fronteggRole) (*fakeRoleAPI, *httptest.Server) {
	t.Helper()
	api := &fakeRoleAPI{roles: roles, tenants: map[string]bool{"tenant-1": true, "tenant-2": true}}
	srv := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(srv.Close)
	return api, srv
}

func roleClientHolder(srv *httptest.Server) *restclient.ClientHolder {
	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(srv.URL, "", "")}
	holder.ApiClient.Authenticate("test-token")
	return holder
}

func (api *fakeRoleAPI) sentMove() bool {
	for _, req := range api.requests {
		if strings.HasPrefix(req, "PUT ") && strings.HasSuffix(strings.Fields(req)[1], "/tenant") {
			return true
		}
	}
	return false
}

func (api *fakeRoleAPI) find(id string, inScope func(fronteggRole) bool) *fronteggRole {
	for i := range api.roles {
		if api.roles[i].ID == id && inScope(api.roles[i]) {
			return &api.roles[i]
		}
	}
	return nil
}

func (api *fakeRoleAPI) serve(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost && r.URL.Path == "/auth/vendor" {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
		return
	}
	tenant := r.Header.Get("frontegg-tenant-id")
	api.requests = append(api.requests, fmt.Sprintf("%s %s tenant=%s", r.Method, r.URL.Path, tenant))
	sameScope := func(role fronteggRole) bool { return role.TenantID == tenant }
	anyScope := func(fronteggRole) bool { return true }
	id, sub, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, fronteggRolePath+"/"), "/")

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, fronteggTenantPath+"/"):
		tenantID := strings.TrimPrefix(r.URL.Path, fronteggTenantPath+"/")
		out := []fronteggTenant{}
		if api.tenants[tenantID] {
			out = append(out, fronteggTenant{Key: tenantID})
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodGet && r.URL.Path == fronteggRoleV2Path:
		q := r.URL.Query()
		if q.Get("_sortBy") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		limit, _ := strconv.Atoi(q.Get("_limit"))
		offset, _ := strconv.Atoi(q.Get("_offset"))
		filter := q.Get("_filter")
		matched := []fronteggRole{}
		for _, role := range api.roles {
			if tenant != "" && role.TenantID != "" && role.TenantID != tenant {
				continue
			}
			if filter == "" || strings.Contains(role.Key, filter) || strings.Contains(role.Name, filter) {
				matched = append(matched, role)
			}
		}
		end := offset + limit
		if offset > len(matched) {
			offset = len(matched)
		}
		if end > len(matched) {
			end = len(matched)
		}
		_ = json.NewEncoder(w).Encode(fronteggRolePage{Items: matched[offset:end]})
	case r.Method == http.MethodGet && r.URL.Path == fronteggRolePath:
		out := []fronteggRole{}
		for _, role := range api.roles {
			if role.TenantID == "" || role.TenantID == tenant {
				out = append(out, role)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPost && r.URL.Path == fronteggRolePath:
		var in []fronteggRole
		_ = json.NewDecoder(r.Body).Decode(&in)
		for _, role := range api.roles {
			if role.Key == in[0].Key && sameScope(role) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string][]string{"errors": {"Roles already exists"}})
				return
			}
		}
		api.nextID++
		created := in[0]
		created.ID = fmt.Sprintf("role-%d", api.nextID)
		created.TenantID = tenant
		created.VendorID = "vendor-1"
		created.CreatedAt = "2026-01-01T00:00:00.000Z"
		created.Permissions = []string{}
		api.roles = append(api.roles, created)
		_ = json.NewEncoder(w).Encode([]fronteggRole{created})
	case r.Method == http.MethodPatch && sub == "":
		role := api.find(id, sameScope)
		if role == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var in fronteggRole
		_ = json.NewDecoder(r.Body).Decode(&in)
		role.Name, role.Key, role.Description = in.Name, in.Key, in.Description
		role.Level, role.IsDefault, role.FirstUserRole = in.Level, in.IsDefault, in.FirstUserRole
		_ = json.NewEncoder(w).Encode(role)
	case r.Method == http.MethodPut && sub == "permissions":
		role := api.find(id, sameScope)
		if role == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var in fronteggRolePermissions
		_ = json.NewDecoder(r.Body).Decode(&in)
		role.Permissions = in.PermissionIDs
		_ = json.NewEncoder(w).Encode(role)
	case r.Method == http.MethodPut && sub == "tenant":
		if api.failMove {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		role := api.find(id, anyScope)
		if role == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var in fronteggRoleTenant
		_ = json.NewDecoder(r.Body).Decode(&in)
		role.TenantID = ""
		if in.TenantID != nil {
			role.TenantID = *in.TenantID
		}
		_ = json.NewEncoder(w).Encode(role)
	case r.Method == http.MethodDelete && sub == "":
		for i, role := range api.roles {
			if role.ID == id && (tenant == "" || sameScope(role)) {
				api.roles = append(api.roles[:i], api.roles[i+1:]...)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestRoleCreateAdoptsTenantRoleNotVendorRoleWithSameKey(t *testing.T) {
	api, srv := newFakeRoleAPI(t,
		fronteggRole{ID: "vendor-role", Key: "admin", Permissions: []string{}},
		fronteggRole{ID: "tenant-role", Key: "admin", TenantID: "tenant-1", Permissions: []string{}},
	)
	holder := roleClientHolder(srv)
	d := roleResourceData(t, map[string]interface{}{
		"name":           "Admin",
		"key":            "admin",
		"description":    "tenant admin",
		"default":        false,
		"level":          1,
		"permission_ids": []interface{}{},
		"tenant_id":      "tenant-1",
	})
	d.MarkNewResource()

	if diags := resourceFronteggRoleCreate(context.Background(), d, holder); diags.HasError() {
		t.Fatalf("create: %v\nrequests: %v", diags, api.requests)
	}
	if d.Id() != "tenant-role" || d.Get("tenant_id").(string) != "tenant-1" {
		t.Errorf("adopted id=%q tenant_id=%q, want the tenant-1 role, not the vendor role sharing its key", d.Id(), d.Get("tenant_id"))
	}
	for _, req := range api.requests {
		if strings.HasSuffix(strings.Fields(req)[1], "/tenant") {
			t.Errorf("create sent %q, want the adopted role left in its tenant", req)
		}
	}
}

func roleState(tenantID string) *terraform.InstanceState {
	return &terraform.InstanceState{ID: "role-1", Attributes: map[string]string{
		"id": "role-1", "name": "probe", "key": "probe", "description": "probe",
		"default": "false", "first_user": "false", "level": "1", "permission_ids.#": "0", "tenant_id": tenantID,
	}}
}

// The SDK saves the planned values when Update fails, so a failed or refused move must leave
// the prior tenant in state; otherwise the next Read looks in the wrong tenant.
func TestRoleFailedMoveKeepsPriorTenantInState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		target   string
		failMove bool
		wantMove bool
		wantErr  string
	}{
		{name: "move request fails", target: "tenant-2", failMove: true, wantMove: true, wantErr: "500"},
		{name: "target tenant does not exist", target: "tenant-typo", wantErr: `tenant "tenant-typo" does not exist`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, srv := newFakeRoleAPI(t, fronteggRole{ID: "role-1", Key: "probe", Name: "probe", Description: "probe", Level: 1, TenantID: "tenant-1", Permissions: []string{}})
			api.failMove = tc.failMove
			holder := roleClientHolder(srv)
			res := resourceFronteggRole()
			prior := roleState("tenant-1")
			cfg := terraform.NewResourceConfigRaw(map[string]interface{}{
				"name": "probe", "key": "probe", "description": "probe",
				"default": false, "level": 1, "permission_ids": []interface{}{}, "tenant_id": tc.target,
			})
			diff, err := res.Diff(context.Background(), prior, cfg, holder)
			if err != nil {
				t.Fatalf("diff: %v", err)
			}

			got, diags := res.Apply(context.Background(), prior, diff, holder)

			if !diags.HasError() || !strings.Contains(fmt.Sprint(diags), tc.wantErr) {
				t.Errorf("diagnostics = %v, want an error containing %q", diags, tc.wantErr)
			}
			if got == nil || got.ID != "role-1" || got.Attributes["tenant_id"] != "tenant-1" {
				t.Errorf("state after the failed apply = %v, want role-1 still in tenant-1", got)
			}
			if api.sentMove() != tc.wantMove {
				t.Errorf("move sent = %v, want %v; requests: %v", api.sentMove(), tc.wantMove, api.requests)
			}
			if api.roles[0].TenantID != "tenant-1" {
				t.Errorf("role moved to %q, want it left in tenant-1", api.roles[0].TenantID)
			}
		})
	}
}

// State saved by a provider that failed to move the role names a tenant the role never
// reached. Read must find the role where it is, not plan a second one.
func TestRoleReadFindsRoleOutsideStateTenant(t *testing.T) {
	roles := []fronteggRole{}
	for i := range 150 {
		roles = append(roles, fronteggRole{ID: fmt.Sprintf("other-%d", i), Key: fmt.Sprintf("probe-%d", i), TenantID: "tenant-2"})
	}
	roles = append(roles, fronteggRole{ID: "role-1", Key: "probe", Name: "probe", TenantID: "tenant-1", Permissions: []string{}})
	api, srv := newFakeRoleAPI(t, roles...)
	holder := roleClientHolder(srv)

	for _, tc := range []struct {
		name, id, wantID, wantTenant string
	}{
		{name: "role in another tenant", id: "role-1", wantID: "role-1", wantTenant: "tenant-1"},
		{name: "role deleted", id: "role-gone", wantID: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := roleResourceData(t, map[string]interface{}{"key": "probe", "tenant_id": "tenant-2"})
			d.SetId(tc.id)

			if diags := resourceFronteggRoleRead(context.Background(), d, holder); diags.HasError() {
				t.Fatalf("read: %v\nrequests: %v", diags, api.requests)
			}
			if d.Id() != tc.wantID {
				t.Errorf("id = %q, want %q", d.Id(), tc.wantID)
			}
			if tc.wantID != "" && d.Get("tenant_id").(string) != tc.wantTenant {
				t.Errorf("tenant_id = %q, want %q, the tenant the role is really in", d.Get("tenant_id"), tc.wantTenant)
			}
		})
	}
}

func roleUnitConfig(tenantID string) string {
	line := ""
	if tenantID != "" {
		line = fmt.Sprintf("  tenant_id      = %q\n", tenantID)
	}
	return fmt.Sprintf(`
resource "frontegg_role" "r" {
  name           = "probe"
  key            = "probe"
  description    = "probe"
  default        = false
  level          = 1
  permission_ids = ["perm-1"]
%s}
`, line)
}

func TestRoleTenantChangeMovesRoleInPlace(t *testing.T) {
	api, srv := newFakeRoleAPI(t)
	t.Setenv("FRONTEGG_API_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")

	// Each change must move the role with the vendor-level PUT /tenant before the
	// tenant-scoped PATCH, which only finds the role in its new tenant.
	expectMoveBeforePatch := func(tenant string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			move := fmt.Sprintf("PUT %s/role-1/tenant tenant=", fronteggRolePath)
			patch := fmt.Sprintf("PATCH %s/role-1 tenant=%s", fronteggRolePath, tenant)
			for i, req := range api.requests {
				if req == move {
					for _, later := range api.requests[i+1:] {
						if later == patch {
							return nil
						}
					}
				}
			}
			return fmt.Errorf("requests %v lack %q followed by %q", api.requests, move, patch)
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: roleUnitConfig("tenant-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_role.r", "id", "role-1"),
					resource.TestCheckResourceAttr("frontegg_role.r", "tenant_id", "tenant-1"),
				),
			},
			{
				ResourceName:      "frontegg_role.r",
				ImportState:       true,
				ImportStateIdFunc: testAccTenantScopedImportID("frontegg_role.r"),
				ImportStateVerify: true,
			},
			{
				// Moved outside Terraform: refresh must find the role and plan a move back,
				// not a second role in tenant-1.
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.roles[0].TenantID = "tenant-2"
					api.requests = nil
				},
				Config: roleUnitConfig("tenant-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_role.r", "id", "role-1"),
					resource.TestCheckResourceAttr("frontegg_role.r", "tenant_id", "tenant-1"),
					expectMoveBeforePatch("tenant-1"),
					func(*terraform.State) error {
						api.mu.Lock()
						defer api.mu.Unlock()
						if len(api.roles) != 1 || api.roles[0].TenantID != "tenant-1" {
							return fmt.Errorf("roles = %+v, want role-1 alone, back in tenant-1", api.roles)
						}
						return nil
					},
				),
			},
			{
				Config: roleUnitConfig("tenant-2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_role.r", "id", "role-1"),
					resource.TestCheckResourceAttr("frontegg_role.r", "tenant_id", "tenant-2"),
					expectMoveBeforePatch("tenant-2"),
				),
			},
			{
				Config: roleUnitConfig(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_role.r", "id", "role-1"),
					resource.TestCheckResourceAttr("frontegg_role.r", "tenant_id", ""),
					expectMoveBeforePatch(""),
				),
			},
			{
				ResourceName:      "frontegg_role.r",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// tenant is the HCL expression for tenant_id.
func testAccRoleTenantConfig(suffix, tenant string) string {
	return fmt.Sprintf(`
resource "frontegg_tenant" "a" {
  key  = "tf-acc-role-tenant-a-%[1]s"
  name = "tf-acc role tenant a %[1]s"
}

resource "frontegg_tenant" "b" {
  key  = "tf-acc-role-tenant-b-%[1]s"
  name = "tf-acc role tenant b %[1]s"
}

resource "frontegg_role" "r" {
  key            = "tf-acc-tenant-role-%[1]s"
  name           = "tf-acc tenant role %[1]s"
  description    = "tenant role acceptance test"
  level          = 1
  default        = false
  permission_ids = []
  tenant_id      = %[2]s
}
`, suffix, tenant)
}

func TestAccFronteggRole_tenantScoped(t *testing.T) {
	suffix := acctest.RandString(8)
	var roleID string
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleTenantConfig(suffix, "frontegg_tenant.a.id"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("frontegg_role.r", "tenant_id", "frontegg_tenant.a", "id"),
					func(s *terraform.State) error {
						roleID = s.RootModule().Resources["frontegg_role.r"].Primary.ID
						return nil
					},
				),
			},
			{
				Config:   testAccRoleTenantConfig(suffix, "frontegg_tenant.a.id"),
				PlanOnly: true,
			},
			{
				ResourceName:      "frontegg_role.r",
				ImportState:       true,
				ImportStateIdFunc: testAccTenantScopedImportID("frontegg_role.r"),
				ImportStateVerify: true,
			},
			{
				Config: testAccRoleTenantConfig(suffix, "frontegg_tenant.b.id"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("frontegg_role.r", "tenant_id", "frontegg_tenant.b", "id"),
					resource.TestCheckResourceAttrPtr("frontegg_role.r", "id", &roleID),
				),
			},
			{
				Config:   testAccRoleTenantConfig(suffix, "frontegg_tenant.b.id"),
				PlanOnly: true,
			},
			{
				Config:      testAccRoleTenantConfig(suffix, fmt.Sprintf("%q", "tf-acc-missing-tenant-"+suffix)),
				ExpectError: regexp.MustCompile(`does not exist`),
			},
			{
				// The refused move kept tenant b in state and left the role there.
				Config:   testAccRoleTenantConfig(suffix, "frontegg_tenant.b.id"),
				PlanOnly: true,
			},
			{
				// Moved to tenant a outside Terraform: the apply moves it back, keeping its ID.
				PreConfig: func() { testAccMoveRole(t, roleID, "tf-acc-role-tenant-a-"+suffix) },
				Config:    testAccRoleTenantConfig(suffix, "frontegg_tenant.b.id"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("frontegg_role.r", "tenant_id", "frontegg_tenant.b", "id"),
					resource.TestCheckResourceAttrPtr("frontegg_role.r", "id", &roleID),
				),
			},
		},
	})
}

func testAccMoveRole(t *testing.T, roleID, tenantID string) {
	t.Helper()
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	client := restclient.MakeRestClient(base, "", os.Getenv("FRONTEGG_APPLICATION_ID"))
	client.Authenticate(fronteggVendorTokenNoT(base))
	if err := client.Put(context.Background(), fmt.Sprintf("%s/%s/tenant", fronteggRolePath, roleID), fronteggRoleTenant{TenantID: &tenantID}, nil); err != nil {
		t.Fatalf("moving role %s to tenant %s: %v", roleID, tenantID, err)
	}
}
