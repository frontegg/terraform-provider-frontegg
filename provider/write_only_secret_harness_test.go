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

type fakeDoc = map[string]interface{}

type fakeAPIRoute struct {
	method string
	path   string
	handle func(api *fakeAPI, id string, body fakeDoc) (int, interface{})
}

type fakeAPI struct {
	mu     sync.Mutex
	docs   map[string]fakeDoc
	order  []string
	nextID int
	writes []fakeDoc
	routes []fakeAPIRoute
}

func newFakeAPI(t *testing.T, routes ...fakeAPIRoute) *fakeAPI {
	t.Helper()
	api := &fakeAPI{docs: map[string]fakeDoc{}, routes: routes}
	srv := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(srv.Close)
	t.Setenv("FRONTEGG_API_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_PORTAL_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")
	return api
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.URL.Path == "/auth/vendor" {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
		return
	}
	var body fakeDoc
	_ = json.NewDecoder(r.Body).Decode(&body)
	for _, route := range a.routes {
		id, ok := matchFakeRoute(route.path, r.URL.Path)
		if !ok || route.method != r.Method {
			continue
		}
		if body != nil {
			a.writes = append(a.writes, body)
		}
		status, out := route.handle(a, id, body)
		w.WriteHeader(status)
		if out != nil {
			_ = json.NewEncoder(w).Encode(out)
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func matchFakeRoute(pattern, path string) (string, bool) {
	if !strings.HasSuffix(pattern, "/{id}") {
		return "", pattern == path
	}
	prefix := strings.TrimSuffix(pattern, "{id}")
	id := strings.TrimPrefix(path, prefix)
	if !strings.HasPrefix(path, prefix) || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (a *fakeAPI) put(id string, body fakeDoc) fakeDoc {
	doc, ok := a.docs[id]
	if !ok {
		doc = fakeDoc{}
		a.order = append(a.order, id)
	}
	for k, v := range body {
		doc[k] = v
	}
	a.docs[id] = doc
	return doc
}

func (a *fakeAPI) list() []fakeDoc {
	out := []fakeDoc{}
	for _, id := range a.order {
		if doc, ok := a.docs[id]; ok {
			out = append(out, doc)
		}
	}
	return out
}

func (a *fakeAPI) remove(id string) {
	delete(a.docs, id)
}

func fakeCreate(idField string) func(*fakeAPI, string, fakeDoc) (int, interface{}) {
	return func(a *fakeAPI, _ string, body fakeDoc) (int, interface{}) {
		a.nextID++
		id := fmt.Sprintf("id-%d", a.nextID)
		body[idField] = id
		return http.StatusOK, a.put(id, body)
	}
}

func fakeUpdate(a *fakeAPI, id string, body fakeDoc) (int, interface{}) {
	if _, ok := a.docs[id]; !ok {
		return http.StatusNotFound, nil
	}
	return http.StatusOK, a.put(id, body)
}

func fakeGet(a *fakeAPI, id string, _ fakeDoc) (int, interface{}) {
	doc, ok := a.docs[id]
	if !ok {
		return http.StatusNotFound, nil
	}
	return http.StatusOK, doc
}

func fakeList(a *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
	return http.StatusOK, a.list()
}

func fakeDelete(a *fakeAPI, id string, _ fakeDoc) (int, interface{}) {
	a.remove(id)
	return http.StatusOK, nil
}

func (a *fakeAPI) lastSent(path ...string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := len(a.writes) - 1; i >= 0; i-- {
		var v interface{} = a.writes[i]
		for _, key := range path {
			m, ok := v.(map[string]interface{})
			if !ok {
				v = nil
				break
			}
			v = m[key]
		}
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

type writeOnlySecretCase struct {
	address  string
	secrets  map[string][]string
	config   func(secretArguments string) string
	importID func(*terraform.State) (string, error)
}

func (c writeOnlySecretCase) arguments(value func(attr string) string, version int) string {
	var b strings.Builder
	for attr := range c.secrets {
		if version == 0 {
			fmt.Fprintf(&b, "  %s = %q\n", attr, value(attr))
			continue
		}
		fmt.Fprintf(&b, "  %s%s = %s\n  %s%s = %d\n", attr, writeOnlySuffix, value(attr), attr, writeOnlyVersionSuffix, version)
	}
	return b.String()
}

func (c writeOnlySecretCase) render(arguments string) string {
	return `
variable "secret" {
  type      = string
  ephemeral = true
  default   = "ephemeral"
}
` + c.config(arguments)
}

func (c writeOnlySecretCase) checkSent(api *fakeAPI, value func(attr string) string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		for attr, path := range c.secrets {
			want := value(attr)
			if got := api.lastSent(path...); got != want {
				return fmt.Errorf("API received %s %q, want %q", attr, got, want)
			}
		}
		return nil
	}
}

func (c writeOnlySecretCase) values(prefix string) func(string) string {
	return func(attr string) string { return prefix + "-" + attr }
}

func (c writeOnlySecretCase) all(prefix string) []string {
	out := []string{}
	for attr := range c.secrets {
		out = append(out, c.values(prefix)(attr))
	}
	return out
}

func testWriteOnlySecretLifecycle(t *testing.T, api *fakeAPI, c writeOnlySecretCase) {
	t.Helper()
	skipBelowWriteOnlyTerraform(t)

	plain := c.values("plain")
	ephemeral := func(attr string) string { return fmt.Sprintf(`"${var.secret}-%s"`, attr) }
	rotated := c.values("rotated")
	quoted := func(f func(string) string) func(string) string {
		return func(attr string) string { return fmt.Sprintf("%q", f(attr)) }
	}

	emptySecrets := []resource.TestCheckFunc{}
	for attr := range c.secrets {
		emptySecrets = append(emptySecrets, resource.TestCheckResourceAttr(c.address, attr, ""))
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c.render(c.arguments(plain, 0)),
				Check:  c.checkSent(api, plain),
			},
			{
				Config: c.render(c.arguments(ephemeral, 1)),
				Check: resource.ComposeAggregateTestCheckFunc(append(emptySecrets,
					c.checkSent(api, c.values("ephemeral")),
					checkSecretOutOfState(append(c.all("plain"), c.all("ephemeral")...)...),
				)...),
			},
			{
				Config:   c.render(c.arguments(ephemeral, 1)),
				PlanOnly: true,
			},
			{
				Config:   c.render(c.arguments(quoted(rotated), 1)),
				PlanOnly: true,
			},
			{
				Config: c.render(c.arguments(quoted(rotated), 2)),
				Check: resource.ComposeAggregateTestCheckFunc(
					c.checkSent(api, rotated),
					checkSecretOutOfState(c.all("rotated")...),
				),
			},
			{
				ResourceName:      c.address,
				ImportState:       true,
				ImportStateIdFunc: c.importID,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					secrets := c.all("rotated")
					for _, state := range states {
						for key, value := range state.Attributes {
							for _, secret := range secrets {
								if value == secret {
									return fmt.Errorf("imported state attribute %s holds the secret", key)
								}
							}
						}
					}
					return nil
				},
			},
		},
	})
}
