package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggAllowedOriginPath = "/vendors"

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
	d.Set("allowed_origin", newOrigin)

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
	d.Set("allowed_origin", origin)

	return nil
}

func resourceFronteggAllowedOriginDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	allowedOrigins, err := getAllowedOrigins(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	originToDelete := d.Get("allowed_origin").(string)
	if !containsAllowedOrigin(allowedOrigins, originToDelete) {
		return diag.FromErr(fmt.Errorf("origin '%s' does not exist", originToDelete))
	}

	newOrigins := make([]string, 0, len(allowedOrigins.AllowedOrigins)-1)
	for _, origin := range allowedOrigins.AllowedOrigins {
		if origin != originToDelete {
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

func containsAllowedOrigin(origins *fronteggAllowedOrigins, newOrigin string) bool {
	for _, origin := range origins.AllowedOrigins {
		if origin == newOrigin {
			return true
		}
	}

	return false
}
