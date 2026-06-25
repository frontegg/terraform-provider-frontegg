package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggJWTTemplatePath = "/identity/resources/jwt-templates/v1"

// fronteggJWTTemplateRequiredClaims are the OIDC claims that Frontegg requires
// in every JWT template. The server rejects templates that omit them, so we
// validate at plan time to surface the error before an apply is attempted.
// Note: the aud claim must resolve to {{clientId}} or {{applicationId}}.
// See https://developers.frontegg.com/ciam/guides/security-center/token-management/claims
var fronteggJWTTemplateRequiredClaims = []string{"iss", "sub", "aud", "exp", "iat"}

// fronteggJWTTemplateReservedClaims are claims Frontegg populates internally and
// rejects when supplied in a template ("Claims reserved for internal use are not
// allowed"). We reject them at plan time so the failure surfaces before apply.
var fronteggJWTTemplateReservedClaims = []string{"type", "tenantId"}

type fronteggJWTTemplateSchema struct {
	Claims map[string]interface{} `json:"claims"`
}

type fronteggJWTTemplate struct {
	ID             string                    `json:"id,omitempty"`
	VendorID       string                    `json:"vendorId,omitempty"`
	Key            string                    `json:"key,omitempty"`
	Name           string                    `json:"name,omitempty"`
	Description    string                    `json:"description,omitempty"`
	Expiration     int                       `json:"expiration"`
	Algorithm      string                    `json:"algorithm,omitempty"`
	TemplateSchema fronteggJWTTemplateSchema `json:"templateSchema"`
	CreatedAt      string                    `json:"createdAt,omitempty"`
	UpdatedAt      string                    `json:"updatedAt,omitempty"`
}

func resourceFronteggJWTTemplate() *schema.Resource {
	return &schema.Resource{
		Description:   `Configures a Frontegg JWT template.`,
		CreateContext: resourceFronteggJWTTemplateCreate,
		ReadContext:   resourceFronteggJWTTemplateRead,
		UpdateContext: resourceFronteggJWTTemplateUpdate,
		DeleteContext: resourceFronteggJWTTemplateDelete,
		CustomizeDiff: resourceFronteggJWTTemplateValidateClaims,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"key": {
				Description: "A unique identifier key for the JWT template.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"name": {
				Description: "A human-readable name for the JWT template.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A human-readable description of the JWT template.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"expiration": {
				Description:  "The token expiration time in seconds.",
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"algorithm": {
				Description: "The JWT signing algorithm. Valid values are `RS256` and `HS256`.",
				Type:        schema.TypeString,
				Required:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"RS256",
					"HS256",
				}, false),
			},
			"claims": {
				Description: "Key-value pairs representing the JWT claims included in the template.",
				Type:        schema.TypeMap,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"vendor_id": {
				Description: "The ID of the vendor that owns the JWT template.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The timestamp at which the JWT template was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "The timestamp at which the JWT template was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

// resourceFronteggJWTTemplateValidateClaims enforces that the claims map
// includes every claim Frontegg requires. The claim keys are always known at
// plan time (only their values may reference computed attributes), so this is
// safe to evaluate during CustomizeDiff.
func resourceFronteggJWTTemplateValidateClaims(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	claims, ok := d.Get("claims").(map[string]interface{})
	if !ok {
		return nil
	}
	if missing := missingRequiredClaims(claims); len(missing) > 0 {
		return fmt.Errorf(
			"jwt template claims must include the required OIDC claims (%s); missing: %s",
			strings.Join(fronteggJWTTemplateRequiredClaims, ", "),
			strings.Join(missing, ", "),
		)
	}
	if reserved := presentReservedClaims(claims); len(reserved) > 0 {
		return fmt.Errorf(
			"jwt template claims must not include claims reserved for internal use by Frontegg (%s); remove: %s",
			strings.Join(fronteggJWTTemplateReservedClaims, ", "),
			strings.Join(reserved, ", "),
		)
	}
	return nil
}

// missingRequiredClaims returns the required claims absent from the given
// claims map, preserving the canonical order of fronteggJWTTemplateRequiredClaims.
func missingRequiredClaims(claims map[string]interface{}) []string {
	var missing []string
	for _, claim := range fronteggJWTTemplateRequiredClaims {
		if _, present := claims[claim]; !present {
			missing = append(missing, claim)
		}
	}
	return missing
}

// presentReservedClaims returns the reserved claims present in the given claims
// map, preserving the canonical order of fronteggJWTTemplateReservedClaims.
func presentReservedClaims(claims map[string]interface{}) []string {
	var present []string
	for _, claim := range fronteggJWTTemplateReservedClaims {
		if _, ok := claims[claim]; ok {
			present = append(present, claim)
		}
	}
	return present
}

func resourceFronteggJWTTemplateSerialize(d *schema.ResourceData) fronteggJWTTemplate {
	return fronteggJWTTemplate{
		Key:         d.Get("key").(string),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Expiration:  d.Get("expiration").(int),
		Algorithm:   d.Get("algorithm").(string),
		TemplateSchema: fronteggJWTTemplateSchema{
			Claims: d.Get("claims").(map[string]interface{}),
		},
	}
}

func resourceFronteggJWTTemplateDeserialize(d *schema.ResourceData, t fronteggJWTTemplate) error {
	d.SetId(t.ID)
	if err := d.Set("key", t.Key); err != nil {
		return err
	}
	if err := d.Set("name", t.Name); err != nil {
		return err
	}
	if err := d.Set("description", t.Description); err != nil {
		return err
	}
	if err := d.Set("expiration", t.Expiration); err != nil {
		return err
	}
	if err := d.Set("algorithm", t.Algorithm); err != nil {
		return err
	}
	if err := d.Set("vendor_id", t.VendorID); err != nil {
		return err
	}
	if err := d.Set("created_at", t.CreatedAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", t.UpdatedAt); err != nil {
		return err
	}
	stringClaims := make(map[string]string, len(t.TemplateSchema.Claims))
	for k, v := range t.TemplateSchema.Claims {
		sv, ok := v.(string)
		if !ok {
			return fmt.Errorf("jwt template claim %q has unexpected non-string value of type %T; only string claim values are supported", k, v)
		}
		stringClaims[k] = sv
	}
	if err := d.Set("claims", stringClaims); err != nil {
		return err
	}
	return nil
}

func resourceFronteggJWTTemplateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggJWTTemplateSerialize(d)
	var out fronteggJWTTemplate
	if err := clientHolder.ApiClient.Post(ctx, fronteggJWTTemplatePath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out fronteggJWTTemplate
	err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), &out)
	if err != nil {
		if restclient.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggJWTTemplateSerialize(d)
	var out fronteggJWTTemplate
	if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplatePath, d.Id()), nil)
	if err != nil && !restclient.IsNotFound(err) {
		return diag.FromErr(err)
	}
	return nil
}
