package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func permissionResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggPermission().Schema, raw)
}

func permissionConfig(assignmentType string) map[string]interface{} {
	raw := map[string]interface{}{
		"name":        "probe",
		"key":         "probe",
		"category_id": "cat-1",
		"description": "probe",
	}
	if assignmentType != "" {
		raw["assignment_type"] = assignmentType
	}
	return raw
}

func TestPermissionSerializeSendsAssignmentType(t *testing.T) {
	d := permissionResourceData(t, permissionConfig("NEVER"))

	if got := resourceFronteggPermissionSerialize(d).AssignmentType; got != "NEVER" {
		t.Errorf("assignmentType = %q, want NEVER", got)
	}
}

func TestPermissionSerializeOmitsUnsetAssignmentType(t *testing.T) {
	d := permissionResourceData(t, permissionConfig(""))

	body, err := json.Marshal(resourceFronteggPermissionSerialize(d))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); strings.Contains(got, "assignmentType") {
		t.Errorf("payload = %s, want no assignmentType so the API keeps the current classification", got)
	}
}

func TestPermissionDeserializeSetsAssignmentType(t *testing.T) {
	d := permissionResourceData(t, map[string]interface{}{})

	if err := resourceFronteggPermissionDeserialize(d, fronteggPermission{ID: "perm-1", AssignmentType: "ALWAYS"}); err != nil {
		t.Fatal(err)
	}
	if got := d.Get("assignment_type").(string); got != "ALWAYS" {
		t.Errorf("assignment_type = %q, want ALWAYS", got)
	}
}

func TestPermissionDeserializeOverwritesAssignmentTypeWhenEmpty(t *testing.T) {
	d := permissionResourceData(t, permissionConfig("NEVER"))

	if err := resourceFronteggPermissionDeserialize(d, fronteggPermission{ID: "perm-1"}); err != nil {
		t.Fatal(err)
	}
	if got := d.Get("assignment_type").(string); got != "" {
		t.Errorf("assignment_type = %q, want the API's empty value so drift shows up in plans", got)
	}
}

func TestPermissionAssignmentTypeValidation(t *testing.T) {
	validate := resourceFronteggPermission().Schema["assignment_type"].ValidateFunc
	for _, name := range []string{"NEVER", "ALWAYS", "ASSIGNABLE"} {
		if _, errs := validate(name, "assignment_type"); len(errs) > 0 {
			t.Errorf("assignment_type %q rejected: %v", name, errs)
		}
	}
	if _, errs := validate("assignable", "assignment_type"); len(errs) == 0 {
		t.Error("assignment_type \"assignable\" accepted, want the API's uppercase values only")
	}
}

func TestPermissionDataSourceExposesAssignmentTypeReadOnly(t *testing.T) {
	field := dataSourceFronteggPermission().Schema["assignment_type"]
	if field == nil {
		t.Fatal("data source has no assignment_type")
	}
	if field.Optional || field.Required || !field.Computed {
		t.Errorf("assignment_type optional=%v required=%v computed=%v, want read-only", field.Optional, field.Required, field.Computed)
	}
	for _, inputGuidance := range []string{"Must be one of", "Removing the attribute"} {
		if strings.Contains(field.Description, inputGuidance) {
			t.Errorf("data source description contains %q, which only applies to the resource", inputGuidance)
		}
	}
}

type fakePermissionAPI struct {
	mu          sync.Mutex
	permissions map[string]fronteggPermission
	listed      bool
	lists       int
	patches     []map[string]interface{}
	nextID      int
}

func newFakePermissionAPI(t *testing.T, listNewPermissions bool) (*fakePermissionAPI, *httptest.Server) {
	t.Helper()
	api := &fakePermissionAPI{permissions: map[string]fronteggPermission{}, listed: listNewPermissions}
	srv := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(srv.Close)
	return api, srv
}

func (api *fakePermissionAPI) serve(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/auth/vendor":
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
	case r.Method == http.MethodPost && r.URL.Path == fronteggPermissionPath:
		var in []fronteggPermission
		_ = json.NewDecoder(r.Body).Decode(&in)
		api.nextID++
		created := in[0]
		created.ID = fmt.Sprintf("perm-%d", api.nextID)
		stored := created
		if stored.AssignmentType == "" {
			stored.AssignmentType = "ASSIGNABLE"
		}
		api.permissions[created.ID] = stored
		created.AssignmentType = ""
		_ = json.NewEncoder(w).Encode([]fronteggPermission{created})
	case r.Method == http.MethodGet && r.URL.Path == fronteggPermissionPath:
		api.lists++
		out := []fronteggPermission{}
		if api.listed {
			for _, p := range api.permissions {
				out = append(out, p)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, fronteggPermissionPath+"/"):
		id := strings.TrimPrefix(r.URL.Path, fronteggPermissionPath+"/")
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		api.patches = append(api.patches, body)
		p := api.permissions[id]
		if v, ok := body["description"].(string); ok {
			p.Description = v
		}
		if v, ok := body["assignmentType"].(string); ok {
			p.AssignmentType = v
		}
		api.permissions[id] = p
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, fronteggPermissionPath+"/"):
		delete(api.permissions, strings.TrimPrefix(r.URL.Path, fronteggPermissionPath+"/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func createPermissionAgainst(t *testing.T, srv *httptest.Server, raw map[string]interface{}) (*schema.ResourceData, diag.Diagnostics) {
	t.Helper()
	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(srv.URL, "", "")}
	holder.ApiClient.Authenticate("test-token")
	d := permissionResourceData(t, raw)
	return d, resourceFronteggPermissionCreate(context.Background(), d, holder)
}

func TestPermissionCreateReadsAssignmentTypeBackWhenUnset(t *testing.T) {
	api, srv := newFakePermissionAPI(t, true)
	d, diags := createPermissionAgainst(t, srv, permissionConfig(""))

	if diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if got := d.Get("assignment_type").(string); got != "ASSIGNABLE" {
		t.Errorf("assignment_type = %q, want ASSIGNABLE read back from the list, because the create response omits it", got)
	}
	if api.lists != 1 {
		t.Errorf("list requests = %d, want 1", api.lists)
	}
}

func TestPermissionCreateSkipsListWhenAssignmentTypeConfigured(t *testing.T) {
	api, srv := newFakePermissionAPI(t, true)
	d, diags := createPermissionAgainst(t, srv, permissionConfig("NEVER"))

	if diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if got := d.Get("assignment_type").(string); got != "NEVER" {
		t.Errorf("assignment_type = %q, want NEVER", got)
	}
	if api.lists != 0 {
		t.Errorf("list requests = %d, want 0 because the configured value is already known", api.lists)
	}
}

func TestPermissionCreateKeepsResourceWhenNotListedYet(t *testing.T) {
	_, srv := newFakePermissionAPI(t, false)
	d, diags := createPermissionAgainst(t, srv, permissionConfig(""))

	if diags.HasError() {
		t.Fatalf("create returned an error, want the created permission kept: %v", diags)
	}
	if d.Id() != "perm-1" {
		t.Errorf("id = %q, want perm-1 so the created permission stays in state", d.Id())
	}
	if len(diags) != 1 || diags[0].Severity != diag.Warning {
		t.Errorf("diagnostics = %v, want one warning about the unread assignment type", diags)
	}
}

func permissionUnitConfig(description, assignmentType string) string {
	line := ""
	if assignmentType != "" {
		line = fmt.Sprintf("  assignment_type = %q\n", assignmentType)
	}
	return fmt.Sprintf(`
resource "frontegg_permission" "p" {
  name        = "probe"
  key         = "probe"
  category_id = "cat-1"
  description = %q
%s}
`, description, line)
}

func TestPermissionUpdateSendsAssignmentTypeOnlyWhenChanged(t *testing.T) {
	api, srv := newFakePermissionAPI(t, true)
	t.Setenv("FRONTEGG_API_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: permissionUnitConfig("first", "")},
			{
				Config: permissionUnitConfig("second", ""),
				Check: func(*terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					last := api.patches[len(api.patches)-1]
					if _, sent := last["assignmentType"]; sent {
						return fmt.Errorf("PATCH body %v resends assignmentType although the plan did not change it", last)
					}
					return nil
				},
			},
			{
				Config: permissionUnitConfig("second", "NEVER"),
				Check: func(*terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					last := api.patches[len(api.patches)-1]
					if got := last["assignmentType"]; got != "NEVER" {
						return fmt.Errorf("PATCH body %v, want assignmentType NEVER", last)
					}
					return nil
				},
			},
		},
	})
}

func testAccPermissionConfig(suffix, assignmentType string) string {
	return fmt.Sprintf(`
resource "frontegg_permission_category" "p" {
  name        = "tf-acc-permission-category-%[1]s"
  description = "category for the permission acceptance test"
}

resource "frontegg_permission" "p" {
  name            = "tf-acc permission %[1]s"
  key             = "tf-acc-permission-%[1]s"
  description     = "permission acceptance test"
  category_id     = frontegg_permission_category.p.id
  assignment_type = %[2]q
}

data "frontegg_permission" "p" {
  key = frontegg_permission.p.key
}
`, suffix, assignmentType)
}

func TestAccFronteggPermission_assignmentType(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPermissionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPermissionConfig(suffix, "NEVER"),
				Check:  resource.TestCheckResourceAttr("frontegg_permission.p", "assignment_type", "NEVER"),
			},
			{
				Config:   testAccPermissionConfig(suffix, "NEVER"),
				PlanOnly: true,
			},
			{
				Config: testAccPermissionConfig(suffix, "ALWAYS"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_permission.p", "assignment_type", "ALWAYS"),
					resource.TestCheckResourceAttr("data.frontegg_permission.p", "assignment_type", "ALWAYS"),
				),
			},
			{
				Config:            testAccPermissionConfig(suffix, "ALWAYS"),
				ResourceName:      "frontegg_permission.p",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFronteggPermission_defaultsToAssignable(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPermissionDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "frontegg_permission_category" "d" {
  name        = "tf-acc-permission-category-default-%[1]s"
  description = "category for the permission default test"
}

resource "frontegg_permission" "d" {
  name        = "tf-acc permission default %[1]s"
  key         = "tf-acc-permission-default-%[1]s"
  description = "permission default test"
  category_id = frontegg_permission_category.d.id
}
`, suffix),
				Check: resource.TestCheckResourceAttr("frontegg_permission.d", "assignment_type", "ASSIGNABLE"),
			},
		},
	})
}

func testAccCheckPermissionDestroy(s *terraform.State) error {
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	var ids []string
	for _, rs := range s.RootModule().Resources {
		if rs.Type == "frontegg_permission" {
			ids = append(ids, rs.Primary.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, base+fronteggPermissionPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+fronteggVendorTokenNoT(base))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out []fronteggPermission
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	for _, p := range out {
		for _, id := range ids {
			if p.ID == id {
				return fmt.Errorf("permission %s still exists after destroy", id)
			}
		}
	}
	return nil
}
