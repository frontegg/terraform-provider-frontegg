package provider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggVendorURL = "/vendors"
const fronteggCustomDomainURL = "/vendors/custom-domains/v2"
const fronteggCustomDomainCreateEndpoint = "verify"
const fronteggMFAURL = "/identity/resources/configurations/v1/mfa"
const fronteggMFAPolicyURL = "/identity/resources/configurations/v1/mfa-policy"
const fronteggLockoutPolicyURL = "/identity/resources/configurations/v1/lockout-policy"
const fronteggPasswordPolicyURL = "/identity/resources/configurations/v1/password"
const fronteggPasswordHistoryPolicyURL = "/identity/resources/configurations/v1/password-history-policy"
const fronteggCaptchaPolicyURL = "/identity/resources/configurations/v1/captcha-policy"
const fronteggOAuthURL = "/oauth/resources/configurations/v1"
const fronteggOAuthRedirectURIsURL = "/oauth/resources/configurations/v1/redirect-uri"
const fronteggSSOURL = "/identity/resources/sso/v2"
const fronteggSSOSAMLURL = "/metadata?entityName=saml"
const fronteggSSOMultiTenantURL = "/team/resources/sso/v1/configurations/multiple-sso-per-domain"
const fronteggSSODomainURL = "/team/resources/sso/v1/configurations/domains"
const fronteggOIDCURL = "/team/resources/sso/v1/oidc/configurations"

type fronteggVendor struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Country           string   `json:"country"`
	BackendStack      string   `json:"backendStack"`
	FrontendStack     string   `json:"frontendStack"`
	OpenSAASInstalled bool     `json:"openSaaSInstalled"`
	Host              string   `json:"host"`
	AllowedOrigins    []string `json:"allowedOrigins"`
}

type fronteggCustomDomainStatus string

const (
	Active   fronteggCustomDomainStatus = `Active`
	Pending  fronteggCustomDomainStatus = `Pending`
	Inactive fronteggCustomDomainStatus = `Inactive`
)

type fronteggCustomDomainRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type fronteggCustomDomain struct {
	ID           string                       `json:"id,omitempty"`
	CustomDomain string                       `json:"domain,omitempty"`
	Status       string                       `json:"status,omitempty"`
	Records      []fronteggCustomDomainRecord `json:"records,omitempty"`
}

type fronteggCustomDomainCreate struct {
	CustomDomain string `json:"customDomain,omitempty"`
}

type fronteggCustomDomains struct {
	CustomDomains []fronteggCustomDomain `json:"customDomains,omitempty"`
}

type fronteggMFA struct {
	AuthenticationApp fronteggMFAAuthenticationApp `json:"authenticationApp"`
}

type fronteggMFAAuthenticationApp struct {
	Active      bool   `json:"active"`
	ServiceName string `json:"serviceName"`
}

type fronteggMFAPolicy struct {
	AllowRememberMyDevice bool   `json:"allowRememberMyDevice"`
	EnforceMFAType        string `json:"enforceMFAType"`
	MFADeviceExpiration   int    `json:"mfaDeviceExpiration"`
}

type fronteggLockoutPolicy struct {
	Enabled     bool `json:"enabled"`
	MaxAttempts int  `json:"maxAttempts"`
}

type fronteggPasswordPolicy struct {
	AllowPassphrases       bool `json:"allowPassphrases"`
	MinLength              int  `json:"minLength"`
	MaxLength              int  `json:"maxLength"`
	MinOptionalTestsToPass int  `json:"minOptionalTestsToPass"`
	MinPhraseLength        int  `json:"minPhraseLength"`
}

type fronteggPasswordHistoryPolicy struct {
	Enabled     bool `json:"enabled"`
	HistorySize int  `json:"historySize"`
}

type fronteggCaptchaPolicy struct {
	Enabled       bool     `json:"enabled"`
	SiteKey       string   `json:"siteKey"`
	SecretKey     string   `json:"secretKey"`
	MinScore      float64  `json:"minScore"`
	IgnoredEmails []string `json:"ignoredEmails"`
}

type fronteggOAuth struct {
	IsActive bool `json:"isActive"`
}

type fronteggOAuthRedirectURIs struct {
	RedirectURIs []fronteggOAuthRedirectURI `json:"redirectUris"`
}

type fronteggOAuthRedirectURI struct {
	ID          string `json:"id,omitempty"`
	RedirectURI string `json:"redirectUri,omitempty"`
}

type fronteggSSO struct {
	Active           bool     `json:"active"`
	ClientID         string   `json:"clientId"`
	RedirectURL      string   `json:"redirectUrl"`
	Secret           string   `json:"secret"`
	Type             string   `json:"type"`
	Cusomised        bool     `json:"customised"`
	AdditionalScopes []string `json:"additionalScopes,omitempty"`
}

type fronteggSSOSAML struct {
	Configuration fronteggSSOSAMLConfiguration `json:"configuration"`
	IsActive      bool                         `json:"isActive"`
	EntityName    string                       `json:"entityName"`
}

type fronteggSSOSAMLConfiguration struct {
	ACSUrl      string `json:"acsUrl"`
	SPEntityID  string `json:"spEntityId"`
	RedirectUrl string `json:"redirectUri"`
}

type fronteggSSOMultiTenant struct {
	Active                    bool   `json:"active"`
	UnspecifiedTenantStrategy string `json:"unspecifiedTenantStrategy,omitempty"`
	UseActiveTenant           bool   `json:"useActiveTenant"`
}

type fronteggSSODomain struct {
	AllowVerifiedUsersToAddDomains bool `json:"allowVerifiedUsersToAddDomains"`
	SkipDomainVerification         bool `json:"skipDomainVerification"`
	BypassDomainCrossValidation    bool `json:"bypassDomainCrossValidation"`
}

type fronteggOIDC struct {
	Active      bool   `json:"active"`
	RedirectUri string `json:"redirectUri,omitempty"`
}

func resourceFronteggWorkspace() *schema.Resource {

	resourceFronteggSocialLogin := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"client_id": {
					Description: fmt.Sprintf("The client ID of the %s application to authenticate with. Required when setting **`customised`** parameter to true.", name),
					Type:        schema.TypeString,
					Optional:    true,
				},
				"redirect_url": {
					Description: "The URL to redirect to after a successful authentication.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"secret": {
					Description: fmt.Sprintf("The secret associated with the %s application. Required when setting **`customised`** parameter to true.", name),
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
				},
				"customised": {
					Description: "Determine whether the SSO should use customized secret and client ID. When passing true, clientId and secret are also required.",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     true,
				},
				"additional_scopes": {
					Description: "Determine whether to ask for additional scopes when authenticating with the SSO provider.",
					Type:        schema.TypeSet,
					Optional:    true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		}
	}

	return &schema.Resource{
		Description: `Workspace configuration.

This is a singleton resource. You must only create one frontegg_workspace resource
per Frontegg provider.`,

		CreateContext: resourceFronteggWorkspaceCreate,
		ReadContext:   resourceFronteggWorkspaceRead,
		UpdateContext: resourceFronteggWorkspaceUpdate,
		DeleteContext: resourceFronteggWorkspaceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "The name of the workspace.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"country": {
				Description: "The country associated with the workspace.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"backend_stack": {
				Description:  "The backend stack of the application associated with the workspace.",
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"Node", "Python"}, false),
			},
			"frontend_stack": {
				Description:  "The frontend stack of the application associated with the worksapce.",
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"React", "Angular", "Vue"}, false),
			},
			"open_saas_installed": {
				Description: "Whether the application associated with the workspace has OpenSaaS installed.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"frontegg_domain": {
				Description: `The domain at which the Frontegg API is served for this workspace.

    The domain must end with ".frontegg.com" or ".us.frontegg.com".`,
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-]*(\.[a-zA-Z]+)?\.frontegg\.com$`),
					"host must be a valid subdomain of .frontegg.com",
				),
			},
			"custom_domains": {
				Description: `List of custom domains at which Frontegg services will be reachable.
				You must configure CNAME for each domain, you can get record values from the portal.`,
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"allowed_origins": {
				Description: `The origins that are allowed to access the workspace.

    This parameter controls the value of the "Origin" header for API responses.`,
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: true,
			},
			"mfa_policy": {
				Description: "Configures the multi-factor authentication (MFA) policy.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allow_remember_device": {
							Description: "Allow users to remember their MFA devices.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enforce": {
							Description: `Whether to force use of MFA.

	Must be one of "off", "on", or "unless-saml".`,
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"off", "on", "unless-saml"}, false),
						},
						"device_expiration": {
							Description: "The number of seconds that MFA devices can be remembered for, if allow_remember_my_device is true.",
							Type:        schema.TypeInt,
							Required:    true,
						},
					},
				},
			},
			"mfa_authentication_app": {
				Description: "Configures the multi-factor authentication (MFA) via an authentication app.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"service_name": {
							Description: "The service name to display in the authentication app.",
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
			"lockout_policy": {
				Description: "Configures the user lockout policy.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"max_attempts": {
							Description: "The number of failed attempts after which a user will be locked out.",
							Type:        schema.TypeInt,
							Required:    true,
						},
					},
				},
			},
			"password_policy": {
				Description: "Configures the password policy.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allow_passphrases": {
							Type:     schema.TypeBool,
							Required: true,
						},
						"min_length": {
							Description: "The minimum length of a password.",
							Type:        schema.TypeInt,
							Required:    true,
						},
						"max_length": {
							Description: "The maximum length of a password.",
							Type:        schema.TypeInt,
							Required:    true,
						},
						"min_tests": {
							Description: "The minimum number of strength tests the password must meet.",
							Type:        schema.TypeInt,
							Required:    true,
						},
						"min_phrase_length": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"history": {
							Description: "The number of historical passwords to prevent users from reusing. Set to zero to disable.",
							Type:        schema.TypeInt,
							Required:    true,
						},
					},
				},
			},
			"captcha_policy": {
				Description: "Configures the CAPTCHA policy in the signup form.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"site_key": {
							Description: "The reCAPTCHA site key to use.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"secret_key": {
							Description: "The reCAPTCHA secret key to use.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"min_score": {
							Description: "The minimum CAPTCHA score to accept. Set to 0.0 to accept all scores.",
							Type:        schema.TypeFloat,
							Required:    true,
						},
						"ignored_emails": {
							Description: "Email addresses that should be exempt from CAPTCHA checks.",
							Type:        schema.TypeSet,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"hosted_login": {
				Description: "Configures Frontegg-hosted OAuth login.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allowed_redirect_urls": {
							Description: "Allowed redirect URLs.",
							Type:        schema.TypeSet,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"facebook_social_login": {
				Description: "Configures social login with Facebook.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggSocialLogin("Facebook"),
			},
			"github_social_login": {
				Description: "Configures social login with GitHub.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggSocialLogin("GitHub"),
			},
			"google_social_login": {
				Description: "Configures social login with Google.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggSocialLogin("Google"),
			},
			"microsoft_social_login": {
				Description: "Configures social login with Google.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggSocialLogin("Microsoft"),
			},
			"saml": {
				Description: "Configures SSO via SAML.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"acs_url": {
							Description: "The ACS URL for the SAML authentication flow.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"sp_entity_id": {
							Description: "The name of the service provider that will be displayed to users.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"redirect_url": {
							Description: "The URL to redirect to after the SAML exchange.",
							Type:        schema.TypeString,
							Optional:    true,
						},
					},
				},
			},
			"oidc": {
				Description: "Configures SSO via OIDC.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"redirect_url": {
							Description: "The URL to redirect to after the OIDC exchange.",
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
			"sso_multi_tenant_policy": {
				Description: "Configures how multiple tenants can claim the same SSO domain.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"unspecified_tenant_strategy": {
							Description: "Strategy for logging in nonexisting users that match SSO configurations for multiple tenants when no tenant has been specified. Either BLOCK or FIRST_CREATED.",
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "BLOCK",
						},
						"use_active_tenant": {
							Description: "Whether users with existing accounts that match SSO configurations for multiple tenants should be logged in using the SSO for their active (last logged into) account, or whether the unspecified tenant strategy should apply.",
							Type:        schema.TypeBool,
							Optional:    true,
						},
					},
				},
			},
			"sso_domain_policy": {
				Description: "Configures how SSO domains are validated.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
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
				},
			},
		},
	}
}

func resourceFronteggWorkspaceSerializeMFAEnforce(s string) string {
	switch s {
	case "off":
		return "DontForce"
	case "force":
		return "Force"
	case "unless-saml":
		return "ForceExceptSAML"
	}
	panic("unreachable")
}

func resourceFronteggWorkspaceDeserializeMFAEnforce(s string) string {
	switch s {
	case "DontForce":
		return "off"
	case "Force":
		return "on"
	case "ForceExceptSAML":
		return "unless-saml"
	default:
		return "off"
	}
}

func resourceFronteggWorkspaceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggWorkspaceUpdate(ctx, d, meta)
}

func resourceFronteggWorkspaceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	{
		var out fronteggVendor
		if err := clientHolder.ApiClient.Get(ctx, fronteggVendorURL, &out); err != nil {
			return diag.FromErr(err)
		}
		d.SetId(out.ID)
		if err := d.Set("name", out.Name); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("country", out.Country); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("backend_stack", out.BackendStack); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("frontend_stack", out.FrontendStack); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("open_saas_installed", out.OpenSAASInstalled); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("frontegg_domain", out.Host); err != nil {
			return diag.FromErr(err)
		}
		// Normalize allowed_origins by trimming trailing slashes to prevent unnecessary plan changes
		normalizedAllowedOrigins := trimRightFromStringSlice(out.AllowedOrigins, "/")
		if err := d.Set("allowed_origins", normalizedAllowedOrigins); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var outCustomDomains fronteggCustomDomains
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggCustomDomainURL, &outCustomDomains); err != nil {
			return diag.FromErr(err)
		}

		var customDomains []string
		for _, cd := range outCustomDomains.CustomDomains {
			customDomains = append(customDomains, cd.CustomDomain)
		}

		if err := d.Set("custom_domains", customDomains); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggMFAPolicy
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggMFAPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		enforce := resourceFronteggWorkspaceDeserializeMFAEnforce(out.EnforceMFAType)

		mfa_policy := map[string]interface{}{
			"allow_remember_device": out.AllowRememberMyDevice,
			"enforce":               enforce,
			"device_expiration":     out.MFADeviceExpiration,
		}
		if err := d.Set("mfa_policy", []interface{}{mfa_policy}); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggMFA
		if err := clientHolder.ApiClient.Get(ctx, fronteggMFAURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.AuthenticationApp.Active {
			items = append(items, map[string]interface{}{
				"service_name": out.AuthenticationApp.ServiceName,
			})
		}
		if err := d.Set("mfa_authentication_app", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggLockoutPolicy
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggLockoutPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Enabled {
			items = append(items, map[string]interface{}{
				"max_attempts": out.MaxAttempts,
			})
		}
		if err := d.Set("lockout_policy", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggPasswordPolicy
		if err := clientHolder.ApiClient.Get(ctx, fronteggPasswordPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		var outHistory fronteggPasswordHistoryPolicy
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggPasswordHistoryPolicyURL, &outHistory); err != nil {
			return diag.FromErr(err)
		}
		history := 0
		if outHistory.Enabled {
			history = outHistory.HistorySize
		}
		password_policy := map[string]interface{}{
			"allow_passphrases": out.AllowPassphrases,
			"min_length":        out.MinLength,
			"max_length":        out.MaxLength,
			"min_tests":         out.MinOptionalTestsToPass,
			"min_phrase_length": out.MinPhraseLength,
			"history":           history,
		}
		if err := d.Set("password_policy", []interface{}{password_policy}); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggCaptchaPolicy
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggCaptchaPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Enabled {
			items = append(items, map[string]interface{}{
				"site_key":       out.SiteKey,
				"secret_key":     out.SecretKey,
				"min_score":      out.MinScore,
				"ignored_emails": out.IgnoredEmails,
			})
		}
		if err := d.Set("captcha_policy", items); err != nil {
			return diag.FromErr(err)
		}
	}
	for _, typ := range []string{"facebook", "github", "google", "microsoft"} {
		var out fronteggSSO
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggSSOURL, typ), &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Active {
			items = append(items, map[string]interface{}{
				"client_id":         out.ClientID,
				"redirect_url":      out.RedirectURL,
				"secret":            out.Secret,
				"customised":        out.Cusomised,
				"additional_scopes": out.AdditionalScopes,
			})
		}
		if err := d.Set(fmt.Sprintf("%s_social_login", typ), items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out struct {
			Rows []fronteggSSOSAML `json:"rows"`
		}
		if err := clientHolder.ApiClient.Get(ctx, fronteggSSOSAMLURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if len(out.Rows) > 0 && out.Rows[0].IsActive {
			items = append(items, map[string]interface{}{
				"acs_url":      out.Rows[0].Configuration.ACSUrl,
				"sp_entity_id": out.Rows[0].Configuration.SPEntityID,
				"redirect_url": out.Rows[0].Configuration.RedirectUrl,
			})
		}
		if err := d.Set("saml", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggSSOMultiTenant
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggSSOMultiTenantURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Active {
			items = append(items, map[string]interface{}{
				"unspecified_tenant_strategy": out.UnspecifiedTenantStrategy,
				"use_active_tenant":           out.UseActiveTenant,
			})
		}
		if err := d.Set("sso_multi_tenant_policy", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggSSODomain
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggSSODomainURL, &out); err != nil {
			return diag.FromErr(err)
		}
		domain_policy := map[string]interface{}{
			"allow_verified_users_to_add_domains": out.AllowVerifiedUsersToAddDomains,
			"skip_domain_verification":            out.SkipDomainVerification,
			"bypass_domain_cross_validation":      out.BypassDomainCrossValidation,
		}
		if err := d.Set("sso_domain_policy", []interface{}{domain_policy}); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggOIDC
		if err := clientHolder.ApiClient.Get(ctx, fronteggOIDCURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Active {
			items = append(items, map[string]interface{}{
				"redirect_url": out.RedirectUri,
			})
		}
		if err := d.Set("oidc", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggOAuth
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggOAuthURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.IsActive {
			var outRedirects fronteggOAuthRedirectURIs
			clientHolder.ApiClient.Ignore404()
			if err := clientHolder.ApiClient.Get(ctx, fronteggOAuthRedirectURIsURL, &outRedirects); err != nil {
				return diag.FromErr(err)
			}
			var allowedRedirectURLs []string
			for _, r := range outRedirects.RedirectURIs {
				allowedRedirectURLs = append(allowedRedirectURLs, r.RedirectURI)
			}
			// Normalize allowed_redirect_urls by trimming trailing slashes to prevent unnecessary plan changes
			normalizedRedirectURLs := trimRightFromStringSlice(allowedRedirectURLs, "/")
			items = append(items, map[string]interface{}{
				"allowed_redirect_urls": normalizedRedirectURLs,
			})
		}
		if err := d.Set("hosted_login", items); err != nil {
			return diag.FromErr(err)
		}
	}
	return diag.Diagnostics{}
}

func resourceFronteggWorkspaceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	{
		if d.HasChange("name") || d.HasChange("country") || d.HasChange("backend_stack") || d.HasChange("frontend_stack") ||
			d.HasChange("open_saas_installed") || d.HasChange("frontegg_domain") || d.HasChange("allowed_origins") {
			in := fronteggVendor{
				ID:                d.Id(),
				Name:              d.Get("name").(string),
				Country:           d.Get("country").(string),
				BackendStack:      d.Get("backend_stack").(string),
				FrontendStack:     d.Get("frontend_stack").(string),
				OpenSAASInstalled: d.Get("open_saas_installed").(bool),
				Host:              d.Get("frontegg_domain").(string),
				AllowedOrigins:    stringSetToList(d.Get("allowed_origins").(*schema.Set)),
			}
			if err := clientHolder.ApiClient.Put(ctx, fronteggVendorURL, in, nil); err != nil {
				return diag.FromErr(err)
			}
		}
	}
	if d.HasChange("custom_domains") {
		var outCustomDomains fronteggCustomDomains
		if err := clientHolder.ApiClient.Get(ctx, fronteggCustomDomainURL, &outCustomDomains); err != nil {
			return diag.FromErr(err)
		}

		var outCustomDomainsList []string
		for _, cd := range outCustomDomains.CustomDomains {
			outCustomDomainsList = append(outCustomDomainsList, cd.CustomDomain)
		}

		customDomains := stringSetToList(d.Get("custom_domains").(*schema.Set))
		for _, cd := range outCustomDomains.CustomDomains {
			if !stringInSlice(cd.CustomDomain, customDomains) {
				err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggCustomDomainURL, cd.ID), nil)
				if err != nil {
					return diag.FromErr(err)
				}
			}
		}

		for _, cd := range customDomains {
			if !stringInSlice(cd, outCustomDomainsList) {
				in := fronteggCustomDomainCreate{CustomDomain: cd}

				err := retry.RetryContext(ctx, time.Minute, func() *retry.RetryError {
					if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s", fronteggCustomDomainURL, fronteggCustomDomainCreateEndpoint), in, nil); err != nil && strings.Contains(err.Error(), "CName not found") {
						return retry.RetryableError(err)
					} else if err != nil {
						return retry.NonRetryableError(err)
					}

					return nil
				})

				if err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}
	{
		in := fronteggMFAPolicy{
			AllowRememberMyDevice: d.Get("mfa_policy.0.allow_remember_device").(bool),
			EnforceMFAType:        resourceFronteggWorkspaceSerializeMFAEnforce(d.Get("mfa_policy.0.enforce").(string)),
			MFADeviceExpiration:   d.Get("mfa_policy.0.device_expiration").(int),
		}
		clientHolder.ApiClient.ConflictRetryMethod("PATCH")
		if err := clientHolder.ApiClient.Post(ctx, fronteggMFAPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		mfa_authentication_app := d.Get("mfa_authentication_app").([]interface{})
		var in fronteggMFA
		if len(mfa_authentication_app) > 0 {
			in.AuthenticationApp.Active = true
			in.AuthenticationApp.ServiceName = d.Get("mfa_authentication_app.0.service_name").(string)
		}
		if err := clientHolder.ApiClient.Post(ctx, fronteggMFAURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		lockout_policy := d.Get("lockout_policy").([]interface{})
		var in fronteggLockoutPolicy
		if len(lockout_policy) > 0 {
			in.Enabled = true
			in.MaxAttempts = d.Get("lockout_policy.0.max_attempts").(int)
		} else {
			in.Enabled = false
			in.MaxAttempts = 5
		}
		clientHolder.ApiClient.ConflictRetryMethod("PATCH")
		if err := clientHolder.ApiClient.Post(ctx, fronteggLockoutPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}

	}
	{
		in := fronteggPasswordPolicy{
			AllowPassphrases:       d.Get("password_policy.0.allow_passphrases").(bool),
			MinLength:              d.Get("password_policy.0.min_length").(int),
			MaxLength:              d.Get("password_policy.0.max_length").(int),
			MinOptionalTestsToPass: d.Get("password_policy.0.min_tests").(int),
			MinPhraseLength:        d.Get("password_policy.0.min_phrase_length").(int),
		}
		if err := clientHolder.ApiClient.Post(ctx, fronteggPasswordPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		history := d.Get("password_policy.0.history").(int)
		in := fronteggPasswordHistoryPolicy{HistorySize: 1}
		if history > 0 {
			in.Enabled = true
			in.HistorySize = history
		}
		clientHolder.ApiClient.ConflictRetryMethod("PATCH")
		if err := clientHolder.ApiClient.Post(ctx, fronteggPasswordHistoryPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		captcha_policy := d.Get("captcha_policy").([]interface{})
		in := fronteggCaptchaPolicy{
			Enabled:   false,
			SiteKey:   "not-specified",
			SecretKey: "not-specified",
			MinScore:  0.5,
		}
		if len(captcha_policy) > 0 {
			in.Enabled = true
			in.SiteKey = d.Get("captcha_policy.0.site_key").(string)
			in.SecretKey = d.Get("captcha_policy.0.secret_key").(string)
			in.MinScore = d.Get("captcha_policy.0.min_score").(float64)
			in.IgnoredEmails = stringSetToList(d.Get("captcha_policy.0.ignored_emails").(*schema.Set))

			clientHolder.ApiClient.ConflictRetryMethod("PUT")
			if err := clientHolder.ApiClient.Post(ctx, fronteggCaptchaPolicyURL, in, nil); err != nil {
				return diag.FromErr(err)
			}
		} else {
			var currentCaptchaPolicy fronteggCaptchaPolicy
			clientHolder.ApiClient.Ignore404()
			if err := clientHolder.ApiClient.Get(ctx, fronteggCaptchaPolicyURL, &currentCaptchaPolicy); err != nil {
				return diag.FromErr(err)
			}

			// If current configuration is applied and was removed from the provider - we are turning it off
			if currentCaptchaPolicy.Enabled {
				currentCaptchaPolicy.Enabled = false
				clientHolder.ApiClient.ConflictRetryMethod("PUT")
				if err := clientHolder.ApiClient.Put(ctx, fronteggCaptchaPolicyURL, currentCaptchaPolicy, nil); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}
	{
		hosted_login := d.Get("hosted_login").([]interface{})
		if len(hosted_login) > 0 {
			err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/activate", fronteggOAuthURL), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		} else {
			err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/deactivate", fronteggOAuthURL), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		}
		if d.HasChange("hosted_login.0.allowed_redirect_urls") {
			var outRedirects fronteggOAuthRedirectURIs
			allowedRedirectURLs := d.Get("hosted_login.0.allowed_redirect_urls")
			allowedRedirectURLsList := stringSetToListWithRightTrim(allowedRedirectURLs.(*schema.Set), "/")

			if err := clientHolder.ApiClient.Get(ctx, fronteggOAuthRedirectURIsURL, &outRedirects); err != nil {
				return diag.FromErr(err)
			}
			for _, r := range outRedirects.RedirectURIs {
				if !stringInSlice(strings.TrimRight(r.RedirectURI, "/"), allowedRedirectURLsList) {
					err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggOAuthRedirectURIsURL, r.ID), nil)
					if err != nil {
						return diag.FromErr(err)
					}
				}
			}
			if allowedRedirectURLs != nil {
				for _, url := range allowedRedirectURLsList {
					exists := false
					for _, item := range outRedirects.RedirectURIs {
						if strings.TrimRight(item.RedirectURI, "/") == strings.TrimRight(url, "/") {
							exists = true
						}
					}

					if !exists {
						in := fronteggOAuthRedirectURI{
							RedirectURI: url,
						}
						if err := clientHolder.ApiClient.Post(ctx, fronteggOAuthRedirectURIsURL, in, nil); err != nil {
							return diag.FromErr(err)
						}
					}
				}
			}
		}
	}

	for _, typ := range []string{"facebook", "github", "google", "microsoft"} {
		name := fmt.Sprintf("%s_social_login", typ)
		if len(d.Get(name).([]interface{})) == 0 {
			clientHolder.ApiClient.Ignore404()
			err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/deactivate", fronteggSSOURL, typ), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		} else {
			in := fronteggSSO{
				ClientID:    d.Get(fmt.Sprintf("%s.0.client_id", name)).(string),
				RedirectURL: d.Get(fmt.Sprintf("%s.0.redirect_url", name)).(string),
				Secret:      d.Get(fmt.Sprintf("%s.0.secret", name)).(string),
				Cusomised:   d.Get(fmt.Sprintf("%s.0.customised", name)).(bool),
				Type:        typ,
			}

			if v, ok := d.GetOk(fmt.Sprintf("%s.0.additional_scopes", name)); ok {
				in.AdditionalScopes = stringSetToList(v.(*schema.Set))
			}
			if err := clientHolder.ApiClient.Post(ctx, fronteggSSOURL, in, nil); err != nil {
				return diag.FromErr(err)
			}
			err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/activate", fronteggSSOURL, typ), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}
	{
		saml := d.Get("saml").([]interface{})
		in := fronteggSSOSAML{
			EntityName: "saml",
		}
		if len(saml) > 0 {
			in.Configuration.ACSUrl = d.Get("saml.0.acs_url").(string)
			in.Configuration.SPEntityID = d.Get("saml.0.sp_entity_id").(string)
			in.Configuration.RedirectUrl = d.Get("saml.0.redirect_url").(string)
			in.IsActive = true
		}
		if err := clientHolder.ApiClient.Post(ctx, fronteggSSOSAMLURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		sso_multi_tenant := d.Get("sso_multi_tenant_policy").([]interface{})
		in := fronteggSSOMultiTenant{}
		if len(sso_multi_tenant) > 0 {
			in.Active = true
			in.UnspecifiedTenantStrategy = d.Get("sso_multi_tenant_policy.0.unspecified_tenant_strategy").(string)
			in.UseActiveTenant = d.Get("sso_multi_tenant_policy.0.use_active_tenant").(bool)
		}
		if err := clientHolder.ApiClient.Put(ctx, fronteggSSOMultiTenantURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		sso_domain := d.Get("sso_domain_policy").([]interface{})
		in := fronteggSSODomain{}
		if len(sso_domain) > 0 {
			in.AllowVerifiedUsersToAddDomains = d.Get("sso_domain_policy.0.allow_verified_users_to_add_domains").(bool)
			in.SkipDomainVerification = d.Get("sso_domain_policy.0.skip_domain_verification").(bool)
			in.BypassDomainCrossValidation = d.Get("sso_domain_policy.0.bypass_domain_cross_validation").(bool)
		}
		if err := clientHolder.ApiClient.Put(ctx, fronteggSSODomainURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		oidc := d.Get("oidc").([]interface{})
		in := fronteggOIDC{}
		if len(oidc) > 0 {
			in.Active = true
			in.RedirectUri = d.Get("oidc.0.redirect_url").(string)
		}
		if err := clientHolder.ApiClient.Post(ctx, fronteggOIDCURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	return resourceFronteggWorkspaceRead(ctx, d, meta)
}

func resourceFronteggWorkspaceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[WARN] Cannot destroy workspace. Terraform will remove this resource from the " +
		"state file, but the workspace will remain in its last-applied state.")
	return nil
}
