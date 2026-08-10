package provider

import (
	"errors"
	"strings"
	"testing"
)

// Verbatim errors observed from the identity API when creating a user in an
// environment whose applications include no default.
var (
	errNoApplication = errors.New(
		`restclient: request failed: POST https://api.frontegg.com/identity/resources/users/v2: ` +
			`403 Forbidden: map[]: {"errors":["Application ID is not specified"],"errorCode":"ER-00008"}`,
	)
	errTenantNotAssigned = errors.New(
		`restclient: request failed: POST https://api.frontegg.com/identity/resources/users/v2: ` +
			`404 Not Found: map[]: {"errors":["This account/tenant is not assigned to the requested application"],"errorCode":"ER-01008"}`,
	)
)

func TestUserApplicationErrorNamesProviderSetting(t *testing.T) {
	got := fronteggUserApplicationError(errNoApplication)
	if got == nil {
		t.Fatal("got nil error, want a translated error")
	}
	for _, want := range []string{"application_id", "FRONTEGG_APPLICATION_ID"} {
		if !strings.Contains(got.Error(), want) {
			t.Errorf("error does not mention %q: %s", want, got)
		}
	}
	if !errors.Is(got, errNoApplication) {
		t.Error("translated error must wrap the original")
	}
}

func TestUserApplicationErrorNamesAssignmentResource(t *testing.T) {
	got := fronteggUserApplicationError(errTenantNotAssigned)
	if got == nil {
		t.Fatal("got nil error, want a translated error")
	}
	if !strings.Contains(got.Error(), "frontegg_application_tenant_assignment") {
		t.Errorf("error does not name the assignment resource: %s", got)
	}
	if !errors.Is(got, errTenantNotAssigned) {
		t.Error("translated error must wrap the original")
	}
}

func TestUserApplicationErrorPassesOthersThrough(t *testing.T) {
	other := errors.New(`restclient: request failed: 400 Bad Request: {"errors":["email must be an email"]}`)
	if got := fronteggUserApplicationError(other); got != other {
		t.Errorf("unrelated error was rewritten: %s", got)
	}
	if got := fronteggUserApplicationError(nil); got != nil {
		t.Errorf("nil error became %v", got)
	}
}
