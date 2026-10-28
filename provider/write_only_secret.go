package provider

import (
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const (
	writeOnlySuffix        = "_wo"
	writeOnlyVersionSuffix = "_wo_version"
)

// addWriteOnlySecret pairs the secret attribute `name` with a write-only
// `<name>_wo` argument and the `<name>_wo_version` trigger that sends it.
func addWriteOnlySecret(attrs map[string]*schema.Schema, name string) {
	writeOnlyName := name + writeOnlySuffix
	versionName := name + writeOnlyVersionSuffix

	secret := attrs[name]
	if secret.Required {
		secret.Required = false
		secret.Optional = true
		secret.ExactlyOneOf = []string{name, writeOnlyName}
		secret.Description += fmt.Sprintf(" Exactly one of `%s` or `%s` is required.", name, writeOnlyName)
	}
	secret.Description += fmt.Sprintf(" Stored in state; use `%s` to keep it out of state.", writeOnlyName)

	attrs[writeOnlyName] = &schema.Schema{
		Description: fmt.Sprintf(
			"Write-only alternative to `%s` that is never stored in the plan or state. Accepts ephemeral values. Requires Terraform 1.11 or later and must be set together with `%s`.",
			name, versionName,
		),
		Type:          schema.TypeString,
		Optional:      true,
		WriteOnly:     true,
		Sensitive:     true,
		ConflictsWith: []string{name},
		RequiredWith:  []string{versionName},
	}
	attrs[versionName] = &schema.Schema{
		Description: fmt.Sprintf(
			"Version of `%s`. Terraform cannot detect changes to a write-only value, so change this number whenever `%s` changes to send the new value.",
			writeOnlyName, writeOnlyName,
		),
		Type:         schema.TypeInt,
		Optional:     true,
		RequiredWith: []string{writeOnlyName},
		ValidateFunc: validation.IntAtLeast(1),
	}
}

// usesWriteOnlySecret reports whether the configuration supplies `name` through `<name>_wo`.
func usesWriteOnlySecret(d fronteggFieldGetter, name string) bool {
	return d.Get(name+writeOnlyVersionSuffix).(int) != 0
}

// secretValue returns the secret to send to the API, preferring the write-only argument.
func secretValue(d *schema.ResourceData, name string) string {
	writeOnly, diags := d.GetRawConfigAt(cty.GetAttrPath(name + writeOnlySuffix))
	if !diags.HasError() && writeOnly.Type().Equals(cty.String) && writeOnly.IsKnown() && !writeOnly.IsNull() {
		return writeOnly.AsString()
	}
	return d.Get(name).(string)
}

// setSecretUnlessWriteOnly refreshes `name` from the API unless the secret is
// supplied write-only, in which case the API value must stay out of state.
func setSecretUnlessWriteOnly(d *schema.ResourceData, name, value string) error {
	if usesWriteOnlySecret(d, name) {
		return nil
	}
	return d.Set(name, value)
}
