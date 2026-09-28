package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/frontegg/terraform-provider-frontegg/provider/validators"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggJWTTemplatePath = "/identity/resources/jwt-templates/v1"

// fronteggJWTTemplateRequiredClaims are the OIDC claims that Frontegg requires
// in every JWT template. The server rejects templates that omit them, so we
// validate at plan time to surface the error before an apply is attempted.
// Note: the aud claim must resolve to {{clientId}} or {{applicationId}}.
// See https://developers.frontegg.com/ciam/guides/security-center/token-management/claims
var fronteggJWTTemplateRequiredClaims = []string{"iss", "sub", "aud", "exp", "iat"}

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
				Description: "Key-value pairs representing the JWT claims included in the template. " +
					"Claims are not auto-populated: the token carries exactly what is set here, so " +
					"include `tenantId` if your application or the Frontegg frontend SDK expects it. " +
					"The standard OIDC claims (`iss`, `sub`, `aud`, `exp`, `iat`) are required, where " +
					"`aud` must be `{{clientId}}` or `{{applicationId}}`. A small set of claims is " +
					"reserved for internal use and rejected by the API (for example `type`, `userId`, " +
					"`superUser`, `act`, `amr`, `acr`, `auth_time`, `nonce`). Only string values are " +
					"supported; use `claims_json` for claims with nested object, array, number or boolean " +
					"values. Exactly one of `claims` or `claims_json` must be set.",
				Type:         schema.TypeMap,
				Optional:     true,
				ExactlyOneOf: []string{"claims", "claims_json"},
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"claims_json": {
				Description: "The JWT claims included in the template, as a JSON object (typically built " +
					"with `jsonencode`). Use this instead of `claims` when any claim value is not a string, " +
					"such as a nested object. The same required and reserved claims apply as for `claims`. " +
					"Numbers are handled as 64-bit floats, so encode integers larger than 2^53 as strings. " +
					"Exactly one of `claims` or `claims_json` must be set.",
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"claims", "claims_json"},
				ValidateFunc: validators.ValidateJSON,
				StateFunc:    normalizeJSONState,
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

// resourceFronteggJWTTemplateValidateClaims rejects claims missing a required claim, skipping claims unknown until apply.
func resourceFronteggJWTTemplateValidateClaims(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if !d.NewValueKnown("claims.%") || !d.NewValueKnown("claims_json") {
		return nil
	}
	claims, err := resourceFronteggJWTTemplateClaims(d.Get("claims_json").(string), d.Get("claims").(map[string]interface{}))
	if err != nil {
		return err
	}
	if missing := missingRequiredClaims(claims); len(missing) > 0 {
		return fmt.Errorf(
			"jwt template claims must include the required OIDC claims (%s); missing: %s",
			strings.Join(fronteggJWTTemplateRequiredClaims, ", "),
			strings.Join(missing, ", "),
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

func resourceFronteggJWTTemplateClaims(claimsJSON string, claims map[string]interface{}) (map[string]interface{}, error) {
	if claimsJSON == "" {
		return claims, nil
	}
	decodedClaims, err := structure.ExpandJsonFromString(claimsJSON)
	if err != nil {
		return nil, fmt.Errorf("claims_json: %w", err)
	}
	if decodedClaims == nil {
		return nil, fmt.Errorf("claims_json must be a JSON object, not null")
	}
	return decodedClaims, nil
}

func normalizeJSONState(value interface{}) string {
	normalized, _ := structure.NormalizeJsonString(value)
	return normalized
}

func hasNonStringClaim(claims map[string]interface{}) bool {
	for _, value := range claims {
		if _, isString := value.(string); !isString {
			return true
		}
	}
	return false
}

func resourceFronteggJWTTemplateSerialize(d *schema.ResourceData) (fronteggJWTTemplate, error) {
	claims, err := resourceFronteggJWTTemplateClaims(d.Get("claims_json").(string), d.Get("claims").(map[string]interface{}))
	if err != nil {
		return fronteggJWTTemplate{}, err
	}
	return fronteggJWTTemplate{
		Key:         d.Get("key").(string),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Expiration:  d.Get("expiration").(int),
		Algorithm:   d.Get("algorithm").(string),
		TemplateSchema: fronteggJWTTemplateSchema{
			Claims: claims,
		},
	}, nil
}

// resourceFronteggJWTTemplateClaimsDeserialize prefers claims_json when it is in use or a value is not a string.
func resourceFronteggJWTTemplateClaimsDeserialize(d *schema.ResourceData, claims map[string]interface{}) error {
	if d.Get("claims_json").(string) == "" && !hasNonStringClaim(claims) {
		return d.Set("claims", claims)
	}
	claimsJSON, err := structure.FlattenJsonToString(claims)
	if err != nil {
		return err
	}
	if err := d.Set("claims_json", claimsJSON); err != nil {
		return err
	}
	return d.Set("claims", nil)
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
	return resourceFronteggJWTTemplateClaimsDeserialize(d, t.TemplateSchema.Claims)
}

func resourceFronteggJWTTemplateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in, err := resourceFronteggJWTTemplateSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}
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
	in, err := resourceFronteggJWTTemplateSerialize(d)
	if err != nil {
		return diag.FromErr(err)
	}
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
