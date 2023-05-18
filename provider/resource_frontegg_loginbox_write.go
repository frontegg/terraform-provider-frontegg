package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
