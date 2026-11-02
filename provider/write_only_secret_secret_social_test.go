package provider

import (
	"net/http"
	"testing"
)

func TestSecretWriteOnlyValueLifecycle(t *testing.T) {
	api := newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, fronteggSecretPath, fakeCreate("id")},
		fakeAPIRoute{http.MethodGet, fronteggSecretPath, func(a *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
			items := []fakeDoc{}
			for _, doc := range a.list() {
				items = append(items, fakeDoc{"id": doc["id"], "key": doc["key"]})
			}
			return http.StatusOK, fakeDoc{"items": items}
		}},
		fakeAPIRoute{http.MethodPatch, fronteggSecretPath + "/{id}", fakeUpdate},
		fakeAPIRoute{http.MethodDelete, fronteggSecretPath + "/{id}", fakeDelete},
	)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_secret.test",
		secrets: map[string][]string{"value": {"value"}},
		config: func(arguments string) string {
			return `
resource "frontegg_secret" "test" {
  key = "LIFECYCLE"
` + arguments + `}
`
		},
	})
}

func TestSocialLoginWriteOnlySecretLifecycle(t *testing.T) {
	upsert := func(a *fakeAPI, _ string, body fakeDoc) (int, interface{}) {
		a.put(body["type"].(string), body)
		return http.StatusOK, nil
	}
	setActive := func(active bool) func(*fakeAPI, string, fakeDoc) (int, interface{}) {
		return func(a *fakeAPI, _ string, _ fakeDoc) (int, interface{}) {
			a.put("google", fakeDoc{"active": active})
			return http.StatusOK, nil
		}
	}
	api := newFakeAPI(t,
		fakeAPIRoute{http.MethodGet, fronteggSSOURL + "/{id}", fakeGet},
		fakeAPIRoute{http.MethodPost, fronteggSSOURL, upsert},
		fakeAPIRoute{http.MethodPost, fronteggSSOURL + "/google/activate", setActive(true)},
		fakeAPIRoute{http.MethodPost, fronteggSSOURL + "/google/deactivate", setActive(false)},
	)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_social_login.test",
		secrets: map[string][]string{"secret": {"secret"}},
		config: func(arguments string) string {
			return `
resource "frontegg_social_login" "test" {
  provider_name = "google"
  client_id     = "google-client"
  redirect_url  = "https://example.com/oauth/callback"
  customised    = true
` + arguments + `}
`
		},
	})
}
