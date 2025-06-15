package provider

import (
	"context"
	"log"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggSSODomainPolicy() *schema.Resource {
	return &schema.Resource{
		Description: `Configures how SSO domains are validated.

This is a singleton resource. You must only create one frontegg_sso_domain_policy resource
per Frontegg provider.`,

		CreateContext: resourceFronteggSSODomainPolicyCreate,
		ReadContext:   resourceFronteggSSODomainPolicyRead,
		UpdateContext: resourceFronteggSSODomainPolicyUpdate,
		DeleteContext: resourceFronteggSSODomainPolicyDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"allow_verified_users_to_add_domains": {
				Description: "Whether to allow users to add their own email domain without validating the domain through DNS.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"skip_domain_verification": {
				Description: "Whether to automatically mark new SSO domains as validated, without validating the domain through DNS.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"bypass_domain_cross_validation": {
				Description: "Whether to allow users to sign in even via SSO even if the associated domain has not been validated through DNS.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
		},
	}
}

func resourceFronteggSSODomainPolicySerialize(d *schema.ResourceData) fronteggSSODomain {
	return fronteggSSODomain{
		AllowVerifiedUsersToAddDomains: d.Get("allow_verified_users_to_add_domains").(bool),
		SkipDomainVerification:         d.Get("skip_domain_verification").(bool),
		BypassDomainCrossValidation:    d.Get("bypass_domain_cross_validation").(bool),
	}
}

func resourceFronteggSSODomainPolicyDeserialize(d *schema.ResourceData, f fronteggSSODomain) error {
	// Use a fixed ID since this is a singleton resource
	d.SetId("sso_domain_policy")

	if err := d.Set("allow_verified_users_to_add_domains", f.AllowVerifiedUsersToAddDomains); err != nil {
		return err
	}
	if err := d.Set("skip_domain_verification", f.SkipDomainVerification); err != nil {
		return err
	}
	if err := d.Set("bypass_domain_cross_validation", f.BypassDomainCrossValidation); err != nil {
		return err
	}
	return nil
}

func resourceFronteggSSODomainPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggSSODomainPolicyUpdate(ctx, d, meta)
}

func resourceFronteggSSODomainPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggSSODomain
	clientHolder.ApiClient.Ignore404()
	if err := clientHolder.ApiClient.Get(ctx, fronteggSSODomainURL, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggSSODomainPolicyDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggSSODomainPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggSSODomainPolicySerialize(d)
	if err := clientHolder.ApiClient.Put(ctx, fronteggSSODomainURL, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggSSODomainPolicyRead(ctx, d, meta)
}

func resourceFronteggSSODomainPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[WARN] Cannot destroy SSO domain policy. Terraform will remove this resource from the " +
		"state file, but the SSO domain policy will remain in its last-applied state.")
	return nil
}
