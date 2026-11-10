package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fakeRedirectURIs struct {
	mu       sync.Mutex
	nextID   int
	uris     []fronteggOAuthRedirectURI
	posts    int
	failPost string
}

func (f *fakeRedirectURIs) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == fronteggOAuthRedirectURIsURL:
		_ = json.NewEncoder(w).Encode(fronteggOAuthRedirectURIs{RedirectURIs: f.uris})
	case r.Method == http.MethodPost && r.URL.Path == fronteggOAuthRedirectURIsURL:
		var in fronteggOAuthRedirectURI
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.posts++
		if in.RedirectURI == f.failPost {
			f.failPost = ""
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, u := range f.uris {
			if u.RedirectURI == in.RedirectURI {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{"Redirect uri already exists"}})
				return
			}
		}
		f.nextID++
		in.ID = fmt.Sprintf("uri-%d", f.nextID)
		f.uris = append(f.uris, in)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, fronteggOAuthRedirectURIsURL+"/"):
		id := strings.TrimPrefix(r.URL.Path, fronteggOAuthRedirectURIsURL+"/")
		kept := []fronteggOAuthRedirectURI{}
		for _, u := range f.uris {
			if u.ID != id {
				kept = append(kept, u)
			}
		}
		f.uris = kept
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeRedirectURIs) registered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for _, u := range f.uris {
		out = append(out, u.RedirectURI)
	}
	sort.Strings(out)
	return out
}

func newFakeRedirectURIs(t *testing.T, registered ...string) (*fakeRedirectURIs, *restclient.Client) {
	t.Helper()
	f := &fakeRedirectURIs{}
	for _, uri := range registered {
		f.nextID++
		f.uris = append(f.uris, fronteggOAuthRedirectURI{ID: fmt.Sprintf("uri-%d", f.nextID), RedirectURI: uri})
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	client := restclient.MakeRestClient(srv.URL, "", "")
	return f, &client
}

func redirectURLsResourceData(t *testing.T, urls ...string) *schema.ResourceData {
	t.Helper()
	list := []interface{}{}
	for _, url := range urls {
		list = append(list, url)
	}
	return schema.TestResourceDataRaw(t, resourceFronteggWorkspace().Schema, map[string]interface{}{
		"hosted_login": []interface{}{map[string]interface{}{"allowed_redirect_urls": list}},
	})
}

func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func TestUpdateAllowedRedirectURLsRegistersTrailingSlashVariantsOnce(t *testing.T) {
	f, client := newFakeRedirectURIs(t)
	d := redirectURLsResourceData(t, "https://localhost:4300", "https://localhost:4300/", "https://app.example.com/")

	if err := updateFronteggAllowedRedirectURLs(context.Background(), client, d); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	want := []string{"https://app.example.com", "https://localhost:4300"}
	if got := f.registered(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("registered = %v, want %v", got, want)
	}
}

func TestUpdateAllowedRedirectURLsKeepsExistingAndDeletesRemoved(t *testing.T) {
	f, client := newFakeRedirectURIs(t, "https://localhost:4300/", "https://old.example.com")
	d := redirectURLsResourceData(t, "https://localhost:4300", "https://new.example.com")

	if err := updateFronteggAllowedRedirectURLs(context.Background(), client, d); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	want := []string{"https://localhost:4300/", "https://new.example.com"}
	if got := f.registered(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("registered = %v, want %v", got, want)
	}
	if f.posts != 1 {
		t.Errorf("posts = %d, want 1", f.posts)
	}
}

func TestReadAllowedRedirectURLsKeepsConfiguredSpelling(t *testing.T) {
	_, client := newFakeRedirectURIs(t, "https://localhost:4300", "https://app.example.com")
	d := redirectURLsResourceData(t, "https://localhost:4300", "https://localhost:4300/", "https://app.example.com/")

	got, err := readFronteggAllowedRedirectURLs(context.Background(), client, d)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	want := []string{"https://app.example.com/", "https://localhost:4300", "https://localhost:4300/"}
	if fmt.Sprint(sortedStrings(got)) != fmt.Sprint(want) {
		t.Errorf("read = %v, want %v", sortedStrings(got), want)
	}
}

func TestReadAllowedRedirectURLsTrimsUnconfiguredURLs(t *testing.T) {
	_, client := newFakeRedirectURIs(t, "https://localhost:4300/", "https://outside.example.com/")
	d := redirectURLsResourceData(t, "https://localhost:4300/", "https://removed.example.com")

	got, err := readFronteggAllowedRedirectURLs(context.Background(), client, d)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	want := []string{"https://localhost:4300/", "https://outside.example.com"}
	if fmt.Sprint(sortedStrings(got)) != fmt.Sprint(want) {
		t.Errorf("read = %v, want %v", sortedStrings(got), want)
	}
}

func TestUpdateAllowedRedirectURLsFinishesAfterFailingMidway(t *testing.T) {
	f, client := newFakeRedirectURIs(t, "https://a.example.com", "https://old.example.com")
	urls := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com/", "https://d.example.com"}
	f.failPost = "https://c.example.com"

	if err := updateFronteggAllowedRedirectURLs(context.Background(), client, redirectURLsResourceData(t, urls...)); err == nil {
		t.Fatal("first update succeeded, want the failed POST to surface")
	}
	read, err := readFronteggAllowedRedirectURLs(context.Background(), client, redirectURLsResourceData(t, urls...))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got, want := fmt.Sprint(sortedStrings(read)), fmt.Sprint(sortedStrings(f.registered())); got != want {
		t.Errorf("read after failure = %s, want what is registered %s", got, want)
	}

	missing := 4 - len(f.registered())
	postsBefore := f.posts
	if err := updateFronteggAllowedRedirectURLs(context.Background(), client, redirectURLsResourceData(t, urls...)); err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	want := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com", "https://d.example.com"}
	if got := f.registered(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("registered = %v, want %v", got, want)
	}
	if got := f.posts - postsBefore; got != missing {
		t.Errorf("second update posted %d URLs, want only the %d still missing", got, missing)
	}
}

func TestUpdateAllowedRedirectURLsKeepsATrailingSlashInAPath(t *testing.T) {
	f, client := newFakeRedirectURIs(t)
	d := redirectURLsResourceData(t, "https://app.example.com/callback/", "https://app.example.com/callback", "https://app.example.com/")

	if err := updateFronteggAllowedRedirectURLs(context.Background(), client, d); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	want := []string{"https://app.example.com", "https://app.example.com/callback", "https://app.example.com/callback/"}
	if got := f.registered(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("registered = %v, want %v", got, want)
	}
}

func TestReadAllowedRedirectURLsTellsPathsWithAndWithoutTrailingSlashApart(t *testing.T) {
	_, client := newFakeRedirectURIs(t, "https://app.example.com/callback/")
	d := redirectURLsResourceData(t, "https://app.example.com/callback")

	got, err := readFronteggAllowedRedirectURLs(context.Background(), client, d)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	want := []string{"https://app.example.com/callback/"}
	if fmt.Sprint(sortedStrings(got)) != fmt.Sprint(want) {
		t.Errorf("read = %v, want %v, so the missing /callback shows as drift", sortedStrings(got), want)
	}
}

func TestNormalizeRedirectURLOnlyDropsABareOriginsSlash(t *testing.T) {
	cases := map[string]string{
		"https://localhost:4300/":           "https://localhost:4300",
		"https://localhost:4300":            "https://localhost:4300",
		"https://app.example.com/callback/": "https://app.example.com/callback/",
		"https://app.example.com/callback":  "https://app.example.com/callback",
		"https://app.example.com/?next=1":   "https://app.example.com/?next=1",
		"myapp://callback/":                 "myapp://callback/",
	}
	for in, want := range cases {
		if got := normalizeRedirectURL(in); got != want {
			t.Errorf("normalizeRedirectURL(%q) = %q, want %q", in, got, want)
		}
	}
}
