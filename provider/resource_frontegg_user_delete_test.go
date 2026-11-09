package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestUserDeleteIsScopedToItsTenant(t *testing.T) {
	var gotMethod, gotPath, gotTenant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotTenant = r.Method, r.URL.Path, r.Header.Get("frontegg-tenant-id")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := restclient.MakeRestClient(srv.URL, "", "")
	client.Authenticate("test-token")
	d := schema.TestResourceDataRaw(t, resourceFronteggUser().Schema, map[string]interface{}{"tenant_id": "tenant-1"})
	d.SetId("user-1")

	if diags := resourceFronteggUserDelete(context.Background(), d, &restclient.ClientHolder{ApiClient: client}); diags.HasError() {
		t.Fatalf("delete: %v", diags)
	}

	if gotMethod != http.MethodDelete || gotPath != fronteggUserPathV1+"/user-1" {
		t.Errorf("request = %s %s, want DELETE %s/user-1", gotMethod, gotPath, fronteggUserPathV1)
	}
	if gotTenant != "tenant-1" {
		t.Errorf("frontegg-tenant-id = %q, want %q, so only the user's membership in its tenant is removed", gotTenant, "tenant-1")
	}
}
