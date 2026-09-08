package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggAssociatedDomainPath = "/vendors/resources/associated-domains/v1"
const fronteggAppRedirectSyncPath = "/oauth/resources/configurations/v1/redirect-uri/sync"

// The API exposes no update: the documented path is delete-and-recreate, which
// is why every attribute below is ForceNew.
type fronteggAssociatedDomain struct {
	Id                     string   `json:"id,omitempty"`
	ConfigurationId        string   `json:"configurationId,omitempty"`
	AppId                  string   `json:"appId,omitempty"`
	PackageName            string   `json:"packageName,omitempty"`
	Sha256CertFingerprints []string `json:"sha256CertFingerprints,omitempty"`
}

func (f fronteggAssociatedDomain) configurationId() string {
	if f.Id != "" {
		return f.Id
	}
	return f.ConfigurationId
}

func resourceFronteggAssociatedDomain() *schema.Resource {
	return &schema.Resource{
		Description: `Registers a mobile app for Frontegg-hosted association files.

Frontegg serves the ` + "`apple-app-site-association`" + ` and ` + "`assetlinks.json`" + ` files that bind
a mobile app to its authentication domain; this resource manages the app registrations
behind them. The Frontegg mobile SDKs rely on that binding for App-Link (https) OAuth
callbacks, magic links, password resets and passkeys.`,

		CreateContext: resourceFronteggAssociatedDomainCreate,
		ReadContext:   resourceFronteggAssociatedDomainRead,
		// Every attribute the API knows about is ForceNew; sync_redirect_uris is
		// local behaviour, so changing it needs an update that does no API work
		// rather than a destroy and recreate of the registration.
		UpdateContext: resourceFronteggAssociatedDomainRead,
		DeleteContext: resourceFronteggAssociatedDomainDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggAssociatedDomainImport,
		},

		Schema: map[string]*schema.Schema{
			"platform": {
				Description:  "The mobile platform: `ios` or `android`.",
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"ios", "android"}, false),
			},
			"app_id": {
				Description:  "The iOS app identifier in `{teamId}.{bundleId}` form, as published in the `apple-app-site-association` file. iOS only.",
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"app_id", "package_name"},
			},
			"package_name": {
				Description: "The Android application package name, as published in `assetlinks.json`. Android only.",
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
			},
			"sha256_cert_fingerprints": {
				Description:  "SHA-256 signing-certificate fingerprints of the Android app. Include every certificate the app ships under (debug, release, Play App Signing).",
				Type:         schema.TypeList,
				Optional:     true,
				ForceNew:     true,
				RequiredWith: []string{"package_name"},
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"sync_redirect_uris": {
				Description: "Whether to register the OAuth redirect URIs this app implies after changing the registration. " +
					"The Frontegg mobile SDKs derive their callback from the auth host, and it has to be allow-listed or " +
					"authorization fails after the user has already signed in. Leaving this off means registering that URI " +
					"separately, with `frontegg_redirect_uri` or by hand. " +
					"Off by default because it depends on a sync endpoint that is not available in every environment yet; " +
					"where it is missing the sync is skipped with a warning rather than failing the apply.",
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func resourceFronteggAssociatedDomainMatches(d *schema.ResourceData, f fronteggAssociatedDomain) bool {
	switch d.Get("platform").(string) {
	case "ios":
		return f.AppId != "" && f.AppId == d.Get("app_id").(string)
	case "android":
		return f.PackageName != "" && f.PackageName == d.Get("package_name").(string)
	}
	return false
}

// The docs describe the GET endpoint only as returning "the configuration", so
// this tolerates both a bare object and an array of them.
func parseAssociatedDomains(raw []byte) ([]fronteggAssociatedDomain, error) {
	var many []fronteggAssociatedDomain
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}

	var one fronteggAssociatedDomain
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, fmt.Errorf("unexpected associated domains response: %w", err)
	}
	return []fronteggAssociatedDomain{one}, nil
}

func resourceFronteggAssociatedDomainList(ctx context.Context, client restclient.Client, platform string) ([]fronteggAssociatedDomain, error) {
	var raw json.RawMessage
	if err := client.Get(ctx, fmt.Sprintf("%s/%s", fronteggAssociatedDomainPath, platform), &raw); err != nil {
		return nil, err
	}
	return parseAssociatedDomains(raw)
}

func resourceFronteggAssociatedDomainCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	platform := d.Get("platform").(string)

	in := fronteggAssociatedDomain{}
	switch platform {
	case "ios":
		in.AppId = d.Get("app_id").(string)
		if in.AppId == "" {
			return diag.Errorf("app_id is required when platform is \"ios\"")
		}
		if d.Get("package_name").(string) != "" || len(d.Get("sha256_cert_fingerprints").([]interface{})) > 0 {
			return diag.Errorf("package_name and sha256_cert_fingerprints cannot be set when platform is \"ios\"")
		}
	case "android":
		in.PackageName = d.Get("package_name").(string)
		if in.PackageName == "" {
			return diag.Errorf("package_name is required when platform is \"android\"")
		}
		for _, fp := range d.Get("sha256_cert_fingerprints").([]interface{}) {
			in.Sha256CertFingerprints = append(in.Sha256CertFingerprints, fp.(string))
		}
		if len(in.Sha256CertFingerprints) == 0 {
			return diag.Errorf("sha256_cert_fingerprints is required when platform is \"android\"")
		}
		if d.Get("app_id").(string) != "" {
			return diag.Errorf("app_id cannot be set when platform is \"android\"")
		}
	}

	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggAssociatedDomain
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s", fronteggAssociatedDomainPath, platform), in, &out); err != nil {
		return diag.FromErr(err)
	}

	if id := out.configurationId(); id != "" {
		d.SetId(id)
		return syncAppRedirectUris(ctx, d, clientHolder)
	}

	// The create response did not carry an id; find the configuration in the list.
	configs, err := resourceFronteggAssociatedDomainList(ctx, clientHolder.ApiClient, platform)
	if err != nil {
		return diag.FromErr(err)
	}
	for _, c := range configs {
		if resourceFronteggAssociatedDomainMatches(d, c) {
			d.SetId(c.configurationId())
			return syncAppRedirectUris(ctx, d, clientHolder)
		}
	}
	return diag.Errorf("associated domain configuration was created but could not be found afterwards")
}

// Registering an app changes the association files Frontegg serves, and the OAuth
// redirect URIs the mobile SDKs use are derived from those. Asking the service to
// reconcile here keeps the two in step; without it the URI is registered whenever
// the vendor configuration next changes, which may be long after the app ships.
func syncAppRedirectUris(
	ctx context.Context,
	d *schema.ResourceData,
	clientHolder *restclient.ClientHolder,
) diag.Diagnostics {
	if !d.Get("sync_redirect_uris").(bool) {
		return nil
	}

	if err := clientHolder.ApiClient.Post(ctx, fronteggAppRedirectSyncPath, struct{}{}, nil); err != nil {
		if restclient.IsNotFound(err) {
			return diag.Diagnostics{{
				Severity: diag.Warning,
				Summary:  "Redirect URI sync is not available in this environment",
				Detail: "The associated domain was registered, but the OAuth redirect URIs it implies were not, " +
					"because this environment does not expose the sync endpoint. Register them with " +
					"frontegg_redirect_uri, or set sync_redirect_uris = false to silence this.",
			}}
		}
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggAssociatedDomainRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	platform := d.Get("platform").(string)

	configs, err := resourceFronteggAssociatedDomainList(ctx, clientHolder.ApiClient, platform)
	if err != nil {
		if restclient.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	for _, c := range configs {
		if c.configurationId() != d.Id() {
			continue
		}
		if err := d.Set("app_id", c.AppId); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("package_name", c.PackageName); err != nil {
			return diag.FromErr(err)
		}
		if c.PackageName != "" {
			if err := d.Set("sha256_cert_fingerprints", c.Sha256CertFingerprints); err != nil {
				return diag.FromErr(err)
			}
		}
		return nil
	}

	d.SetId("")
	return nil
}

func resourceFronteggAssociatedDomainDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	platform := d.Get("platform").(string)
	err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s/%s", fronteggAssociatedDomainPath, platform, d.Id()), nil)
	if err != nil && !restclient.IsNotFound(err) {
		return diag.FromErr(err)
	}

	// Reconciliation only adds URIs, so this does not withdraw the callback for the
	// app just removed. It keeps the sync point consistent with create, and picks up
	// anything else the association files still publish.
	return syncAppRedirectUris(ctx, d, clientHolder)
}

// Import expects "{platform}/{configurationId}", because the API namespaces
// configurations by platform and the id alone does not say which list to read.
func resourceFronteggAssociatedDomainImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	platform, id, found := strings.Cut(d.Id(), "/")
	if !found || (platform != "ios" && platform != "android") || id == "" {
		return nil, fmt.Errorf("import id must be \"ios/{configurationId}\" or \"android/{configurationId}\", got %q", d.Id())
	}
	if err := d.Set("platform", platform); err != nil {
		return nil, err
	}
	d.SetId(id)
	return []*schema.ResourceData{d}, nil
}
