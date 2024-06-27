package validators

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	Mailgun  = "mailgun"
	SES      = "ses"
	Sendgrid = "sendgrid"
	SES_ROLE = "ses-role"
)

func ValidateProvider(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	ps := []string{Mailgun, SES, Sendgrid, SES_ROLE}

	contain := slices.Contains(ps, v)

	if !contain {
		errs = append(errs, fmt.Errorf("%q must be either %v ,got: %s", key, strings.Join(ps, ", "), v))
	}
	return
}

func ValidateRequiredFields(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {

	provider := rd.Get("provider_name").(string)

	requiredFieldError := func(field, provider string) error {
		return fmt.Errorf("%s is required when provider is '%s'", field, provider)
	}

	if provider == SES {
		if region, ok := rd.GetOk("region"); !ok || region.(string) == "" {
			return requiredFieldError("region", SES)
		}

		if id, ok := rd.GetOk("provider_id"); !ok || id.(string) == "" {
			return requiredFieldError("provider_id", SES)
		}
	}

	if provider == Mailgun {
		if domain, ok := rd.GetOk("domain"); !ok || domain.(string) == "" {
			return requiredFieldError("domain", Mailgun)
		}
		if region, ok := rd.GetOk("region"); !ok || region.(string) == "" {
			return requiredFieldError("region", Mailgun)
		}
	}

	return nil
}
