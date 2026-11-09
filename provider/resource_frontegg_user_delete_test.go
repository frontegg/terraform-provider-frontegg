package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func fakeUserCreateAPI(t *testing.T, conflicts int) (*restclient.ClientHolder, *int) {
	t.Helper()
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != fronteggUserPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		posts++
		w.Header().Set("Content-Type", "application/json")
		if posts <= conflicts {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"errors":["Email is already in use"],"errorCode":"ER-01200"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"user-1","email":"user@example.com"}`))
	}))
	t.Cleanup(srv.Close)
	client := restclient.MakeRestClient(srv.URL, "", "")
	client.Authenticate("test-token")
	previous := userCreateEmailRetryDelay
	userCreateEmailRetryDelay = time.Millisecond
	t.Cleanup(func() { userCreateEmailRetryDelay = previous })
	return &restclient.ClientHolder{ApiClient: client}, &posts
}

func userCreateData(t *testing.T) *schema.ResourceData {
	return schema.TestResourceDataRaw(t, resourceFronteggUser().Schema, map[string]interface{}{
		"tenant_id": "tenant-2",
		"email":     "user@example.com",
		"role_ids":  []interface{}{"role-1"},
	})
}

func TestUserCreateWaitsForAJustDeletedEmail(t *testing.T) {
	holder, posts := fakeUserCreateAPI(t, 2)
	d := userCreateData(t)

	if diags := resourceFronteggUserCreate(context.Background(), d, holder); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}

	if d.Id() != "user-1" || *posts != 3 {
		t.Errorf("id = %q after %d POSTs, want user-1 after 3", d.Id(), *posts)
	}
}

func TestUserCreateReportsAnEmailThatStaysInUse(t *testing.T) {
	holder, posts := fakeUserCreateAPI(t, 1000)

	diags := resourceFronteggUserCreate(context.Background(), userCreateData(t), holder)

	if !diags.HasError() || !strings.Contains(diags[0].Summary, "Email is already in use") {
		t.Fatalf("diags = %v, want the email-in-use error", diags)
	}
	if *posts != userCreateEmailRetries {
		t.Errorf("POSTs = %d, want %d", *posts, userCreateEmailRetries)
	}
}

func TestUserDeleteIgnoresAUserAlreadyGone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":["User not found: user-1"],"errorCode":"ER-01089"}`))
	}))
	defer srv.Close()
	client := restclient.MakeRestClient(srv.URL, "", "")
	client.Authenticate("test-token")
	d := schema.TestResourceDataRaw(t, resourceFronteggUser().Schema, map[string]interface{}{"tenant_id": "tenant-1"})
	d.SetId("user-1")

	if diags := resourceFronteggUserDelete(context.Background(), d, &restclient.ClientHolder{ApiClient: client}); diags.HasError() {
		t.Fatalf("delete of a user that is already gone: %v", diags)
	}
}

func TestUserCreateNamesTheExistingUserToImport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fronteggUserPath:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":["User already exists"],"errorCode":"ER-01086"}`))
		case r.Method == http.MethodGet && r.URL.Path == fronteggUserPathV1+"/email" && r.URL.Query().Get("email") == "user@example.com":
			_, _ = w.Write([]byte(`{"id":"user-9","email":"user@example.com"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := restclient.MakeRestClient(srv.URL, "", "")
	client.Authenticate("test-token")

	diags := resourceFronteggUserCreate(context.Background(), userCreateData(t), &restclient.ClientHolder{ApiClient: client})

	if !diags.HasError() {
		t.Fatal("create succeeded, want an error")
	}
	if got := diags[0].Summary; !strings.Contains(got, "already exists in tenant tenant-2") || !strings.Contains(got, "tenant-2:user-9") {
		t.Errorf("error = %q, want it to say the user exists in tenant-2 and give the import ID tenant-2:user-9", got)
	}
}
