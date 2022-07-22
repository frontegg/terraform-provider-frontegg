package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
