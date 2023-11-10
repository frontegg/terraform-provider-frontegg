package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggTenantPath = "/tenants/resources/tenants/v1"

type fronteggTenant struct {
	Key            string `json:"tenantId,omitempty"`
	Name           string `json:"name,omitempty"`
	ApplicationUri string `json:"applicationUrl,omitempty"`
	// `Metadata` is only populated when deserializing an API response from JSON,
	// since metadata is returned as a jsonified-string.
	Metadata string `json:"metadata,omitempty"`
}

func resourceFronteggTenant() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg tenant.`,

		CreateContext: resourceFronteggTenantCreate,
		ReadContext:   resourceFronteggTenantRead,
		UpdateContext: resourceFronteggTenantUpdate,
		DeleteContext: resourceFronteggTenantDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A human-readable name for the tenant.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "A human-readable identifier for the tenant.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"application_uri": {
				Description: "The application URI for this tenant.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"desired_metadata": {
				Description: "Metadata to set and manage; will be merged with upstream metadata fields set outside of terraform.",
				Type:        schema.TypeMap,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceFronteggTenantSerialize(d *schema.ResourceData) fronteggTenant {
	return fronteggTenant{
		Name:           d.Get("name").(string),
		Key:            d.Get("key").(string),
		ApplicationUri: d.Get("application_uri").(string),
		// Don't serialize 'Metadata' here, since it will overwrite all upstream metadata keys when
		// used in the Update Tenant API, and it's not supported in the Create Tenant API.
		// Instead we update metadata fields using the Set Tenant Metadata API
	}
}

func resourceFronteggTenantDeserialize(d *schema.ResourceData, f fronteggTenant) error {
	d.SetId(f.Key)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("key", f.Key); err != nil {
		return err
	}
	if err := d.Set("application_uri", f.ApplicationUri); err != nil {
		return err
	}

	if len(f.Metadata) > 0 {
		if err := resourceFronteggTenantMetadataDeserialize(d, f.Metadata); err != nil {
			return err
		}
	}
	return nil
}

func resourceFronteggTenantMetadataDeserialize(d *schema.ResourceData, metadata string) error {
	// Get the existing desired_metadata field in the state
	// and only set the keys that exist in that field with
	// data from the upstream 'metadata' response, so we only manage
	// keys that are explicitly desired
	desiredMetadata := castResourceStringMap(d.Get("desired_metadata"))
	var allUpstreamMetadata map[string]string
	if err := json.Unmarshal([]byte(metadata), &allUpstreamMetadata); err != nil {
		return err
	}
	for key := range desiredMetadata {
		if newValue, ok := allUpstreamMetadata[key]; ok {
			desiredMetadata[key] = newValue
		} else {
			delete(desiredMetadata, key)
		}
	}
	d.Set("desired_metadata", desiredMetadata)
	return nil
}

func resourceFronteggTenantCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggTenantSerialize(d)
	var out fronteggTenant
	if err := clientHolder.ApiClient.Post(ctx, fronteggTenantPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggTenantDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	// Metadata:
	if desiredMetadata, is_set := d.GetOk("desired_metadata"); is_set {
		var out fronteggTenant
		if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/metadata", fronteggTenantPath, d.Id()), struct {
			Metadata map[string]string `json:"metadata"`
		}{castResourceStringMap(desiredMetadata)}, &out); err != nil {
			return diag.FromErr(err)
		}
		if err := resourceFronteggTenantMetadataDeserialize(d, out.Metadata); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourceFronteggTenantRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out []fronteggTenant
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), &out); err != nil {
		return diag.FromErr(err)
	}
	for _, c := range out {
		if c.Key == d.Id() {
			if err := resourceFronteggTenantDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggTenantUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	// Metadata:
	if d.HasChange("desired_metadata") {
		old, new := d.GetChange("desired_metadata")
		oldMeta := castResourceStringMap(old)
		newMeta := castResourceStringMap(new)
		for key := range oldMeta {
			// If this key was in oldMeta and not in newMeta, it needs to be removed
			if _, ok := newMeta[key]; !ok {
				if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s/metadata/%s", fronteggTenantPath, d.Id(), key), nil); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		// Update all the keys in newMeta
		var out fronteggTenant
		if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/metadata", fronteggTenantPath, d.Id()), struct {
			Metadata map[string]string `json:"metadata"`
		}{newMeta}, &out); err != nil {
			return diag.FromErr(err)
		}
		if err := resourceFronteggTenantMetadataDeserialize(d, out.Metadata); err != nil {
			return diag.FromErr(err)
		}
	}

	var out fronteggTenant
	in := resourceFronteggTenantSerialize(d)
	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggTenantDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggTenantDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggTenantPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// Convenience function to cast a schema TypeMap of TypeString elements
// to a map[string]string
func castResourceStringMap(resourceMapValue interface{}) map[string]string {
	new := make(map[string]string)
	for key, val := range resourceMapValue.(map[string]interface{}) {
		new[key] = val.(string)
	}
	return new
}
