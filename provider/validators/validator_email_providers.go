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
	return warns, errs
}

func ValidateRequiredFields(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {

	if !rd.NewValueKnown("provider_name") {
		return nil
	}
	provider := rd.Get("provider_name").(string)

	requiredFields := map[string][]string{
		SES:     {"region", "provider_id"},
		Mailgun: {"domain", "region"},
	}
	for _, field := range requiredFields[provider] {
		if rd.NewValueKnown(field) && rd.Get(field).(string) == "" {
			return fmt.Errorf("%s is required when provider is '%s'", field, provider)
		}
	}
	return nil
}
