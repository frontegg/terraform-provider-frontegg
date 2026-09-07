package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggAllowedOriginPath = "/vendors"

// Origins live in a single list on the vendor object, so creating or deleting one
// means reading the whole list, changing an entry and writing it all back. Two of
// those interleaving lose each other's writes, which a `for_each` over origins does
// by construction. Serialize them: within one provider process this is sufficient,
// though concurrent runs against the same vendor would still need the API to expose
// a single-origin add and remove.
var allowedOriginMu sync.Mutex

type fronteggAllowedOrigins struct {
	AllowedOrigins []string `json:"allowedOrigins,omitempty"`
}

func resourceFronteggAllowedOrigin() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg allowed origin.`,

		CreateContext: resourceFronteggAllowedOriginCreate,
		ReadContext:   resourceFronteggAllowedOriginRead,
		DeleteContext: resourceFronteggAllowedOriginDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"allowed_origin": {
				Description: "The allowed origin URI.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
		},
	}
}

func resourceFronteggAllowedOriginCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	allowedOriginMu.Lock()
	defer allowedOriginMu.Unlock()

	allowedOrigins, err := getAllowedOrigins(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	newOrigin := d.Get("allowed_origin").(string)
	if containsAllowedOrigin(allowedOrigins, newOrigin) {
		return diag.FromErr(fmt.Errorf("origin '%s' already exists", newOrigin))
	}

	allowedOrigins.AllowedOrigins = append(allowedOrigins.AllowedOrigins, newOrigin)

	if err := updateAllowedOrigins(ctx, meta, allowedOrigins); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(newOrigin)
	if err := d.Set("allowed_origin", newOrigin); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggAllowedOriginRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	allowedOrigins, err := getAllowedOrigins(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	origin := d.Get("allowed_origin").(string)
	if !containsAllowedOrigin(allowedOrigins, origin) {
		d.SetId("")
		return nil
	}

	d.SetId(origin)
	if err := d.Set("allowed_origin", origin); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggAllowedOriginDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	allowedOriginMu.Lock()
	defer allowedOriginMu.Unlock()

	allowedOrigins, err := getAllowedOrigins(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	originToDelete := d.Get("allowed_origin").(string)
	if !containsAllowedOrigin(allowedOrigins, originToDelete) {
		return diag.FromErr(fmt.Errorf("origin '%s' does not exist", originToDelete))
	}

	target := normalizeAllowedOrigin(originToDelete)
	newOrigins := make([]string, 0, len(allowedOrigins.AllowedOrigins)-1)
	for _, origin := range allowedOrigins.AllowedOrigins {
		if normalizeAllowedOrigin(origin) != target {
			newOrigins = append(newOrigins, origin)
		}
	}
	allowedOrigins.AllowedOrigins = newOrigins

	if err := updateAllowedOrigins(ctx, meta, allowedOrigins); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func getAllowedOrigins(ctx context.Context, meta interface{}) (*fronteggAllowedOrigins, error) {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggAllowedOrigins
	if err := clientHolder.ApiClient.Get(ctx, fronteggAllowedOriginPath, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func updateAllowedOrigins(ctx context.Context, meta interface{}, origins *fronteggAllowedOrigins) error {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Put(ctx, fronteggAllowedOriginPath, origins, nil); err != nil {
		return err
	}

	return nil
}

// The API stores origins with a trailing slash, so "https://example.com" comes back
// as "https://example.com/". Comparing the raw strings makes Read miss origins that
// are present, which shows up as a resource Terraform wants to recreate on every plan
// and a Delete that reports the origin does not exist.
func normalizeAllowedOrigin(origin string) string {
	return strings.TrimSuffix(origin, "/")
}

func containsAllowedOrigin(origins *fronteggAllowedOrigins, newOrigin string) bool {
	target := normalizeAllowedOrigin(newOrigin)
	for _, origin := range origins.AllowedOrigins {
		if normalizeAllowedOrigin(origin) == target {
			return true
		}
	}

	return false
}
