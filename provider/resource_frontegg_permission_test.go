package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func permissionResourceData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceFronteggPermission().Schema, raw)
}

func TestPermissionSerializeSendsAssignmentType(t *testing.T) {
	d := permissionResourceData(t, map[string]interface{}{
		"name":            "probe",
		"key":             "probe",
		"category_id":     "cat-1",
		"description":     "probe",
		"assignment_type": "NEVER",
	})

	if got := resourceFronteggPermissionSerialize(d).AssignmentType; got != "NEVER" {
		t.Errorf("assignmentType = %q, want NEVER", got)
	}
}

func TestPermissionSerializeOmitsUnsetAssignmentType(t *testing.T) {
	d := permissionResourceData(t, map[string]interface{}{
		"name":        "probe",
		"key":         "probe",
		"category_id": "cat-1",
		"description": "probe",
	})

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

	if err := resourceFronteggPermissionDeserialize(d, fronteggPermission{
		ID:             "perm-1",
		Name:           "probe",
		Key:            "probe",
		CategoryID:     "cat-1",
		Description:    "probe",
		AssignmentType: "ALWAYS",
	}); err != nil {
		t.Fatal(err)
	}
	if got := d.Get("assignment_type").(string); got != "ALWAYS" {
		t.Errorf("assignment_type = %q, want ALWAYS", got)
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

func TestPermissionCreateReadsAssignmentTypeBack(t *testing.T) {
	created := fronteggPermission{ID: "perm-1", Name: "probe", Key: "probe", CategoryID: "cat-1", Description: "probe"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fronteggPermissionPath:
			_ = json.NewEncoder(w).Encode([]fronteggPermission{created})
		case r.Method == http.MethodGet && r.URL.Path == fronteggPermissionPath:
			withType := created
			withType.AssignmentType = "NEVER"
			_ = json.NewEncoder(w).Encode([]fronteggPermission{withType})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(srv.URL, "", "")}
	holder.ApiClient.Authenticate("test-token")
	d := permissionResourceData(t, map[string]interface{}{
		"name":            "probe",
		"key":             "probe",
		"category_id":     "cat-1",
		"description":     "probe",
		"assignment_type": "NEVER",
	})

	if diags := resourceFronteggPermissionCreate(context.Background(), d, holder); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if got := d.Get("assignment_type").(string); got != "NEVER" {
		t.Errorf("assignment_type = %q, want NEVER; the create response omits it, so it has to be read back", got)
	}
}

func testAccPermissionConfig(assignmentType string) string {
	return fmt.Sprintf(`
resource "frontegg_permission_category" "p" {
  name        = "tf-acc-permission-category"
  description = "category for the permission acceptance test"
}

resource "frontegg_permission" "p" {
  name            = "tf-acc permission"
  key             = "tf-acc-permission"
  description     = "permission acceptance test"
  category_id     = frontegg_permission_category.p.id
  assignment_type = %q
}

data "frontegg_permission" "p" {
  key = frontegg_permission.p.key
}
`, assignmentType)
}

func TestAccFronteggPermission_assignmentType(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPermissionConfig("NEVER"),
				Check:  resource.TestCheckResourceAttr("frontegg_permission.p", "assignment_type", "NEVER"),
			},
			{
				Config:   testAccPermissionConfig("NEVER"),
				PlanOnly: true,
			},
			{
				Config: testAccPermissionConfig("ALWAYS"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_permission.p", "assignment_type", "ALWAYS"),
					resource.TestCheckResourceAttr("data.frontegg_permission.p", "assignment_type", "ALWAYS"),
				),
			},
			{
				Config:            testAccPermissionConfig("ALWAYS"),
				ResourceName:      "frontegg_permission.p",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFronteggPermission_defaultsToAssignable(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "frontegg_permission_category" "d" {
  name        = "tf-acc-permission-category-default"
  description = "category for the permission default test"
}

resource "frontegg_permission" "d" {
  name        = "tf-acc permission default"
  key         = "tf-acc-permission-default"
  description = "permission default test"
  category_id = frontegg_permission_category.d.id
}
`,
				Check: resource.TestCheckResourceAttr("frontegg_permission.d", "assignment_type", "ASSIGNABLE"),
			},
		},
	})
}

func TestPermissionDataSourceExposesAssignmentTypeReadOnly(t *testing.T) {
	field := dataSourceFronteggPermission().Schema["assignment_type"]
	if field == nil {
		t.Fatal("data source has no assignment_type")
	}
	if field.Optional || field.Required || !field.Computed {
		t.Errorf("assignment_type optional=%v required=%v computed=%v, want read-only", field.Optional, field.Required, field.Computed)
	}
}
