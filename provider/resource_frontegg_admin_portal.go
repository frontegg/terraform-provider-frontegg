package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggAdminPortalURL = "/metadata?entityName=adminBox"

type fronteggAdminPortal struct {
	Configuration fronteggAdminPortalConfiguration `json:"configuration"`
	EntityName    string                           `json:"entityName"`
}

type fronteggAdminPortalConfiguration struct {
	Navigation fronteggAdminPortalNavigation `json:"navigation"`
	Theme      fronteggPaletteV1             `json:"theme"`
	ThemeV2    fronteggAdminPortalThemeV2    `json:"themeV2"`
}

type fronteggAdminPortalThemeV2 struct {
	LoginBox    fronteggThemeOptions `json:"loginBox"`
	AdminPortal fronteggThemeOptions `json:"adminPortal"`
}

type fronteggThemeOptions struct {
	Palette   fronteggPaletteV2 `json:"palette"`
	ThemeName string            `json:"themeName"`
}

type fronteggPaletteV1 struct {
	Success       string `json:"success"`
	Info          string `json:"info"`
	Warning       string `json:"warning"`
	Error         string `json:"error"`
	Primary       string `json:"primary"`
	PrimaryText   string `json:"primaryText"`
	Secondary     string `json:"secondary"`
	SecondaryText string `json:"secondaryText"`
}

type fronteggPaletteV2 struct {
	Success   fronteggPaletteSeverityColor `json:"success"`
	Info      fronteggPaletteSeverityColor `json:"info"`
	Warning   fronteggPaletteSeverityColor `json:"warning"`
	Error     fronteggPaletteSeverityColor `json:"error"`
	Primary   fronteggPaletteColor         `json:"primary"`
	Secondary fronteggPaletteColor         `json:"secondary"`
}

type fronteggAdminPortalNavigation struct {
	Account           fronteggAdminPortalVisibility `json:"account"`
	APITokens         fronteggAdminPortalVisibility `json:"apiTokens"`
	Audits            fronteggAdminPortalVisibility `json:"audits"`
	Groups            fronteggAdminPortalVisibility `json:"groups"`
	PersonalAPITokens fronteggAdminPortalVisibility `json:"personalApiTokens"`
	Privacy           fronteggAdminPortalVisibility `json:"privacy"`
	Profile           fronteggAdminPortalVisibility `json:"profile"`
	Provisioning      fronteggAdminPortalVisibility `json:"provisioning"`
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

func resourceFronteggAdminPortal() *schema.Resource {
	resourceFronteggPaletteSeverityColor := func() *schema.Resource {
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

	resourceFronteggPaletteColor := func() *schema.Resource {
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

	resourceFronteggPalette := func() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"success": {
					Description: "Success color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor(),
				},
				"info": {
					Description: "Info color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor(),
				},
				"warning": {
					Description: "Warning color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor(),
				},
				"error": {
					Description: "Error color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteSeverityColor(),
				},
				"primary": {
					Description: "Primary color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor(),
				},
				"secondary": {
					Description: "Secondary color.",
					Type:        schema.TypeList | schema.TypeString,
					Optional:    true,
					MinItems:    1,
					Elem:        resourceFronteggPaletteColor(),
				},
			},
		}
	}

	return &schema.Resource{
		Description: `Admin Portal configuration.

This resource configures the Frontegg Admin Portal settings, including navigation visibility and theme customization.`,

		CreateContext: resourceFronteggAdminPortalCreate,
		ReadContext:   resourceFronteggAdminPortalRead,
		UpdateContext: resourceFronteggAdminPortalUpdate,
		DeleteContext: resourceFronteggAdminPortalDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

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
			"enable_groups": {
				Description: "Enable access to groups in the admin portal.",
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
			"enable_provisioning": {
				Description: "Enable access to provisioning settings in the admin portal.",
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
				Optional:    true,
				Deprecated:  "Use `palette_admin_portal Or/And palette_login_box` instead.",
				MinItems:    1,
				MaxItems:    1,
				Elem:        resourceFronteggPalette(),
			},
			"palette_login_box": {
				Description: "Configures the color palette for the login box.",
				Type:        schema.TypeList,
				Optional:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem:        resourceFronteggPalette(),
			},
			"palette_admin_portal": {
				Description: "Configures the color palette for the admin portal.",
				Type:        schema.TypeList,
				Optional:    true,
				MinItems:    1,
				MaxItems:    1,
				Elem:        resourceFronteggPalette(),
			},
			"admin_portal_theme_name": {
				Description: "Configures the theme name for the admin portal.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"login_box_theme_name": {
				Description: "Configures the theme name for the login box.",
				Type:        schema.TypeString,
				Optional:    true,
			},
		},
	}
}

func resourceFronteggAdminPortalCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("admin_portal")
	return resourceFronteggAdminPortalUpdate(ctx, d, meta)
}

func resourceFronteggAdminPortalRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	var metadataResponse map[string]interface{}
	if err := clientHolder.ApiClient.Get(ctx, fronteggAdminPortalURL, &metadataResponse); err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[DEBUG] metadataResponse: %v", metadataResponse)

	// Convert the response to our known structure
	jsonData, err := json.Marshal(metadataResponse)
	if err != nil {
		return diag.FromErr(err)
	}

	var out struct {
		Rows []fronteggAdminPortal `json:"rows"`
	}
	err = json.Unmarshal(jsonData, &out)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[DEBUG] out: %v", out)

	if len(out.Rows) == 0 {
		log.Printf("[DEBUG] no admin portal configuration found")
		d.SetId("")
		return nil
	}

	nav := out.Rows[0].Configuration.Navigation

	// Handle palette configuration
	var paletteItems []map[string]interface{}

	// Get palette configurations from the response, with nil checks
	var paletteV1 fronteggPaletteV1
	var paletteV2LoginBox fronteggPaletteV2
	var paletteV2AdminPortal fronteggPaletteV2

	if len(out.Rows) > 0 {
		paletteV1 = out.Rows[0].Configuration.Theme
		if out.Rows[0].Configuration.ThemeV2.LoginBox != (fronteggThemeOptions{}) {
			paletteV2LoginBox = out.Rows[0].Configuration.ThemeV2.LoginBox.Palette
		}
		if out.Rows[0].Configuration.ThemeV2.AdminPortal != (fronteggThemeOptions{}) {
			paletteV2AdminPortal = out.Rows[0].Configuration.ThemeV2.AdminPortal.Palette
		}
	}

	// Handle deprecated palette field - only set if there's actual palette data from the current config
	// This prevents setting empty/null values that cause drift
	if configPalette := d.Get("palette").([]interface{}); len(configPalette) > 0 {
		// Check if the current config uses the deprecated palette field
		// If V1 palette has values, use V1 format; otherwise use V2 format
		if paletteV1.Success != "" || paletteV1.Info != "" || paletteV1.Warning != "" || paletteV1.Error != "" {
			// Use V1 palette format for backward compatibility
			paletteMap := make(map[string]interface{})

			// Helper function to check if value is a string
			addIfString := func(key string, value interface{}) {
				if str, ok := value.(string); ok && str != "" {
					paletteMap[key] = str
				}
			}

			addIfString("success", paletteV1.Success)
			addIfString("info", paletteV1.Info)
			addIfString("warning", paletteV1.Warning)
			addIfString("error", paletteV1.Error)
			addIfString("primary", paletteV1.Primary)
			addIfString("primaryText", paletteV1.PrimaryText)
			addIfString("secondary", paletteV1.Secondary)
			addIfString("secondaryText", paletteV1.SecondaryText)

			// Only add the palette if we have any string values
			if len(paletteMap) > 0 {
				paletteItems = append(paletteItems, paletteMap)
			}
		} else {
			// Use V2 format if V1 is empty but deprecated palette is configured
			paletteItems = getPaletteItemsV2(paletteV2LoginBox)
		}
	}
	// If no deprecated palette is configured in the config, leave paletteItems empty

	// Set all the navigation configuration values using iteration
	navigationSettings := map[string]string{
		"enable_account_settings":    nav.Account.Visibility,
		"enable_api_tokens":          nav.APITokens.Visibility,
		"enable_audit_logs":          nav.Audits.Visibility,
		"enable_groups":              nav.Groups.Visibility,
		"enable_personal_api_tokens": nav.PersonalAPITokens.Visibility,
		"enable_privacy":             nav.Privacy.Visibility,
		"enable_profile":             nav.Profile.Visibility,
		"enable_provisioning":        nav.Provisioning.Visibility,
		"enable_roles":               nav.Roles.Visibility,
		"enable_security":            nav.Security.Visibility,
		"enable_sso":                 nav.SSO.Visibility,
		"enable_subscriptions":       nav.Subscriptions.Visibility,
		"enable_usage":               nav.Usage.Visibility,
		"enable_users":               nav.Users.Visibility,
		"enable_webhooks":            nav.Webhooks.Visibility,
	}

	for schemaKey, visibility := range navigationSettings {
		if err := d.Set(schemaKey, visibility == "byPermissions"); err != nil {
			return diag.FromErr(err)
		}
	}
	if err := d.Set("palette", paletteItems); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("palette_login_box", getPaletteItemsV2(paletteV2LoginBox)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("palette_admin_portal", getPaletteItemsV2(paletteV2AdminPortal)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("login_box_theme_name", out.Rows[0].Configuration.ThemeV2.LoginBox.ThemeName); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("admin_portal_theme_name", out.Rows[0].Configuration.ThemeV2.AdminPortal.ThemeName); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggAdminPortalUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

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

	serializeNewPalette := func(key string) fronteggPaletteV2 {
		paletteSuccess := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.success", key))
		paletteInfo := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.info", key))
		paletteWarning := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.warning", key))
		paletteError := serializeSeverityPaletteColor(fmt.Sprintf("%s.0.error", key))
		palettePrimary := serializePaletteColor(fmt.Sprintf("%s.0.primary", key))
		paletteSecondary := serializePaletteColor(fmt.Sprintf("%s.0.secondary", key))

		return fronteggPaletteV2{
			Success:   paletteSuccess,
			Info:      paletteInfo,
			Warning:   paletteWarning,
			Error:     paletteError,
			Primary:   palettePrimary,
			Secondary: paletteSecondary,
		}
	}

	serializeOldPalette := func(key string) fronteggPaletteV1 {
		paletteSuccess := d.Get(fmt.Sprintf("%s.0.success", key)).(string)
		paletteInfo := d.Get(fmt.Sprintf("%s.0.info", key)).(string)
		paletteWarning := d.Get(fmt.Sprintf("%s.0.warning", key)).(string)
		paletteError := d.Get(fmt.Sprintf("%s.0.error", key)).(string)
		palettePrimary := d.Get(fmt.Sprintf("%s.0.primary", key)).(string)
		palettePrimaryText := d.Get(fmt.Sprintf("%s.0.primary_text", key)).(string)
		paletteSecondary := d.Get(fmt.Sprintf("%s.0.secondary", key)).(string)
		paletteSecondaryText := d.Get(fmt.Sprintf("%s.0.secondary_text", key)).(string)

		return fronteggPaletteV1{
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

	var metadataResponse map[string]interface{}
	if err := clientHolder.ApiClient.Get(ctx, fronteggAdminPortalURL, &metadataResponse); err != nil {
		return diag.FromErr(err)
	}
	metadataResponseConfiguration := getMetadataUnstructuredConfiguration(metadataResponse)

	var configuration *fronteggAdminPortalConfiguration
	var adminPortal fronteggAdminPortal
	// adminBox is only defined when the default style of the web page has been modified, if not it's 0 rows and
	// this is not an error.
	jsonData, err := json.Marshal(metadataResponse)
	if err != nil {
		return diag.FromErr(err)
	}

	var out struct {
		Rows []fronteggAdminPortal `json:"rows"`
	}
	err = json.Unmarshal(jsonData, &out)
	if err != nil {
		return diag.FromErr(err)
	}

	switch len(out.Rows) {
	case 0:
		log.Printf("[DEBUG] no admin portal found, creating one with default config.")
		configuration = &adminPortal.Configuration
	case 1:
		adminPortal = out.Rows[0]
		configuration = &adminPortal.Configuration
	default:
		return diag.FromErr(fmt.Errorf("too many admin portals"))
	}

	configuration.Navigation.Account = serializeVisibility("enable_account_settings")
	configuration.Navigation.APITokens = serializeVisibility("enable_api_tokens")
	configuration.Navigation.Audits = serializeVisibility("enable_audit_logs")
	configuration.Navigation.Groups = serializeVisibility("enable_groups")
	configuration.Navigation.PersonalAPITokens = serializeVisibility("enable_personal_api_tokens")
	configuration.Navigation.Privacy = serializeVisibility("enable_privacy")
	configuration.Navigation.Profile = serializeVisibility("enable_profile")
	configuration.Navigation.Provisioning = serializeVisibility("enable_provisioning")
	configuration.Navigation.Roles = serializeVisibility("enable_roles")
	configuration.Navigation.Security = serializeVisibility("enable_security")
	configuration.Navigation.SSO = serializeVisibility("enable_sso")
	configuration.Navigation.Subscriptions = serializeVisibility("enable_subscriptions")
	configuration.Navigation.Usage = serializeVisibility("enable_usage")
	configuration.Navigation.Users = serializeVisibility("enable_users")
	configuration.Navigation.Webhooks = serializeVisibility("enable_webhooks")

	paletteSuccess := d.Get("palette.0.success")
	log.Printf("[DEBUG] paletteSuccess: %v", paletteSuccess)
	if reflect.TypeOf(paletteSuccess).Kind() == reflect.String {
		log.Printf("[DEBUG] paletteSuccess is a string")
		configuration.Theme = serializeOldPalette("palette")
	} else if isNonEmptySlice(paletteSuccess) {
		log.Printf("[DEBUG] paletteSuccess is a slice")
		configuration.ThemeV2.LoginBox.Palette = serializeNewPalette("palette")
	} else {
		log.Printf("[DEBUG] paletteSuccess is not a string or slice")
		if isNonEmptySlice(d.Get("palette_admin_portal.0.success")) {
			configuration.ThemeV2.AdminPortal.Palette = serializeNewPalette("palette_admin_portal")
		}
		if isNonEmptySlice(d.Get("palette_login_box.0.success")) {
			configuration.ThemeV2.LoginBox.Palette = serializeNewPalette("palette_login_box")
		}
		configuration.ThemeV2.LoginBox.ThemeName = d.Get("login_box_theme_name").(string)
		configuration.ThemeV2.AdminPortal.ThemeName = d.Get("admin_portal_theme_name").(string)
	}

	type MergedObject struct {
		EntityName    string                 `json:"entityName"`
		Configuration map[string]interface{} `json:"configuration"`
	}
	var mergedObject MergedObject
	mergedObject.EntityName = adminPortal.EntityName

	var adminPortalMapped = structToMap(adminPortal.Configuration)
	merged := mergeMaps(metadataResponseConfiguration, adminPortalMapped)
	mergedObject.Configuration = merged

	if err := clientHolder.ApiClient.Post(ctx, fronteggAdminPortalURL, mergedObject, nil); err != nil {
		return diag.FromErr(err)
	}

	return resourceFronteggAdminPortalRead(ctx, d, meta)
}

func resourceFronteggAdminPortalDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[WARN] Cannot destroy admin portal configuration. Terraform will remove this resource from the " +
		"state file, but the admin portal configuration will remain in its last-applied state.")
	return nil
}

// Helper functions
func isNonEmptySlice(value interface{}) bool {
	return len(value.([]interface{})) > 0
}

func getPaletteItemsV2(palette fronteggPaletteV2) []map[string]interface{} {
	var paletteItems []map[string]interface{}

	// Check if the palette has any actual values before creating the map
	hasValues := palette.Success.Light != "" || palette.Success.Main != "" || palette.Success.Dark != "" ||
		palette.Info.Light != "" || palette.Info.Main != "" || palette.Info.Dark != "" ||
		palette.Warning.Light != "" || palette.Warning.Main != "" || palette.Warning.Dark != "" ||
		palette.Error.Light != "" || palette.Error.Main != "" || palette.Error.Dark != "" ||
		palette.Primary.Light != "" || palette.Primary.Main != "" || palette.Primary.Dark != "" ||
		palette.Secondary.Light != "" || palette.Secondary.Main != "" || palette.Secondary.Dark != ""

	if !hasValues {
		return paletteItems // Return empty slice if no values
	}

	palleteMap := map[string]interface{}{
		"success": []interface{}{map[string]interface{}{
			"light":         palette.Success.Light,
			"main":          palette.Success.Main,
			"dark":          palette.Success.Dark,
			"contrast_text": palette.Success.ContrastText,
		}},
		"info": []interface{}{map[string]interface{}{
			"light":         palette.Info.Light,
			"main":          palette.Info.Main,
			"dark":          palette.Info.Dark,
			"contrast_text": palette.Info.ContrastText,
		}},
		"warning": []interface{}{map[string]interface{}{
			"light":         palette.Warning.Light,
			"main":          palette.Warning.Main,
			"dark":          palette.Warning.Dark,
			"contrast_text": palette.Warning.ContrastText,
		}},
		"error": []interface{}{map[string]interface{}{
			"light":         palette.Error.Light,
			"main":          palette.Error.Main,
			"dark":          palette.Error.Dark,
			"contrast_text": palette.Error.ContrastText,
		}},
		"primary": []interface{}{map[string]interface{}{
			"light":         palette.Primary.Light,
			"main":          palette.Primary.Main,
			"dark":          palette.Primary.Dark,
			"contrast_text": palette.Primary.ContrastText,
			"active":        palette.Primary.Active,
			"hover":         palette.Primary.Hover,
		}},
		"secondary": []interface{}{map[string]interface{}{
			"light":         palette.Secondary.Light,
			"main":          palette.Secondary.Main,
			"dark":          palette.Secondary.Dark,
			"contrast_text": palette.Secondary.ContrastText,
			"active":        palette.Secondary.Active,
			"hover":         palette.Secondary.Hover,
		}},
	}
	paletteItems = append(paletteItems, palleteMap)
	return paletteItems
}

func getMetadataUnstructuredConfiguration(metadataResponse map[string]interface{}) map[string]interface{} {
	var metadataResponseConfiguration map[string]interface{}

	rows, rowsExist := metadataResponse["rows"].([]interface{})
	if rowsExist && len(rows) > 0 {
		firstRow, firstRowIsMap := rows[0].(map[string]interface{})
		if firstRowIsMap {
			configuration, configIsMap := firstRow["configuration"].(map[string]interface{})
			if configIsMap {
				metadataResponseConfiguration = configuration
			}
		}
	}

	return metadataResponseConfiguration
}

func mergeMaps(m1, m2 map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	for k, v := range m1 {
		merged[k] = v
	}

	for k, v := range m2 {
		merged[k] = v
	}

	return merged
}

func structToMap(input interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	val := reflect.ValueOf(input)
	typ := reflect.TypeOf(input)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		jsonTag := fieldType.Tag.Get("json")
		result[jsonTag] = field.Interface()
	}

	return result
}
