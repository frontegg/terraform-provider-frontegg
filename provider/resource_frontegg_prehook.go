package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggPrehookPath = "/prehooks/resources/configurations/v1"

type fronteggPrehook struct {
	ID          string   `json:"id,omitempty"`
	IsActive    bool     `json:"isActive"`
	DisplayName string   `json:"displayName,omitempty"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url,omitempty"`
	Secret      string   `json:"secret,omitempty"`
	EventKeys   []string `json:"eventKeys,omitempty"`
	FailMethod  string   `json:"failMethod,omitempty"` // Can be "OPEN"	or "CLOSE"
}

func resourceFronteggPrehook() *schema.Resource {
	return &schema.Resource{
		Description:   `Configures a Frontegg prehook.`,
		CreateContext: resourceFronteggPrehookCreate,
		ReadContext:   resourceFronteggPrehookRead,
		UpdateContext: resourceFronteggPrehookUpdate,
		DeleteContext: resourceFronteggPrehookDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"enabled": {
				Description: "Whether the prehook is enabled.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"name": {
				Description: "A human-readable name for the prehook.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"description": {
				Description: "A human-readable description of the prehook.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"url": {
				Description: "The URL to send events to.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"secret": {
				Description: "A secret to validate the event with.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"events": {
				Description: "The name of the event to subscribe to.",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: true,
			},
			"fail_method": {
				Description: "The action to take when the prehook fails.",
				Type:        schema.TypeString,
				Required:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"OPEN",
					"CLOSE",
				}, false),
			},
		},
	}
}

func resourceFronteggPrehookSerialize(d *schema.ResourceData) fronteggPrehook {
	return fronteggPrehook{
		IsActive:    d.Get("enabled").(bool),
		DisplayName: d.Get("name").(string),
		Description: d.Get("description").(string),
		URL:         d.Get("url").(string),
		Secret:      d.Get("secret").(string),
		EventKeys:   stringSetToList(d.Get("events").(*schema.Set)),
		FailMethod:  d.Get("fail_method").(string),
	}
}

func resourceFronteggPrehookDeserialize(d *schema.ResourceData, prehook fronteggPrehook) error {
	d.SetId(prehook.ID)

	if err := d.Set("enabled", prehook.IsActive); err != nil {
		return err
	}
	if err := d.Set("name", prehook.DisplayName); err != nil {
		return err
	}
	if err := d.Set("description", prehook.Description); err != nil {
		return err
	}
	if err := d.Set("url", prehook.URL); err != nil {
		return err
	}
	if err := d.Set("secret", prehook.Secret); err != nil {
		return err
	}
	if err := d.Set("events", prehook.EventKeys); err != nil {
		return err
	}
	if err := d.Set("fail_method", prehook.FailMethod); err != nil {
		return err
	}
	return nil
}

func resourceFronteggPrehookCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggPrehookSerialize(d)
	var out fronteggPrehook

	if err := clientHolder.ApiClient.Post(ctx, fronteggPrehookPath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPrehookDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPrehookRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out []fronteggPrehook

	if err := clientHolder.ApiClient.Get(ctx, fronteggPrehookPath, &out); err != nil {
		return diag.FromErr(err)
	}

	for _, c := range out {
		if c.ID == d.Id() {
			if err := resourceFronteggPrehookDeserialize(d, c); err != nil {
				return diag.FromErr(err)
			}
			return diag.Diagnostics{}
		}
	}
	return nil
}

func resourceFronteggPrehookUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggPrehookSerialize(d)
	var out fronteggPrehook

	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPrehookPath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPrehookDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPrehookDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPrehookPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
