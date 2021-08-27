package provider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/benesch/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggVendorURL = "https://api.frontegg.com/vendors"
const fronteggCustomDomainURL = "https://api.frontegg.com/vendors/custom-domains"
const fronteggAuthURL = "https://api.frontegg.com/identity/resources/configurations/v1"
const fronteggMFAURL = "https://api.frontegg.com/identity/resources/configurations/v1/mfa"
const fronteggMFAPolicyURL = "https://api.frontegg.com/identity/resources/configurations/v1/mfa-policy"
const fronteggLockoutPolicyURL = "https://api.frontegg.com/identity/resources/configurations/v1/lockout-policy"
const fronteggPasswordPolicyURL = "https://api.frontegg.com/identity/resources/configurations/v1/password"
const fronteggPasswordHistoryPolicyURL = "https://api.frontegg.com/identity/resources/configurations/v1/password-history-policy"
const fronteggCaptchaPolicyURL = "https://api.frontegg.com/identity/resources/configurations/v1/captcha-policy"
const fronteggOAuthURL = "https://api.frontegg.com/oauth/resources/configurations/v1"
const fronteggOAuthRedirectURIsURL = "https://api.frontegg.com/oauth/resources/configurations/v1/redirect-uri"
const fronteggSSOURL = "https://api.frontegg.com/identity/resources/sso/v1"
const fronteggSSOSAMLURL = "https://api.frontegg.com/metadata?entityName=saml"
const fronteggEmailTemplatesURL = "https://api.frontegg.com/identity/resources/mail/v1/configs/templates"
const fronteggAdminPortalURL = "https://api.frontegg.com/metadata?entityName=adminBox"

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

type fronteggCustomDomain struct {
	CustomDomain string `json:"customDomain"`
}

type fronteggAuth struct {
	ID                            string `json:"id"`
	AllowNotVerifiedUsersLogin    bool   `json:"allowNotVerifiedUsersLogin"`
	AllowSignups                  bool   `json:"allowSignups"`
	APITokensEnabled              bool   `json:"apiTokensEnabled"`
	CookieSameSite                string `json:"cookieSameSite"`
	DefaultRefreshTokenExpiration int    `json:"defaultRefreshTokenExpiration"`
	DefaultTokenExpiration        int    `json:"defaultTokenExpiration"`
	ForcePermissions              bool   `json:"forcePermissions"`
	JWTAlgorithm                  string `json:"jwtAlgorithm"`
	PublicKey                     string `json:"publicKey"`
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
	Enabled   bool    `json:"enabled"`
	SiteKey   string  `json:"siteKey"`
	SecretKey string  `json:"secretKey"`
	MinScore  float64 `json:"minScore"`
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
	Active      bool   `json:"active"`
	ClientID    string `json:"clientId"`
	RedirectURL string `json:"redirectUrl"`
	Secret      string `json:"secret"`
	Type        string `json:"type"`
}

type fronteggSSOSAML struct {
	Configuration fronteggSSOSAMLConfiguration `json:"configuration"`
	IsActive      bool                         `json:"isActive"`
	EntityName    string                       `json:"entityName"`
}

type fronteggSSOSAMLConfiguration struct {
	ACSUrl     string `json:"acsUrl"`
	SPEntityID string `json:"spEntityId"`
}

type fronteggEmailTemplate struct {
	Active       bool   `json:"active"`
	FromName     string `json:"fromName"`
	HTMLTemplate string `json:"htmlTemplate"`
	RedirectURL  string `json:"redirectURL"`
	SenderEmail  string `json:"senderEmail"`
	Subject      string `json:"subject"`
	Type         string `json:"type"`
}

type fronteggAdminPortal struct {
	Configuration fronteggAdminPortalConfiguration `json:"configuration"`
	EntityName    string                           `json:"entityName"`
}

type fronteggAdminPortalConfiguration struct {
	Navigation fronteggAdminPortalNavigation `json:"navigation"`
	Theme      fronteggAdminPortalTheme      `json:"theme"`
}

type fronteggAdminPortalNavigation struct {
	Account           fronteggAdminPortalVisibility `json:"account"`
	APITokens         fronteggAdminPortalVisibility `json:"apiTokens"`
	Audits            fronteggAdminPortalVisibility `json:"audits"`
	PersonalAPITokens fronteggAdminPortalVisibility `json:"personalApiTokens"`
	Privacy           fronteggAdminPortalVisibility `json:"privacy"`
	Profile           fronteggAdminPortalVisibility `json:"profile"`
	Roles             fronteggAdminPortalVisibility `json:"roles"`
	Security          fronteggAdminPortalVisibility `json:"security"`
	SSO               fronteggAdminPortalVisibility `json:"sso"`
	Subscriptions     fronteggAdminPortalVisibility `json:"subscriptions"`
	Usage             fronteggAdminPortalVisibility `json:"usage"`
	Users             fronteggAdminPortalVisibility `json:"users"`
	Webhooks          fronteggAdminPortalVisibility `json:"webhooks"`
}

type fronteggAdminPortalVisibility struct {
	Visibility string `json:"visibility"`
}

type fronteggAdminPortalTheme struct {
	Palette fronteggAdminPortalPalette `json:"palette"`
}

type fronteggAdminPortalPalette struct {
	Success       string `json:"success"`
	Info          string `json:"info"`
	Warning       string `json:"warning"`
	Error         string `json:"error"`
	Primary       string `json:"primary"`
	PrimaryText   string `json:"primaryText"`
	Secondary     string `json:"secondary"`
	SecondaryText string `json:"secondaryText"`
}

func resourceFronteggWorkspace() *schema.Resource {
	resourceFronteggEmail := func(typ string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
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
			},
		}
	}

	resourceFronteggSocialLogin := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"client_id": {
					Description: fmt.Sprintf("The client ID of the %s application to authenticate with.", name),
					Type:        schema.TypeString,
					Required:    true,
				},
				"redirect_url": {
					Description: "The URL to redirect to after a successful authentication.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"secret": {
					Description: fmt.Sprintf("The secret associated with the %s application.", name),
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
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

    The domain must end with ".frontegg.com".`,
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-]*\.frontegg\.com$`),
					"host must be a valid subdomain of .frontegg.com",
				),
			},
			"custom_domain": {
				Description: `A custom domain at which Frontegg services will be reachable.

    You must configure a CNAME record for this domain that points to
    "ssl.frontegg.com" before setting this field.
`,
				Type:     schema.TypeString,
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
			"auth_policy": {
				Description: "Configures the general authentication policy.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allow_unverified_users": {
							Description: "Whether unverified users are allowed to log in.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"allow_signups": {
							Description: "Whether users are allowed to sign up.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_api_tokens": {
							Description: "Whether users can create API tokens.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_roles": {
							Description: "Whether granular roles and permissions are enabled.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"jwt_algorithm": {
							Description:  "The algorithm Frontegg uses to sign JWT tokens.",
							Type:         schema.TypeString,
							Optional:     true,
							Default:      "RS256",
							ValidateFunc: validation.StringInSlice([]string{"RS256"}, false),
						},
						"jwt_access_token_expiration": {
							Description: "The expiration time for the JWT access tokens issued by Frontegg.",
							Type:        schema.TypeInt,
							Required:    true,
						},
						"jwt_refresh_token_expiration": {
							Description: "The expiration time for the JWT refresh tokens issued by Frontegg.",
							Type:        schema.TypeInt,
							Required:    true,
						},
						"jwt_public_key": {
							Description: "The public key that Frontegg uses to sign JWT tokens.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"same_site_cookie_policy": {
							Description: `The SameSite policy to use for Frontegg cookies.

	Must be one of "none", "lax", or "strict".`,
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"none", "lax", "strict"}, false),
						},
					},
				},
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
					},
				},
			},
			"reset_password_email": {
				Description: "Configures the password reset email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("ResetPassword"),
			},
			"user_activation_email": {
				Description: "Configures the user activation email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("ActivateUser"),
			},
			"user_invitation_email": {
				Description: "Configures the user invitation email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("InviteToTenant"),
			},
			"pwned_password_email": {
				Description: "Configures the pwned password email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("PwnedPassword"),
			},
			"admin_portal": {
				Description: "Configures the admin portal.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enable_account_settings": {
							Description: "Enable access to account settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_api_tokens": {
							Description: "Enable access to API tokens in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_audit_logs": {
							Description: "Enable access to audit logs in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_personal_api_tokens": {
							Description: "Enable access to personal API tokens in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_privacy": {
							Description: "Enable access to privacy settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_profile": {
							Description: "Enable access to profile settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_roles": {
							Description: "Enable access to roles and permissions in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_security": {
							Description: "Enable access to security settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_sso": {
							Description: "Enable access to SSO settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_subscriptions": {
							Description: "Enable access to subscription settings in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_usage": {
							Description: "Enable access to usage information in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_users": {
							Description: "Enable access to user management in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"enable_webhooks": {
							Description: "Enable access to webhooks in the admin portal.",
							Type:        schema.TypeBool,
							Required:    true,
						},
						"palette": {
							Description: "Configures the color palette for the admin portal.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"success": {
										Description: "Success color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"info": {
										Description: "Info color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"warning": {
										Description: "Warning color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"error": {
										Description: "Error color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"primary": {
										Description: "Primary color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"primary_text": {
										Description: "Primary text color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"secondary": {
										Description: "Secondary color.",
										Type:        schema.TypeString,
										Required:    true,
									},
									"secondary_text": {
										Description: "Secondary text color.",
										Type:        schema.TypeString,
										Required:    true,
									},
								},
							},
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

func resourceFronteggWorkspaceDeserializeMFAEnforce(s string) (string, error) {
	switch s {
	case "DontForce":
		return "off", nil
	case "Force":
		return "on", nil
	case "ForceExceptSAML":
		return "unless-saml", nil
	default:
		return "", fmt.Errorf("unexpected mfa enforcement policy: %s", s)
	}
}

func resourceFronteggWorkspaceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggWorkspaceUpdate(ctx, d, meta)
}

func resourceFronteggWorkspaceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.Client)
	{
		var out fronteggVendor
		if err := client.Get(ctx, fronteggVendorURL, &out); err != nil {
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
		if err := d.Set("allowed_origins", out.AllowedOrigins); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggCustomDomain
		client.Ignore404()
		if err := client.Get(ctx, fronteggCustomDomainURL, &out); err != nil {
			return diag.FromErr(err)
		}
		d.Set("custom_domain", out.CustomDomain)
	}
	{
		var out fronteggAuth
		if err := client.Get(ctx, fronteggAuthURL, &out); err != nil {
			return diag.FromErr(err)
		}
		auth_policy := map[string]interface{}{
			"allow_unverified_users":       out.AllowNotVerifiedUsersLogin,
			"allow_signups":                out.AllowSignups,
			"enable_api_tokens":            out.APITokensEnabled,
			"enable_roles":                 out.ForcePermissions,
			"jwt_algorithm":                out.JWTAlgorithm,
			"jwt_access_token_expiration":  out.DefaultTokenExpiration,
			"jwt_refresh_token_expiration": out.DefaultRefreshTokenExpiration,
			"jwt_public_key":               out.PublicKey,
			"same_site_cookie_policy":      strings.ToLower(out.CookieSameSite),
		}
		if err := d.Set("auth_policy", []interface{}{auth_policy}); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggMFAPolicy
		if err := client.Get(ctx, fronteggMFAPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		enforce, err := resourceFronteggWorkspaceDeserializeMFAEnforce(out.EnforceMFAType)
		if err != nil {
			return diag.FromErr(err)
		}
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
		if err := client.Get(ctx, fronteggMFAURL, &out); err != nil {
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
		if err := client.Get(ctx, fronteggLockoutPolicyURL, &out); err != nil {
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
		if err := client.Get(ctx, fronteggPasswordPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		var outHistory fronteggPasswordHistoryPolicy
		if err := client.Get(ctx, fronteggPasswordHistoryPolicyURL, &outHistory); err != nil {
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
		if err := client.Get(ctx, fronteggCaptchaPolicyURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Enabled {
			items = append(items, map[string]interface{}{
				"site_key":   out.SiteKey,
				"secret_key": out.SecretKey,
				"min_score":  out.MinScore,
			})
		}
		if err := d.Set("captcha_policy", items); err != nil {
			return diag.FromErr(err)
		}
	}
	for _, typ := range []string{"facebook", "github", "google", "microsoft"} {
		var out fronteggSSO
		client.Ignore404()
		if err := client.Get(ctx, fmt.Sprintf("%s/%s", fronteggSSOURL, typ), &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Active {
			items = append(items, map[string]interface{}{
				"client_id":    out.ClientID,
				"redirect_url": out.RedirectURL,
				"secret":       out.Secret,
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
		if err := client.Get(ctx, fronteggSSOSAMLURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.Rows[0].IsActive {
			items = append(items, map[string]interface{}{
				"acs_url":      out.Rows[0].Configuration.ACSUrl,
				"sp_entity_id": out.Rows[0].Configuration.SPEntityID,
			})
		}
		if err := d.Set("saml", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggOAuth
		if err := client.Get(ctx, fronteggOAuthURL, &out); err != nil {
			return diag.FromErr(err)
		}
		items := []interface{}{}
		if out.IsActive {
			var outRedirects fronteggOAuthRedirectURIs
			if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &outRedirects); err != nil {
				return diag.FromErr(err)
			}
			var allowedRedirectURLs []string
			for _, r := range outRedirects.RedirectURIs {
				allowedRedirectURLs = append(allowedRedirectURLs, r.RedirectURI)
			}
			items = append(items, map[string]interface{}{
				"allowed_redirect_urls": allowedRedirectURLs,
			})
		}
		if err := d.Set("hosted_login", items); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out []fronteggEmailTemplate
		if err := client.Get(ctx, fronteggEmailTemplatesURL, &out); err != nil {
			return diag.FromErr(err)
		}
		deserialize := func(field string, typ string) error {
			for _, t := range out {
				if t.Type == typ {
					var items []interface{}
					if t.Active {
						items = append(items, map[string]interface{}{
							"from_address":  t.SenderEmail,
							"from_name":     t.FromName,
							"subject":       t.Subject,
							"html_template": t.HTMLTemplate,
							"redirect_url":  t.RedirectURL,
						})
					}
					d.Set(field, items)
					return nil
				}
			}
			return fmt.Errorf("frontegg missing required email template %s", typ)
		}
		for field, typ := range map[string]string{
			"reset_password_email":  "ResetPassword",
			"user_activation_email": "ActivateUser",
			"user_invitation_email": "InviteToTenant",
			"pwned_password_email":  "PwnedPassword",
		} {
			if err := deserialize(field, typ); err != nil {
				return diag.FromErr(err)
			}
		}
	}
	{
		var out struct {
			Rows []fronteggAdminPortal `json:"rows"`
		}
		if err := client.Get(ctx, fronteggAdminPortalURL, &out); err != nil {
			return diag.FromErr(err)
		}
		nav := out.Rows[0].Configuration.Navigation
		palette := out.Rows[0].Configuration.Theme.Palette
		adminPortal := map[string]interface{}{
			"enable_account_settings":    nav.Account.Visibility == "byPermissions",
			"enable_api_tokens":          nav.APITokens.Visibility == "byPermissions",
			"enable_audit_logs":          nav.Audits.Visibility == "byPermissions",
			"enable_personal_api_tokens": nav.PersonalAPITokens.Visibility == "byPermissions",
			"enable_privacy":             nav.Privacy.Visibility == "byPermissions",
			"enable_profile":             nav.Profile.Visibility == "byPermissions",
			"enable_roles":               nav.Roles.Visibility == "byPermissions",
			"enable_security":            nav.Security.Visibility == "byPermissions",
			"enable_sso":                 nav.SSO.Visibility == "byPermissions",
			"enable_subscriptions":       nav.Subscriptions.Visibility == "byPermissions",
			"enable_usage":               nav.Usage.Visibility == "byPermissions",
			"enable_users":               nav.Users.Visibility == "byPermissions",
			"enable_webhooks":            nav.Webhooks.Visibility == "byPermissions",
			"palette": []interface{}{map[string]interface{}{
				"success":        palette.Success,
				"info":           palette.Info,
				"warning":        palette.Warning,
				"error":          palette.Error,
				"primary":        palette.Primary,
				"primary_text":   palette.PrimaryText,
				"secondary":      palette.Secondary,
				"secondary_text": palette.SecondaryText,
			}},
		}
		if err := d.Set("admin_portal", []interface{}{adminPortal}); err != nil {
			return diag.FromErr(err)
		}
	}
	return diag.Diagnostics{}
}

func resourceFronteggWorkspaceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.Client)
	{
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
		if err := client.Put(ctx, fronteggVendorURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange("custom_domain") {
		if err := client.Delete(ctx, fronteggCustomDomainURL, nil); err != nil {
			return diag.FromErr(err)
		}
		if domain, ok := d.GetOk("custom_domain"); ok {
			in := fronteggCustomDomain{CustomDomain: domain.(string)}
			// Retry for up to a minute if the CName is not found, in case it
			// was just installed and DNS is still propagating.
			err := resource.RetryContext(ctx, time.Minute, func() *resource.RetryError {
				if err := client.Post(ctx, fronteggCustomDomainURL, in, nil); err != nil {
					if strings.Contains(err.Error(), "CName not found") {
						return resource.RetryableError(err)
					} else {
						return resource.NonRetryableError(err)
					}
				}
				return nil
			})
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}
	{
		in := fronteggAuth{
			AllowNotVerifiedUsersLogin:    d.Get("auth_policy.0.allow_unverified_users").(bool),
			AllowSignups:                  d.Get("auth_policy.0.allow_signups").(bool),
			APITokensEnabled:              d.Get("auth_policy.0.enable_api_tokens").(bool),
			ForcePermissions:              d.Get("auth_policy.0.enable_roles").(bool),
			JWTAlgorithm:                  d.Get("auth_policy.0.jwt_algorithm").(string),
			DefaultTokenExpiration:        d.Get("auth_policy.0.jwt_access_token_expiration").(int),
			DefaultRefreshTokenExpiration: d.Get("auth_policy.0.jwt_refresh_token_expiration").(int),
			PublicKey:                     d.Get("auth_policy.0.jwt_public_key").(string),
			CookieSameSite:                strings.ToUpper(d.Get("auth_policy.0.same_site_cookie_policy").(string)),
		}
		if err := client.Post(ctx, fronteggAuthURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		in := fronteggMFAPolicy{
			AllowRememberMyDevice: d.Get("mfa_policy.0.allow_remember_device").(bool),
			EnforceMFAType:        resourceFronteggWorkspaceSerializeMFAEnforce(d.Get("mfa_policy.0.enforce").(string)),
			MFADeviceExpiration:   d.Get("mfa_policy.0.device_expiration").(int),
		}
		client.ConflictRetryMethod("PATCH")
		if err := client.Post(ctx, fronteggMFAPolicyURL, in, nil); err != nil {
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
		if err := client.Post(ctx, fronteggMFAURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		lockout_policy := d.Get("lockout_policy").([]interface{})
		var in fronteggLockoutPolicy
		if len(lockout_policy) > 0 {
			in.Enabled = true
			in.MaxAttempts = d.Get("lockout_policy.0.max_attempts").(int)
		}
		client.ConflictRetryMethod("PATCH")
		if err := client.Post(ctx, fronteggLockoutPolicyURL, in, nil); err != nil {
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
		if err := client.Post(ctx, fronteggPasswordPolicyURL, in, nil); err != nil {
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
		client.ConflictRetryMethod("PATCH")
		if err := client.Post(ctx, fronteggPasswordHistoryPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		captcha_policy := d.Get("captcha_policy").([]interface{})
		in := fronteggCaptchaPolicy{
			SiteKey:   " ",
			SecretKey: " ",
		}
		if len(captcha_policy) > 0 {
			in.Enabled = true
			in.SiteKey = d.Get("captcha_policy.0.site_key").(string)
			in.SecretKey = d.Get("captcha_policy.0.secret_key").(string)
			in.MinScore = d.Get("captcha_policy.0.min_score").(float64)
		}
		client.ConflictRetryMethod("PUT")
		if err := client.Post(ctx, fronteggCaptchaPolicyURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		hosted_login := d.Get("hosted_login").([]interface{})
		if len(hosted_login) > 0 {
			err := client.Post(ctx, fmt.Sprintf("%s/activate", fronteggOAuthURL), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		} else {
			err := client.Post(ctx, fmt.Sprintf("%s/deactivate", fronteggOAuthURL), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		}
		if d.HasChange("hosted_login.0.allowed_redirect_urls") {
			var outRedirects fronteggOAuthRedirectURIs
			if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &outRedirects); err != nil {
				return diag.FromErr(err)
			}
			for _, r := range outRedirects.RedirectURIs {
				err := client.Delete(ctx, fmt.Sprintf("%s/%s", fronteggOAuthRedirectURIsURL, r.ID), nil)
				if err != nil {
					return diag.FromErr(err)
				}
			}
			allowedRedirectURLs := d.Get("hosted_login.0.allowed_redirect_urls")
			if allowedRedirectURLs != nil {
				for _, url := range allowedRedirectURLs.(*schema.Set).List() {
					in := fronteggOAuthRedirectURI{
						RedirectURI: url.(string),
					}
					if err := client.Post(ctx, fronteggOAuthRedirectURIsURL, in, nil); err != nil {
						return diag.FromErr(err)
					}
				}
			}
		}
	}
	for _, typ := range []string{"facebook", "github", "google", "microsoft"} {
		name := fmt.Sprintf("%s_social_login", typ)
		if len(d.Get(name).([]interface{})) == 0 {
			client.Ignore404()
			err := client.Post(ctx, fmt.Sprintf("%s/%s/deactivate", fronteggSSOURL, typ), nil, nil)
			if err != nil {
				return diag.FromErr(err)
			}
		} else {
			in := fronteggSSO{
				ClientID:    d.Get(fmt.Sprintf("%s.0.client_id", name)).(string),
				RedirectURL: d.Get(fmt.Sprintf("%s.0.redirect_url", name)).(string),
				Secret:      d.Get(fmt.Sprintf("%s.0.secret", name)).(string),
				Type:        typ,
			}
			if err := client.Post(ctx, fronteggSSOURL, in, nil); err != nil {
				return diag.FromErr(err)
			}
			err := client.Post(ctx, fmt.Sprintf("%s/%s/activate", fronteggSSOURL, typ), nil, nil)
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
			in.IsActive = true
		}
		if err := client.Post(ctx, fronteggSSOSAMLURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	for field, typ := range map[string]string{
		"reset_password_email":  "ResetPassword",
		"user_activation_email": "ActivateUser",
		"user_invitation_email": "InviteToTenant",
		"pwned_password_email":  "PwnedPassword",
	} {
		email := d.Get(field).([]interface{})
		in := fronteggEmailTemplate{
			SenderEmail: "hello@frontegg.com",
			RedirectURL: "http://disabled",
			Type:        typ,
		}
		if len(email) > 0 {
			in.Active = true
			in.FromName = d.Get(fmt.Sprintf("%s.0.from_name", field)).(string)
			in.SenderEmail = d.Get(fmt.Sprintf("%s.0.from_address", field)).(string)
			in.Subject = d.Get(fmt.Sprintf("%s.0.subject", field)).(string)
			in.HTMLTemplate = d.Get(fmt.Sprintf("%s.0.html_template", field)).(string)
			in.RedirectURL = d.Get(fmt.Sprintf("%s.0.redirect_url", field)).(string)
		}
		if err := client.Post(ctx, fronteggEmailTemplatesURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		serializeVisibility := func(key string) fronteggAdminPortalVisibility {
			visibility := "hidden"
			if d.Get(key).(bool) {
				visibility = "byPermissions"
			}
			return fronteggAdminPortalVisibility{
				Visibility: visibility,
			}
		}
		in := fronteggAdminPortal{
			Configuration: fronteggAdminPortalConfiguration{
				Navigation: fronteggAdminPortalNavigation{
					Account:           serializeVisibility("admin_portal.0.enable_account_settings"),
					APITokens:         serializeVisibility("admin_portal.0.enable_api_tokens"),
					Audits:            serializeVisibility("admin_portal.0.enable_audit_logs"),
					PersonalAPITokens: serializeVisibility("admin_portal.0.enable_personal_api_tokens"),
					Privacy:           serializeVisibility("admin_portal.0.enable_privacy"),
					Profile:           serializeVisibility("admin_portal.0.enable_profile"),
					Roles:             serializeVisibility("admin_portal.0.enable_roles"),
					Security:          serializeVisibility("admin_portal.0.enable_security"),
					SSO:               serializeVisibility("admin_portal.0.enable_sso"),
					Subscriptions:     serializeVisibility("admin_portal.0.enable_subscriptions"),
					Usage:             serializeVisibility("admin_portal.0.enable_usage"),
					Users:             serializeVisibility("admin_portal.0.enable_users"),
					Webhooks:          serializeVisibility("admin_portal.0.enable_webhooks"),
				},
				Theme: fronteggAdminPortalTheme{
					Palette: fronteggAdminPortalPalette{
						Success:       d.Get("admin_portal.0.palette.0.success").(string),
						Info:          d.Get("admin_portal.0.palette.0.info").(string),
						Warning:       d.Get("admin_portal.0.palette.0.warning").(string),
						Error:         d.Get("admin_portal.0.palette.0.error").(string),
						Primary:       d.Get("admin_portal.0.palette.0.primary").(string),
						PrimaryText:   d.Get("admin_portal.0.palette.0.primary_text").(string),
						Secondary:     d.Get("admin_portal.0.palette.0.secondary").(string),
						SecondaryText: d.Get("admin_portal.0.palette.0.secondary_text").(string),
					},
				},
			},
			EntityName: "adminBox",
		}
		if err := client.Post(ctx, fronteggAdminPortalURL, in, nil); err != nil {
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
