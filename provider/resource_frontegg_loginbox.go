package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggLoginBoxPath = "/metadata/admin-box"

type fronteggDisclaimerProperties struct {
	Enabled bool `json:"enabled,omitempty"`
}

type fronteggDisclaimer struct {
	Terms   fronteggDisclaimerProperties `json:"terms,omitempty"`
	Privacy fronteggDisclaimerProperties `json:"privacy,omitempty"`
}

type fronteggSignup struct {
	Disclaimer fronteggDisclaimer `json:"disclaimer,omitempty"`
}

type fronteggLogin struct {
	Disclaimer fronteggDisclaimer `json:"disclaimer,omitempty"`
}

type fronteggActivateAccount struct {
	Disclaimer fronteggDisclaimer `json:"disclaimer,omitempty"`
}

type fronteggLoginBoxPaletteColor struct {
	Light        string `json:"light"`
	Main         string `json:"main"`
	Dark         string `json:"dark"`
	Hover        string `json:"hover"`
	Active       string `json:"active"`
	ContrastText string `json:"contrastText"`
}

type fronteggLoginBoxPalette struct {
	Primary   fronteggLoginBoxPaletteColor `json:"primary"`
	Secondary fronteggLoginBoxPaletteColor `json:"secondary"`
}

type fronteggSocialLoginsLayout struct {
	MainButton string `json:"mainButton,omitempty"`
}

type fronteggSocialLogins struct {
	SocialLoginsLayout fronteggSocialLoginsLayout `json:"socialLoginsLayout"`
}

type fronteggLoginBox struct {
	ID              string                  `json:"id,omitempty"`
	Login           fronteggLogin           `json:"login,omitempty"`
	Signup          fronteggSignup          `json:"signup,omitempty"`
	Palette         fronteggLoginBoxPalette `json:"palette,omitempty"`
	ThemeName       string                  `json:"themeName,omitempty"`
	SocialLogins    fronteggSocialLogins    `json:"socialLogins,omitempty"`
	ActivateAccount fronteggActivateAccount `json:"activateAccount,omitempty"`
	TenantID        string                  `json:"tenantId,omitempty"`
	VendorID        string                  `json:"vendorId,omitempty"`
	CreatedAt       string                  `json:"createdAt,omitempty"`
}

func resourceFronteggLoginBox() *schema.Resource {
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
					Description: "contrast text color.",
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

	resourceFronteggPalette := func() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"primary": {
					Description: "Primary color.",
					Type:        schema.TypeList,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor("primary"),
				},
				"secondary": {
					Description: "Secondary color.",
					Type:        schema.TypeList,
					Required:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor("secondary"),
				},
			},
		}
	}

	resourceFronteggSocialLoginsLayout := func() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"main_button": {
					Description:  "Configure main social logins button. Must be one of: 'google', 'facebook', 'microsoft', 'github', 'slack', 'apple', 'linkedin'.",
					Type:         schema.TypeString,
					Optional:     true,
					Default:      "google",
					ValidateFunc: validation.StringInSlice([]string{"google", "facebook", "microsoft", "github", "slack", "apple", "linkedin"}, false),
				},
			},
		}
	}

	resourceFronteggDisclaimerProperties := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"enabled": {
					Description: "Whether disclaimer is enabled or not.",
					Type:        schema.TypeString,
					Required:    true,
				},
			},
		}
	}

	resourceFronteggDisclaimer := func(name string) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"terms": {
					Description: "Configure whether users need to agree to your terms of use.",
					Type:        schema.TypeList,
					Required:    true,
					MinItems:    1,
					MaxItems:    1,
					Elem:        resourceFronteggDisclaimerProperties("terms"),
				},
				"privacy": {
					Description: "Configure whether users need to agree to your privacy policy.",
					Type:        schema.TypeList,
					Required:    true,
					MinItems:    1,
					MaxItems:    1,
					Elem:        resourceFronteggDisclaimerProperties("privacy"),
				},
			},
		}
	}

	return &schema.Resource{
		Description: "Configures Frontegg loginBox",

		CreateContext: resourceFronteggLoginBoxCreate,
		ReadContext:   resourceFronteggLoginBoxRead,
		UpdateContext: resourceFronteggLoginBoxUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"login": {
				Description: "Login configurations.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"disclaimer": {
							Description: "Configure disclaimer.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem:        resourceFronteggDisclaimer("login"),
						},
					},
				},
			},
			"signup": {
				Description: "Signup configurations.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"disclaimer": {
							Description: "Configure disclaimer.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem:        resourceFronteggDisclaimer("signup"),
						},
					},
				},
			},
			"palette": {
				Description: "Login palette configurations.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem:        resourceFronteggPalette(),
			},
			"theme_name": {
				Description:  "Name of theme type. Must be one of: 'modern', 'classic', 'vivid', 'dark'.",
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "modern",
				ValidateFunc: validation.StringInSlice([]string{"modern", "classic", "vivid", "dark"}, false),
			},
			"social_logins": {
				Description: "Social logins configurations.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"social_logins_layout": {
							Description: "Configure layout of social logins.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem:        resourceFronteggSocialLoginsLayout(),
						},
					},
				},
			},
			"activate_account": {
				Description: "Activate account configurations.",
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"disclaimer": {
							Description: "Configure disclaimer.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							MaxItems:    1,
							Elem:        resourceFronteggDisclaimer("activate_account"),
						},
					},
				},
			},
			"tenant_id": {
				Description: "The ID of the tenant that owns the login box.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"vendor_id": {
				Description: "The ID of the vendor that owns the login box.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The timestamp at which the role was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}
