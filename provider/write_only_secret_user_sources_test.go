package provider

import (
	"net/http"
	"testing"
)

func fakeUserSourceAPI(t *testing.T, typePath string) *fakeAPI {
	t.Helper()
	return newFakeAPI(t,
		fakeAPIRoute{http.MethodPost, typePath, fakeCreate("id")},
		fakeAPIRoute{http.MethodPut, typePath + "/{id}", fakeUpdate},
		fakeAPIRoute{http.MethodGet, fronteggUserSourceBasePath + "/{id}", fakeGet},
		fakeAPIRoute{http.MethodDelete, fronteggUserSourceBasePath + "/{id}", fakeDelete},
	)
}

const userSourceTestFields = `
  name                 = "lifecycle"
  description          = "write-only lifecycle"
  index                = 1
  sync_on_login        = false
  is_migrated          = false
  tenant_resolver_type = "new"
`

func TestAuth0UserSourceWriteOnlySecretLifecycle(t *testing.T) {
	api := fakeUserSourceAPI(t, fronteggAuth0UserSourcePath)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_auth0_user_source.test",
		secrets: map[string][]string{"secret": {"configuration", "secret"}},
		config: func(arguments string) string {
			return `
resource "frontegg_auth0_user_source" "test" {` + userSourceTestFields + `
  domain    = "example.auth0.com"
  client_id = "auth0-client"
` + arguments + `}
`
		},
	})
}

func TestCognitoUserSourceWriteOnlySecretsLifecycle(t *testing.T) {
	api := fakeUserSourceAPI(t, fronteggCognitoUserSourcePath)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_cognito_user_source.test",
		secrets: map[string][]string{
			"secret_access_key": {"configuration", "secretAccessKey"},
			"client_secret":     {"configuration", "clientSecret"},
		},
		config: func(arguments string) string {
			return `
resource "frontegg_cognito_user_source" "test" {` + userSourceTestFields + `
  region        = "us-west-2"
  client_id     = "cognito-client"
  user_pool_id  = "us-west-2_abcdefghi"
  access_key_id = "AKIAIOSFODNN7EXAMPLE"
` + arguments + `}
`
		},
	})
}

func TestFirebaseUserSourceWriteOnlyPrivateKeyLifecycle(t *testing.T) {
	api := fakeUserSourceAPI(t, fronteggFirebaseUserSourcePath)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_firebase_user_source.test",
		secrets: map[string][]string{"private_key": {"configuration", "serviceAccount", "private_key"}},
		config: func(arguments string) string {
			return `
resource "frontegg_firebase_user_source" "test" {` + userSourceTestFields + `
  api_key                     = "firebase-api-key"
  service_account_type        = "service_account"
  project_id                  = "example-project"
  private_key_id              = "1234567890abcdef"
  client_email                = "sa@example-project.iam.gserviceaccount.com"
  client_id                   = "123456789012345678901"
  auth_uri                    = "https://accounts.google.com/o/oauth2/auth"
  token_uri                   = "https://oauth2.googleapis.com/token"
  auth_provider_x509_cert_url = "https://www.googleapis.com/oauth2/v1/certs"
  client_x509_cert_url        = "https://www.googleapis.com/robot/v1/metadata/x509/sa"
  universe_domain             = "googleapis.com"
` + arguments + `}
`
		},
	})
}

func TestFederationUserSourceWriteOnlySecretLifecycle(t *testing.T) {
	api := fakeUserSourceAPI(t, fronteggFederationUserSourcePath)
	testWriteOnlySecretLifecycle(t, api, writeOnlySecretCase{
		address: "frontegg_federation_user_source.test",
		secrets: map[string][]string{"secret": {"configuration", "secret"}},
		config: func(arguments string) string {
			return `
resource "frontegg_federation_user_source" "test" {
  name                 = "lifecycle"
  description          = "write-only lifecycle"
  sync_on_login        = false
  tenant_resolver_type = "new"
  client_id            = "federation-client"
  oauth2_config {
    authorization_url      = "https://idp.example.com/authorize"
    token_url              = "https://idp.example.com/token"
    user_info_url          = "https://idp.example.com/userinfo"
    scopes                 = ["openid"]
    grant_types            = ["authorization_code"]
    code_challenge_methods = ["S256"]
  }
` + arguments + `}
`
		},
	})
}
