package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
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
			"tenantId": {
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

func resourceFronteggDisclaimerDeserialize(disclaimer fronteggDisclaimer) []interface{} {
	var disclaimerItems []interface{}
	disclaimerItems = append(disclaimerItems, map[string]interface{}{
		"terms": map[string]interface{}{
			"enabled": disclaimer.Terms.Enabled,
		},
		"privacy": map[string]interface{}{
			"enabled": disclaimer.Privacy.Enabled,
		},
	})
	return disclaimerItems
}

func resourceFronteggLoginBoxPaletteDeserialize(paletteColor fronteggLoginBoxPaletteColor) []interface{} {
	var paletteColorItems []interface{}
	paletteColorItems = append(paletteColorItems, map[string]interface{}{
		"light":         paletteColor.Light,
		"main":          paletteColor.Main,
		"dark":          paletteColor.Dark,
		"hover":         paletteColor.Hover,
		"active":        paletteColor.Active,
		"contrast_text": paletteColor.ContrastText,
	})
	return paletteColorItems
}

func resourceFronteggLoginBoxRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggLoginBox
	if err := clientHolder.ApiClient.GetWithHeaders(ctx, fronteggLoginBoxPath, headers, &out); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(out.ID)
	var loginItems []interface{}
	loginItems = append(loginItems, map[string]interface{}{
		"disclaimer": resourceFronteggDisclaimerDeserialize(out.Login.Disclaimer),
	})
	if err := d.Set("login", []interface{}{loginItems}); err != nil {
		return diag.FromErr(err)
	}
	signupItems := append(loginItems, map[string]interface{}{
		"disclaimer": resourceFronteggDisclaimerDeserialize(out.Signup.Disclaimer),
	})
	if err := d.Set("signup", []interface{}{signupItems}); err != nil {
		return diag.FromErr(err)
	}
	var paletteItems []interface{}
	paletteItems = append(paletteItems, map[string]interface{}{
		"primary":   resourceFronteggLoginBoxPaletteDeserialize(out.Palette.Primary),
		"secondary": resourceFronteggLoginBoxPaletteDeserialize(out.Palette.Secondary),
	})
	if err := d.Set("palette", []interface{}{paletteItems}); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("theme_name", out.ThemeName); err != nil {
		return diag.FromErr(err)
	}
	var socialLoginsItems []interface{}
	socialLoginsItems = append(socialLoginsItems, map[string]interface{}{
		"social_logins_layout": map[string]interface{}{
			"main_button": out.SocialLogins.SocialLoginsLayout.MainButton,
		},
	})
	if err := d.Set("social_logins", []interface{}{socialLoginsItems}); err != nil {
		return diag.FromErr(err)
	}
	activateAccountItems := append(loginItems, map[string]interface{}{
		"disclaimer": resourceFronteggDisclaimerDeserialize(out.ActivateAccount.Disclaimer),
	})
	if err := d.Set("activate_account", []interface{}{activateAccountItems}); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("tenant_id", out.TenantID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("vendor_id", out.VendorID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("created_at", out.CreatedAt); err != nil {
		return diag.FromErr(err)
	}
	return diag.Diagnostics{}
}

func resourceFronteggLoginBoxCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggLoginBoxUpdate(ctx, d, meta)
}

func resourceFronteggLoginBoxSerialize(d *schema.ResourceData) fronteggLoginBox {

	var loginBox fronteggLoginBox

	serializeLoginBoxDisclaimer := func(key string) fronteggDisclaimer {
		return fronteggDisclaimer{
			Terms: fronteggDisclaimerProperties{
				Enabled: d.Get(fmt.Sprintf("%s.0.terms.0.enabled", key)).(bool),
			},
			Privacy: fronteggDisclaimerProperties{
				Enabled: d.Get(fmt.Sprintf("%s.0.privacy.0.enabled", key)).(bool),
			},
		}
	}

	serializeLoginBoxPaletteColor := func(key string) fronteggLoginBoxPaletteColor {
		return fronteggLoginBoxPaletteColor{
			Light:        d.Get(fmt.Sprintf("%s.0.light", key)).(string),
			Main:         d.Get(fmt.Sprintf("%s.0.main", key)).(string),
			Dark:         d.Get(fmt.Sprintf("%s.0.dark", key)).(string),
			Hover:        d.Get(fmt.Sprintf("%s.0.hover", key)).(string),
			Active:       d.Get(fmt.Sprintf("%s.0.active", key)).(string),
			ContrastText: d.Get(fmt.Sprintf("%s.0.contrast_text", key)).(string),
		}
	}

	serializeLoginBoxPalettte := func(key string) fronteggLoginBoxPalette {
		return fronteggLoginBoxPalette{
			Primary:   serializeLoginBoxPaletteColor(fmt.Sprintf("%s.0.primary", key)),
			Secondary: serializeLoginBoxPaletteColor(fmt.Sprintf("%s.0.secondary", key)),
		}
	}

	serializeLoginBoxSocialLoginsLayout := func(key string) fronteggSocialLoginsLayout {
		return fronteggSocialLoginsLayout{
			MainButton: d.Get(fmt.Sprintf("%s.0.main_button", key)).(string),
		}
	}

	loginBox.Login.Disclaimer = serializeLoginBoxDisclaimer("login")
	loginBox.Signup.Disclaimer = serializeLoginBoxDisclaimer("signup")
	loginBox.Palette = serializeLoginBoxPalettte("palette")
	loginBox.ThemeName = d.Get("theme_name").(string)
	loginBox.SocialLogins.SocialLoginsLayout = serializeLoginBoxSocialLoginsLayout("social_logins")
	loginBox.ActivateAccount.Disclaimer = serializeLoginBoxDisclaimer("activate_account")

	return loginBox
}

func resourceFronteggLoginBoxUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	headers := getTenantIdHeaders(d)
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggLoginBoxSerialize(d)
	if err := clientHolder.ApiClient.PostWithHeaders(ctx, fronteggLoginBoxPath, headers, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggLoginBoxRead(ctx, d, meta)
}
