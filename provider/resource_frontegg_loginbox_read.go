package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
