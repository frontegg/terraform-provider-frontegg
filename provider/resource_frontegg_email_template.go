package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

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

// fronteggEmailTemplateRequest is the POST body. HTMLTemplate is nil when the HTML is not
// written: without htmlTemplate Frontegg keeps the stored HTML, while "" clears it.
type fronteggEmailTemplateRequest struct {
	fronteggEmailTemplateResource
	HTMLTemplate *string `json:"htmlTemplate,omitempty"`
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
		CustomizeDiff: resourceFronteggEmailTemplateCustomizeDiff,

		Schema: map[string]*schema.Schema{
			"template_type": {
				Description: `The type of email template to configure.

Must be one of: "ResetPassword", "ActivateUser", "InviteToTenant", "PwnedPassword", "MagicLink", "OTC", "ConnectNewDevice", "UserUsedInvitation", "ResetPhoneNumber", "BulkInvitesToTenant", "MFAEnroll", "MFAUnenroll", "NewMFAMethod", "MFARecoveryCode", "RemoveMFAMethod", "EmailVerification", "BruteForceProtection", "SuspiciousIP", "MFAOTC", "ImpossibleTravel", "BotDetection", "SmsAuthenticationEnabled", "UnlockUser", "UnlockUserSuccess", "ActivateUserWithCode", "InviteToTenantWithCode", "VerifyNewEmail", "EmailAddressChanged", "ApprovalFlowApprove", "CountryRestriction".`,
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"ResetPassword", "ActivateUser", "InviteToTenant", "PwnedPassword", "MagicLink", "OTC",
					"ConnectNewDevice", "UserUsedInvitation", "ResetPhoneNumber", "BulkInvitesToTenant",
					"MFAEnroll", "MFAUnenroll", "NewMFAMethod", "MFARecoveryCode", "RemoveMFAMethod",
					"EmailVerification", "BruteForceProtection", "SuspiciousIP", "MFAOTC",
					"ImpossibleTravel", "BotDetection", "SmsAuthenticationEnabled",
					"UnlockUser", "UnlockUserSuccess", "ActivateUserWithCode", "InviteToTenantWithCode",
					"VerifyNewEmail", "EmailAddressChanged", "ApprovalFlowApprove", "CountryRestriction",
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
				Description: `The HTML template to use in the email.

Frontegg sanitizes the HTML it stores, so the stored form can differ from the configured one. A refresh keeps the configured HTML while the stored HTML is unchanged since Terraform last wrote or read it, and reads the stored HTML when it changed outside Terraform, or on import.`,
				Type:     schema.TypeString,
				Required: true,
			},
			"html_template_sha256": {
				Description: "The SHA-256 hash of the HTML template as Frontegg stored it, after sanitizing. Used to detect changes made outside Terraform.",
				Type:        schema.TypeString,
				Computed:    true,
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

// Frontegg sanitizes the HTML it stores, so the hash of a changed template is known only after apply.
func resourceFronteggEmailTemplateCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.HasChange("html_template") {
		return d.SetNewComputed("html_template_sha256")
	}
	return nil
}

func resourceFronteggEmailTemplateSerialize(d *schema.ResourceData) fronteggEmailTemplateRequest {
	template := fronteggEmailTemplateRequest{fronteggEmailTemplateResource: fronteggEmailTemplateResource{
		Active:             d.Get("active").(bool),
		FromName:           d.Get("from_name").(string),
		SenderEmail:        d.Get("from_address").(string),
		Subject:            d.Get("subject").(string),
		RedirectURL:        d.Get("redirect_url").(string),
		SuccessRedirectURL: d.Get("success_redirect_url").(string),
		Type:               d.Get("template_type").(string),
	}}
	// Sending the HTML unchanged would make Frontegg sanitize it again and could change the
	// stored HTML the plan left alone.
	if d.IsNewResource() || d.HasChange("html_template") {
		html := d.Get("html_template").(string)
		template.HTMLTemplate = &html
	}
	return template
}

// afterWrite marks the read that follows a write of the HTML: the stored HTML then differs
// from the configured one only by Frontegg's sanitizing, so the configured HTML is kept.
func resourceFronteggEmailTemplateDeserialize(d *schema.ResourceData, f fronteggEmailTemplateResource, afterWrite bool) error {
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
	if err := setEmailTemplateHTML(d, f.HTMLTemplate, afterWrite); err != nil {
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

func setEmailTemplateHTML(d *schema.ResourceData, stored string, afterWrite bool) error {
	sum := sha256.Sum256([]byte(stored))
	storedHash := hex.EncodeToString(sum[:])
	knownHash := d.Get("html_template_sha256").(string)
	switch {
	case afterWrite, knownHash == storedHash:
		// Unchanged since Terraform last saw it.
	case knownHash == "" && d.Get("html_template").(string) != "":
		// State from a provider version that did not record the stored HTML: take it as the
		// baseline rather than show every sanitized template as changed after an upgrade.
	default:
		// Imported, or changed outside Terraform.
		if err := d.Set("html_template", stored); err != nil {
			return err
		}
	}
	return d.Set("html_template_sha256", storedHash)
}

func resourceFronteggEmailTemplateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggEmailTemplateUpdate(ctx, d, meta)
}

func resourceFronteggEmailTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return readFronteggEmailTemplate(ctx, d, meta, false)
}

func readFronteggEmailTemplate(ctx context.Context, d *schema.ResourceData, meta interface{}, afterWrite bool) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggEmailTemplateResource
	if err := clientHolder.ApiClient.Get(ctx, fronteggEmailTemplateURL, &out); err != nil {
		return diag.FromErr(err)
	}

	templateType := d.Get("template_type").(string)
	if templateType == "" {
		templateType = d.Id()
	}
	for _, template := range out {
		if template.Type == templateType {
			if err := resourceFronteggEmailTemplateDeserialize(d, template, afterWrite); err != nil {
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
		// Nothing was written, so keep the prior state; otherwise the rejected HTML would be
		// saved as applied and never retried.
		d.Partial(true)
		return diag.FromErr(err)
	}

	d.SetId(in.Type)
	return readFronteggEmailTemplate(ctx, d, meta, in.HTMLTemplate != nil)
}

func resourceFronteggEmailTemplateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Email templates cannot be deleted, only reset to defaults
	// We'll just remove from Terraform state
	return nil
}
