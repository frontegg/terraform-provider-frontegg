package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggEmailTemplateURL = "/identity/resources/mail/v1/configs/templates"

type fronteggEmailTemplateResource struct {
	Active             bool   `json:"active"`
	FromName           string `json:"fromName"`
	HTMLTemplate       string `json:"htmlTemplate"`
	RedirectURL        string `json:"redirectURL"`
	SuccessRedirectURL string `json:"successRedirectUrl,omitempty"`
	SenderEmail        string `json:"senderEmail"`
	Subject            string `json:"subject"`
	Type               string `json:"type"`

	// The API renders template variables (e.g. "{{APP_ID}}") in redirectURL
	// and successRedirectUrl in its responses, and returns the raw configured
	// values in these read-only pattern fields.
	RedirectURLPattern        string `json:"redirectURLPattern,omitempty"`
	SuccessRedirectURLPattern string `json:"successRedirectUrlPattern,omitempty"`
}

func resourceFronteggEmailTemplate() *schema.Resource {
	return &schema.Resource{
		Description: `Configures Frontegg email templates.

Each email template resource manages one specific email template type for the workspace.

**Note:** This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the email template will remain in its last-applied state.`,

		CreateContext: resourceFronteggEmailTemplateCreate,
		ReadContext:   resourceFronteggEmailTemplateRead,
		UpdateContext: resourceFronteggEmailTemplateUpdate,
		DeleteContext: resourceFronteggEmailTemplateDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"template_type": {
				Description: `The type of email template to configure.

Must be one of: "ResetPassword", "ActivateUser", "InviteToTenant", "PwnedPassword", "MagicLink", "OTC", "ConnectNewDevice", "UserUsedInvitation", "ResetPhoneNumber", "BulkInvitesToTenant", "MFAEnroll", "MFAUnenroll", "NewMFAMethod", "MFARecoveryCode", "RemoveMFAMethod", "EmailVerification", "BruteForceProtection", "SuspiciousIP", "MFAOTC", "ImpossibleTravel", "BotDetection", "SmsAuthenticationEnabled".`,
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"ResetPassword", "ActivateUser", "InviteToTenant", "PwnedPassword", "MagicLink", "OTC",
					"ConnectNewDevice", "UserUsedInvitation", "ResetPhoneNumber", "BulkInvitesToTenant",
					"MFAEnroll", "MFAUnenroll", "NewMFAMethod", "MFARecoveryCode", "RemoveMFAMethod",
					"EmailVerification", "BruteForceProtection", "SuspiciousIP", "MFAOTC",
					"ImpossibleTravel", "BotDetection", "SmsAuthenticationEnabled",
				}, false),
			},
			"active": {
				Description: "Whether the email template is active.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"from_address": {
				Description: `The address to use in the "From" header of the email.`,
				Type:        schema.TypeString,
				Required:    true,
			},
			"from_name": {
				Description: `The name to use in the "From" header of the email.`,
				Type:        schema.TypeString,
				Required:    true,
			},
			"subject": {
				Description: "The subject of the email.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"html_template": {
				Description: "The HTML template to use in the email.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"redirect_url": {
				Description: `The redirect URL to use, if applicable.

Access this value as "\{\{redirectURL\}\}" in the template.`,
				Type:     schema.TypeString,
				Optional: true,
			},
			"success_redirect_url": {
				Description: `The success redirect URL to use, if applicable.`,
				Type:        schema.TypeString,
				Optional:    true,
			},
		},
	}
}

func resourceFronteggEmailTemplateSerialize(d *schema.ResourceData) fronteggEmailTemplateResource {
	return fronteggEmailTemplateResource{
		Active:             d.Get("active").(bool),
		FromName:           d.Get("from_name").(string),
		SenderEmail:        d.Get("from_address").(string),
		Subject:            d.Get("subject").(string),
		HTMLTemplate:       d.Get("html_template").(string),
		RedirectURL:        d.Get("redirect_url").(string),
		SuccessRedirectURL: d.Get("success_redirect_url").(string),
		Type:               d.Get("template_type").(string),
	}
}

func resourceFronteggEmailTemplateDeserialize(d *schema.ResourceData, f fronteggEmailTemplateResource) error {
	d.SetId(f.Type)
	if err := d.Set("template_type", f.Type); err != nil {
		return err
	}
	if err := d.Set("active", f.Active); err != nil {
		return err
	}
	if err := d.Set("from_address", f.SenderEmail); err != nil {
		return err
	}
	if err := d.Set("from_name", f.FromName); err != nil {
		return err
	}
	if err := d.Set("subject", f.Subject); err != nil {
		return err
	}

	// Prefer the raw pattern fields: the API substitutes template variables
	// (e.g. "{{APP_ID}}") in redirectURL and successRedirectUrl, which would
	// produce a permanent diff for configurations that use such variables.
	redirectURL := f.RedirectURL
	if f.RedirectURLPattern != "" {
		redirectURL = f.RedirectURLPattern
	}
	if err := d.Set("redirect_url", redirectURL); err != nil {
		return err
	}
	successRedirectURL := f.SuccessRedirectURL
	if f.SuccessRedirectURLPattern != "" {
		successRedirectURL = f.SuccessRedirectURLPattern
	}
	if err := d.Set("success_redirect_url", successRedirectURL); err != nil {
		return err
	}
	return nil
}

func resourceFronteggEmailTemplateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggEmailTemplateUpdate(ctx, d, meta)
}

func resourceFronteggEmailTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggEmailTemplateResource
	if err := clientHolder.ApiClient.Get(ctx, fronteggEmailTemplateURL, &out); err != nil {
		return diag.FromErr(err)
	}

	// On import only the ID is set, and the ID is the template type.
	templateType := d.Get("template_type").(string)
	if templateType == "" {
		templateType = d.Id()
	}
	for _, template := range out {
		if template.Type == templateType {
			if err := resourceFronteggEmailTemplateDeserialize(d, template); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	// If template not found, it might be in default state
	d.SetId("")
	return nil
}

func resourceFronteggEmailTemplateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggEmailTemplateSerialize(d)

	if err := clientHolder.ApiClient.Post(ctx, fronteggEmailTemplateURL, in, nil); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(in.Type)
	return resourceFronteggEmailTemplateRead(ctx, d, meta)
}

func resourceFronteggEmailTemplateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Email templates cannot be deleted, only reset to defaults
	// We'll just remove from Terraform state
	return nil
}
