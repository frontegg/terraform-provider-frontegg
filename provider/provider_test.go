package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProviderFactories = map[string]func() (*schema.Provider, error){
	"frontegg": func() (*schema.Provider, error) {
		return New("test")(), nil
	},
}

func testAccPreCheck(t *testing.T) {
	for _, key := range []string{"FRONTEGG_CLIENT_ID", "FRONTEGG_SECRET_KEY"} {
		if os.Getenv(key) == "" {
			t.Skipf("%s must be set for acceptance tests", key)
		}
	}
}

func TestValidateProviderSchema(t *testing.T) {
	scm := schema.InternalMap(New("0.0.0")().Schema)
	if err := scm.InternalValidate(nil); err != nil {
		t.Errorf("Schema failed to validate: %v", err)
	}
}

func TestValidateResourceSchemas(t *testing.T) {
	prov := New("0.0.0")()
	for name, res := range prov.ResourcesMap {
		test := res
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scm := schema.InternalMap(test.Schema)
			if err := scm.InternalValidate(nil); err != nil {
				t.Errorf("Schema failed to validate: %v", err)
			}
		})
	}
}
