package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggApplicationPath = "/applications/resources/applications/v1"

type fronteggApplication struct {
	ID                    string            `json:"id,omitempty"`
	Name                  string            `json:"name"`
	AppURL                string            `json:"appURL"`
	LoginURL              string            `json:"loginURL"`
	LogoURL               string            `json:"logoURL,omitempty"`
	AccessType            string            `json:"accessType,omitempty"`
	IsDefault             bool              `json:"isDefault"`
	IsActive              bool              `json:"isActive"`
	Type                  string            `json:"type,omitempty"`
	FrontendStack         string            `json:"frontendStack,omitempty"`
	Description           string            `json:"description,omitempty"`
	IntegrationFinishedAt string            `json:"integrationFinishedAt,omitempty"`
	CreatedAt             string            `json:"createdAt,omitempty"`
	UpdatedAt             string            `json:"updatedAt,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

type fronteggApplicationCredentials struct {
	ClientSecret string `json:"clientSecret"`
	SharedSecret string `json:"sharedSecret"`
}

func resourceFronteggApplication() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg application.`,

		CreateContext: resourceFronteggApplicationCreate,
		ReadContext:   resourceFronteggApplicationRead,
		UpdateContext: resourceFronteggApplicationUpdate,
		DeleteContext: resourceFronteggApplicationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "The name of the application.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"app_url": {
				Description: "The URL of the application.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"login_url": {
				Description: "The login URL of the application.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"logo_url": {
				Description: "The URL of the application's logo.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"access_type": {
				Description: "The access type of the application.",
				Type:        schema.TypeString,
				Optional:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"FREE_ACCESS",
					"MANAGED_ACCESS",
				}, false),
			},
			"is_default": {
				Description: "Whether this is the default application.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"is_active": {
				Description: "Whether the application is active.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"type": {
				Description: "The type of the application.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "web",
				ValidateFunc: validation.StringInSlice([]string{
					"web",
					"mobile-ios",
					"mobile-android",
					"other",
				}, false),
			},
			"frontend_stack": {
				Description: "The frontend stack used by the application.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "react",
				ValidateFunc: validation.StringInSlice([]string{
					"react",
					"vue",
					"angular",
					"next.js",
					"vanilla.js",
					"ionic",
					"flutter",
					"react-native",
					"kotlin",
					"swift",
				}, false),
			},
			"description": {
				Description: "A description of the application.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"metadata": {
				Description: "Custom metadata key-value pairs for the application.",
				Type:        schema.TypeMap,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
			},
			"integration_finished_at": {
				Description: "When the integration was finished.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "When the application was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "When the application was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"client_secret": {
				Description: "The client secret of the application.",
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
			},
			"shared_secret": {
				Description: "The shared secret of the application.",
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func resourceFronteggApplicationSerialize(d *schema.ResourceData) fronteggApplication {
	var metadata map[string]string
	if v, ok := d.GetOk("metadata"); ok {
		metadata = make(map[string]string)
		for key, value := range v.(map[string]interface{}) {
			metadata[key] = value.(string)
		}
	}

	return fronteggApplication{
		Name:          d.Get("name").(string),
		AppURL:        d.Get("app_url").(string),
		LoginURL:      d.Get("login_url").(string),
		LogoURL:       d.Get("logo_url").(string),
		AccessType:    d.Get("access_type").(string),
		IsDefault:     d.Get("is_default").(bool),
		IsActive:      d.Get("is_active").(bool),
		Type:          d.Get("type").(string),
		FrontendStack: d.Get("frontend_stack").(string),
		Description:   d.Get("description").(string),
		Metadata:      metadata,
	}
}

func resourceFronteggApplicationDeserialize(d *schema.ResourceData, f fronteggApplication) error {
	d.SetId(f.ID)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("app_url", f.AppURL); err != nil {
		return err
	}
	if err := d.Set("login_url", f.LoginURL); err != nil {
		return err
	}
	if err := d.Set("logo_url", f.LogoURL); err != nil {
		return err
	}
	if err := d.Set("access_type", f.AccessType); err != nil {
		return err
	}
	if err := d.Set("is_default", f.IsDefault); err != nil {
		return err
	}
	if err := d.Set("is_active", f.IsActive); err != nil {
		return err
	}
	if err := d.Set("type", f.Type); err != nil {
		return err
	}
	if err := d.Set("frontend_stack", f.FrontendStack); err != nil {
		return err
	}
	if err := d.Set("description", f.Description); err != nil {
		return err
	}
	if err := d.Set("integration_finished_at", f.IntegrationFinishedAt); err != nil {
		return err
	}
	if err := d.Set("created_at", f.CreatedAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", f.UpdatedAt); err != nil {
		return err
	}
	if err := d.Set("metadata", f.Metadata); err != nil {
		return err
	}
	return nil
}

func resourceFronteggApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := resourceFronteggApplicationSerialize(d)
	var out fronteggApplication
	if err := clientHolder.ApiClient.Post(ctx, fronteggApplicationPath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggApplicationDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	// Get credentials after creation
	var creds fronteggApplicationCredentials
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/credentials/%s", fronteggApplicationPath, out.ID), &creds); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("client_secret", creds.ClientSecret); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("shared_secret", creds.SharedSecret); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggApplicationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	var out fronteggApplication
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggApplicationPath, d.Id()), &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggApplicationDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	// Get credentials
	var creds fronteggApplicationCredentials
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/credentials/%s", fronteggApplicationPath, d.Id()), &creds); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("client_secret", creds.ClientSecret); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("shared_secret", creds.SharedSecret); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := resourceFronteggApplicationSerialize(d)
	// Don't try to unmarshal response - API returns empty response but updates succeed
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggApplicationPath, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	// Refresh state by reading the resource after update
	return resourceFronteggApplicationRead(ctx, d, meta)
}

func resourceFronteggApplicationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggApplicationPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
