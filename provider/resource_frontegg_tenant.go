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

type fronteggTenantMetadata struct {
	Metadata map[string]string `json:"metadata,omitempty"`
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
			"selected_metadata": {
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
		// Don't serialize 'Metadata' here, since it will overwrite all upstream metadata.
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
	// We only manage keys that are explicitly selected.
	selectedMetadata := castResourceStringMap(d.Get("selected_metadata"))
	var allUpstreamMetadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &allUpstreamMetadata); err != nil {
		return err
	}
	for key := range selectedMetadata {
		if newValue, ok := allUpstreamMetadata[key]; ok {
			// All metadata keys managed by this provider must use string values
			// so ignore the upstream value if it is not a string
			newValueString, ok := newValue.(string)
			if ok {
				selectedMetadata[key] = newValueString
			}
		} else {
			delete(selectedMetadata, key)
		}
	}
	if err := d.Set("selected_metadata", selectedMetadata); err != nil {
		return err
	}

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
	if selectedMetadata, is_set := d.GetOk("selected_metadata"); is_set {
		in := fronteggTenantMetadata{castResourceStringMap(selectedMetadata)}
		var out fronteggTenant
		if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/metadata", fronteggTenantPath, d.Id()), in, &out); err != nil {
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
	if d.HasChange("selected_metadata") {
		oldMetadata, newMetadata := d.GetChange("selected_metadata")
		newMetadataAsStringMap := castResourceStringMap(newMetadata)
		for key := range castResourceStringMap(oldMetadata) {
			if _, ok := newMetadataAsStringMap[key]; !ok {
				if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s/metadata/%s", fronteggTenantPath, d.Id(), key), nil); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		// Update all the keys in newMeta
		in := fronteggTenantMetadata{newMetadataAsStringMap}
		var out fronteggTenant
		if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/metadata", fronteggTenantPath, d.Id()), in, &out); err != nil {
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
