package provider

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggVendorURL = "/vendors"
const fronteggCustomDomainURL = "/vendors/custom-domains"
const fronteggAuthURL = "/identity/resources/configurations/v1"
const fronteggMFAURL = "/identity/resources/configurations/v1/mfa"
const fronteggMFAPolicyURL = "/identity/resources/configurations/v1/mfa-policy"
const fronteggLockoutPolicyURL = "/identity/resources/configurations/v1/lockout-policy"
const fronteggPasswordPolicyURL = "/identity/resources/configurations/v1/password"
const fronteggPasswordHistoryPolicyURL = "/identity/resources/configurations/v1/password-history-policy"
const fronteggCaptchaPolicyURL = "/identity/resources/configurations/v1/captcha-policy"
const fronteggOAuthURL = "/oauth/resources/configurations/v1"
const fronteggOAuthRedirectURIsURL = "/oauth/resources/configurations/v1/redirect-uri"
const fronteggSSOURL = "/identity/resources/sso/v1"
const fronteggSSOSAMLURL = "/metadata?entityName=saml"
const fronteggEmailTemplatesURL = "/identity/resources/mail/v1/configs/templates"
const fronteggAdminPortalURL = "/metadata?entityName=adminBox"

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
	AuthStrategy                  string `json:"authStrategy"`
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
	ACSUrl      string `json:"acsUrl"`
	SPEntityID  string `json:"spEntityId"`
	RedirectUrl string `json:"redirectUri"`
}

type fronteggEmailTemplate struct {
	Active             bool   `json:"active"`
	FromName           string `json:"fromName"`
	HTMLTemplate       string `json:"htmlTemplate"`
	RedirectURL        string `json:"redirectURL"`
	SuccessRedirectURL string `json:"successRedirectUrl,omitempty"`
	SenderEmail        string `json:"senderEmail"`
	Subject            string `json:"subject"`
	Type               string `json:"type"`
}

type fronteggAdminPortal struct {
	Configuration fronteggAdminPortalConfiguration `json:"configuration"`
	EntityName    string                           `json:"entityName"`
}

type fronteggAdminPortalConfiguration struct {
	Navigation fronteggAdminPortalNavigation `json:"navigation"`
	Theme      fronteggAdminPortalTheme      `json:"theme"`
	ThemeV2    interface{}                   `json:"themeV2"`
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
	PaletteV2 fronteggAdminPortalPaletteV2 `json:"palette"`
	PaletteV1 fronteggAdminPortalPaletteV1 `json:"Palette"`
}

type fronteggAdminPortalPaletteV1 struct {
	Success       string `json:"success"`
	Info          string `json:"info"`
	Warning       string `json:"warning"`
	Error         string `json:"error"`
	Primary       string `json:"primary"`
	PrimaryText   string `json:"primaryText"`
	Secondary     string `json:"secondary"`
	SecondaryText string `json:"secondaryText"`
}

type fronteggAdminPortalPaletteV2 struct {
	Success   fronteggPaletteSeverityColor `json:"success"`
	Info      fronteggPaletteSeverityColor `json:"info"`
	Warning   fronteggPaletteSeverityColor `json:"warning"`
	Error     fronteggPaletteSeverityColor `json:"error"`
	Primary   fronteggPaletteColor         `json:"primary"`
	Secondary fronteggPaletteColor         `json:"secondary"`
}

type fronteggPaletteColor struct {
	Light        string `json:"light"`
	Main         string `json:"main"`
	Dark         string `json:"dark"`
	Hover        string `json:"hover"`
	Active       string `json:"active"`
	ContrastText string `json:"contrast_text"`
}

type fronteggPaletteSeverityColor struct {
	Light        string `json:"light"`
	Main         string `json:"main"`
	Dark         string `json:"dark"`
	ContrastText string `json:"contrast_text"`
}

func resourceFronteggWorkspace() *schema.Resource {
	resourceFronteggPaletteSeverityColor := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"light": {
					Description: "light color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"main": {
					Description: "main color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"dark": {
					Description: "dark color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"contrast_text": {
					Description: "contrast_text color.",
					Type:        schema.TypeString,
					Required:    true,
				},
			},
		}
	}

	resourceFronteggPaletteColor := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"light": {
					Description: "light color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"main": {
					Description: "main color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"dark": {
					Description: "dark color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"contrast_text": {
					Description: "contrast_text color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"active": {
					Description: "active color.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"hover": {
					Description: "hover color.",
					Type:        schema.TypeString,
					Required:    true,
				},
			},
		}
	}

	resourceFronteggPalette := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"success": {
					Description: "Success color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor("success"),
				},
				"info": {
					Description: "Info color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor("info"),
				},
				"warning": {
					Description: "Warning color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor("warning"),
				},
				"error": {
					Description: "Error color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor("error"),
				},
				"primary": {
					Description: "Primary color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor("primary"),
				},
				"secondary": {
					Description: "Secondary color.",
					Type:        schema.TypeList | schema.TypeString,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor("secondary"),
				},
			},
		}
	}

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
				"success_redirect_url": {
					Description: `The success redirect URL to use, if applicable.`,
					Type:        schema.TypeString,
					Optional:    true,
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

    The domain must end with ".frontegg.com" or ".us.frontegg.com".`,
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-]*(\.us)?\.frontegg\.com$`),
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
						"auth_strategy": {
							Description: `The authentication strategy to use for people logging in.

	Must be one of "EmailAndPassword" or "Code"`,
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"EmailAndPassword", "Code"}, false),
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
							Description: "The redirect URL to redirect after the SAML ",
							Type:        schema.TypeString,
							Optional:    true,
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
			"magic_link_email": {
				Description: "Configures the magic link email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("MagicLink"),
			},
			"magic_code_email": {
				Description: "Configures the one time code email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("OTC"),
			},
			"new_device_connected_email": {
				Description: "Configures the new device connected email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("ConnectNewDevice"),
			},
			"user_used_invitation_email": {
				Description: "Configures the user used invitation email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("UserUsedInvitation"),
			},
			"reset_phone_number_email": {
				Description: "Configures the reset phone number email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("ResetPhoneNumber"),
			},
			"bulk_tenants_invites_email": {
				Description: "Configures the bulk tenants invite email.",
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Elem:        resourceFronteggEmail("BulkInvitesToTenant"),
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
							Elem:        resourceFronteggPalette("palette"),
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
		return "off", nil
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
		if err := d.Set("allowed_origins", out.AllowedOrigins); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggCustomDomain
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggCustomDomainURL, &out); err != nil {
			return diag.FromErr(err)
		}
		d.Set("custom_domain", out.CustomDomain)
	}
	{
		var out fronteggAuth
		if err := clientHolder.ApiClient.Get(ctx, fronteggAuthURL, &out); err != nil {
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
			"auth_strategy":                out.AuthStrategy,
		}
		if err := d.Set("auth_policy", []interface{}{auth_policy}); err != nil {
			return diag.FromErr(err)
		}
	}
	{
		var out fronteggMFAPolicy
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Get(ctx, fronteggMFAPolicyURL, &out); err != nil {
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
		if err := clientHolder.ApiClient.Get(ctx, fronteggEmailTemplatesURL, &out); err != nil {
			return diag.FromErr(err)
		}
		deserialize := func(field string, typ string) error {
			for _, t := range out {
				if t.Type == typ {
					var items []interface{}
					if t.Active {
						items = append(items, map[string]interface{}{
							"from_address":         t.SenderEmail,
							"from_name":            t.FromName,
							"subject":              t.Subject,
							"html_template":        t.HTMLTemplate,
							"redirect_url":         t.RedirectURL,
							"success_redirect_url": t.SuccessRedirectURL,
						})
					}
					d.Set(field, items)
					return nil
				}
			}
			return fmt.Errorf("frontegg missing required email template %s", typ)
		}
		for field, typ := range map[string]string{
			"reset_password_email":       "ResetPassword",
			"user_activation_email":      "ActivateUser",
			"user_invitation_email":      "InviteToTenant",
			"pwned_password_email":       "PwnedPassword",
			"magic_link_email":           "MagicLink",
			"magic_code_email":           "OTC",
			"new_device_connected_email": "ConnectNewDevice",
			"user_used_invitation_email": "UserUsedInvitation",
			"reset_phone_number_email":   "ResetPhoneNumber",
			"bulk_tenants_invites_email": "BulkInvitesToTenant",
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
		if err := clientHolder.ApiClient.Get(ctx, fronteggAdminPortalURL, &out); err != nil {
			return diag.FromErr(err)
		}
		nav := out.Rows[0].Configuration.Navigation
		paletteV2 := out.Rows[0].Configuration.Theme.PaletteV2
		paletteV1 := out.Rows[0].Configuration.Theme.PaletteV1

		var paletteItems []interface{}
		if paletteV1.Error == "" && paletteV1.Success == "" {
			paletteItems = append(paletteItems, map[string]interface{}{
				"success": []interface{}{map[string]interface{}{
					"light":         paletteV2.Success.Light,
					"main":          paletteV2.Success.Main,
					"dark":          paletteV2.Success.Dark,
					"contrast_text": paletteV2.Success.ContrastText,
				}},
				"info": []interface{}{map[string]interface{}{
					"light":         paletteV2.Info.Light,
					"main":          paletteV2.Info.Main,
					"dark":          paletteV2.Info.Dark,
					"contrast_text": paletteV2.Info.ContrastText,
				}},
				"warning": []interface{}{map[string]interface{}{
					"light":         paletteV2.Warning.Light,
					"main":          paletteV2.Warning.Main,
					"dark":          paletteV2.Warning.Dark,
					"contrast_text": paletteV2.Warning.ContrastText,
				}},
				"error": []interface{}{map[string]interface{}{
					"light":         paletteV2.Error.Light,
					"main":          paletteV2.Error.Main,
					"dark":          paletteV2.Error.Dark,
					"contrast_text": paletteV2.Error.ContrastText,
				}},
				"primary": []interface{}{map[string]interface{}{
					"light":         paletteV2.Primary.Light,
					"main":          paletteV2.Primary.Main,
					"dark":          paletteV2.Primary.Dark,
					"contrast_text": paletteV2.Primary.ContrastText,
					"active":        paletteV2.Primary.Active,
					"hover":         paletteV2.Primary.Hover,
				}},
				"secondary": []interface{}{map[string]interface{}{
					"light":         paletteV2.Secondary.Light,
					"main":          paletteV2.Secondary.Main,
					"dark":          paletteV2.Secondary.Dark,
					"contrast_text": paletteV2.Secondary.ContrastText,
					"active":        paletteV2.Secondary.Active,
					"hover":         paletteV2.Secondary.Hover,
				}},
			})
		} else {
			paletteItems = append(paletteItems, map[string]interface{}{
				"success":       paletteV1.Success,
				"info":          paletteV1.Info,
				"warning":       paletteV1.Warning,
				"error":         paletteV1.Error,
				"primary":       paletteV1.Primary,
				"primaryText":   paletteV1.PrimaryText,
				"secondary":     paletteV1.Secondary,
				"secondaryText": paletteV1.SecondaryText,
			})
		}

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
			"palette":                    paletteItems,
		}
		if err := d.Set("admin_portal", []interface{}{adminPortal}); err != nil {
			return diag.FromErr(err)
		}
	}
	return diag.Diagnostics{}
}

func resourceFronteggWorkspaceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
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
		if err := clientHolder.ApiClient.Put(ctx, fronteggVendorURL, in, nil); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange("custom_domain") {
		clientHolder.ApiClient.Ignore404()
		if err := clientHolder.ApiClient.Delete(ctx, fronteggCustomDomainURL, nil); err != nil {
			return diag.FromErr(err)
		}
		if domain, ok := d.GetOk("custom_domain"); ok {
			in := fronteggCustomDomain{CustomDomain: domain.(string)}
			// Retry for up to a minute if the CName is not found, in case it
			// was just installed and DNS is still propagating.
			err := resource.RetryContext(ctx, time.Minute, func() *resource.RetryError {
				if err := clientHolder.ApiClient.Post(ctx, fronteggCustomDomainURL, in, nil); err != nil {
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
			AuthStrategy:                  d.Get("auth_policy.0.auth_strategy").(string),
		}
		if err := clientHolder.ApiClient.Post(ctx, fronteggAuthURL, in, nil); err != nil {
			return diag.FromErr(err)
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
			if err := clientHolder.ApiClient.Get(ctx, fronteggOAuthRedirectURIsURL, &outRedirects); err != nil {
				return diag.FromErr(err)
			}
			for _, r := range outRedirects.RedirectURIs {
				err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggOAuthRedirectURIsURL, r.ID), nil)
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
					if err := clientHolder.ApiClient.Post(ctx, fronteggOAuthRedirectURIsURL, in, nil); err != nil {
						return diag.FromErr(err)
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
				Type:        typ,
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
	for field, typ := range map[string]string{
		"reset_password_email":       "ResetPassword",
		"user_activation_email":      "ActivateUser",
		"user_invitation_email":      "InviteToTenant",
		"pwned_password_email":       "PwnedPassword",
		"magic_link_email":           "MagicLink",
		"magic_code_email":           "OTC",
		"new_device_connected_email": "ConnectNewDevice",
		"user_used_invitation_email": "UserUsedInvitation",
		"reset_phone_number_email":   "ResetPhoneNumber",
		"bulk_tenants_invites_email": "BulkInvitesToTenant",
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
			in.SuccessRedirectURL = d.Get(fmt.Sprintf("%s.0.success_redirect_url", field)).(string)
			if err := clientHolder.ApiClient.Post(ctx, fronteggEmailTemplatesURL, in, nil); err != nil {
				return diag.FromErr(err)
			}
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

		serializeSeverityPaletteColor := func(key string) fronteggPaletteSeverityColor {
			light := d.Get(fmt.Sprintf("%s.0.light", key)).(string)
			main := d.Get(fmt.Sprintf("%s.0.main", key)).(string)
			dark := d.Get(fmt.Sprintf("%s.0.dark", key)).(string)
			contrastText := d.Get(fmt.Sprintf("%s.0.contrast_text", key)).(string)

			return fronteggPaletteSeverityColor{
				Light:        light,
				Main:         main,
				Dark:         dark,
				ContrastText: contrastText,
			}
		}

		serializePaletteColor := func(key string) fronteggPaletteColor {
			light := d.Get(fmt.Sprintf("%s.0.light", key)).(string)
			main := d.Get(fmt.Sprintf("%s.0.main", key)).(string)
			dark := d.Get(fmt.Sprintf("%s.0.dark", key)).(string)
			hover := d.Get(fmt.Sprintf("%s.0.hover", key)).(string)
			active := d.Get(fmt.Sprintf("%s.0.active", key)).(string)
			contrastText := d.Get(fmt.Sprintf("%s.0.contrast_text", key)).(string)

			return fronteggPaletteColor{
				Light:        light,
				Main:         main,
				Dark:         dark,
				ContrastText: contrastText,
				Hover:        hover,
				Active:       active,
			}
		}

		serializeNewPalette := func(key string) fronteggAdminPortalPaletteV2 {
			paletteSuccess := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.success", key))
			paletteInfo := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.info", key))
			paletteWarning := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.warning", key))
			paletteError := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.error", key))
			palettePrimary := serializePaletteColor(fmt.Sprintf("%s.0.primary", key))
			paletteSecondary := serializePaletteColor(fmt.Sprintf("%s.0.secondary", key))

			return fronteggAdminPortalPaletteV2{
				Success:   paletteSuccess,
				Info:      paletteInfo,
				Warning:   paletteWarning,
				Error:     paletteError,
				Primary:   palettePrimary,
				Secondary: paletteSecondary,
			}
		}

		serializeOldPalette := func(key string) fronteggAdminPortalPaletteV1 {
			paletteSuccess := d.Get(fmt.Sprintf("%s.0.success", key)).(string)
			paletteInfo := d.Get(fmt.Sprintf("%s.0.info", key)).(string)
			paletteWarning := d.Get(fmt.Sprintf("%s.0.warning", key)).(string)
			paletteError := d.Get(fmt.Sprintf("%s.0.error", key)).(string)
			palettePrimary := d.Get(fmt.Sprintf("%s.0.primary", key)).(string)
			palettePrimaryText := d.Get(fmt.Sprintf("%s.0.primary_text", key)).(string)
			paletteSecondary := d.Get(fmt.Sprintf("%s.0.secondary", key)).(string)
			paletteSecondaryText := d.Get(fmt.Sprintf("%s.0.secondary_text", key)).(string)

			return fronteggAdminPortalPaletteV1{
				Success:       paletteSuccess,
				Info:          paletteInfo,
				Warning:       paletteWarning,
				Error:         paletteError,
				Primary:       palettePrimary,
				PrimaryText:   palettePrimaryText,
				Secondary:     paletteSecondary,
				SecondaryText: paletteSecondaryText,
			}
		}

		var out struct {
			Rows []fronteggAdminPortal `json:"rows"`
		}
		if err := clientHolder.ApiClient.Get(ctx, fronteggAdminPortalURL, &out); err != nil {
			return diag.FromErr(err)
		}

		var configuration fronteggAdminPortalConfiguration
		// adminBox is only defined when the default style of the web page has been modified, if not it's 0 rows and
		// this is not an error.
		if len(out.Rows) > 0 {
			adminPortal := out.Rows[0]
			configuration = adminPortal.Configuration
		}

		configuration.Navigation.Account = serializeVisibility("admin_portal.0.enable_account_settings")
		configuration.Navigation.APITokens = serializeVisibility("admin_portal.0.enable_api_tokens")
		configuration.Navigation.Audits = serializeVisibility("admin_portal.0.enable_audit_logs")
		configuration.Navigation.PersonalAPITokens = serializeVisibility("admin_portal.0.enable_personal_api_tokens")
		configuration.Navigation.Privacy = serializeVisibility("admin_portal.0.enable_privacy")
		configuration.Navigation.Profile = serializeVisibility("admin_portal.0.enable_profile")
		configuration.Navigation.Roles = serializeVisibility("admin_portal.0.enable_roles")
		configuration.Navigation.Security = serializeVisibility("admin_portal.0.enable_security")
		configuration.Navigation.SSO = serializeVisibility("admin_portal.0.enable_sso")
		configuration.Navigation.Subscriptions = serializeVisibility("admin_portal.0.enable_subscriptions")
		configuration.Navigation.Usage = serializeVisibility("admin_portal.0.enable_usage")
		configuration.Navigation.Users = serializeVisibility("admin_portal.0.enable_users")
		configuration.Navigation.Webhooks = serializeVisibility("admin_portal.0.enable_webhooks")

		paletteSuccess := d.Get("admin_portal.0.palette.0.success")
		if reflect.TypeOf(paletteSuccess).Kind() == reflect.String {
			configuration.Theme.PaletteV1 = serializeOldPalette("admin_portal.0.palette")
		} else {
			configuration.Theme.PaletteV2 = serializeNewPalette("admin_portal.0.palette")
		}

		if err := clientHolder.ApiClient.Post(ctx, fronteggAdminPortalURL, adminPortal, nil); err != nil {
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
