package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggPlanPath = "/entitlements/resources/plans/v1"

type fronteggPlan struct {
	ID                    string                   `json:"id,omitempty"`
	VendorID              string                   `json:"vendorId,omitempty"`
	Name                  string                   `json:"name"`
	DefaultTreatment      string                   `json:"defaultTreatment,omitempty"`
	Rules                 []map[string]interface{} `json:"rules,omitempty"`
	Description           string                   `json:"description,omitempty"`
	DefaultTimeLimitation int                      `json:"defaultTimeLimitation,omitempty"`
	AssignOnSignup        bool                     `json:"assignOnSignup"`
	CreatedAt             string                   `json:"createdAt,omitempty"`
	UpdatedAt             string                   `json:"updatedAt,omitempty"`
	FeatureKeys           []string                 `json:"featureKeys,omitempty"`
}

func resourceFronteggPlan() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg plan.`,

		CreateContext: resourceFronteggPlanCreate,
		ReadContext:   resourceFronteggPlanRead,
		UpdateContext: resourceFronteggPlanUpdate,
		DeleteContext: resourceFronteggPlanDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "The name of the plan.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"default_treatment": {
				Description: "The default treatment for the plan.",
				Type:        schema.TypeString,
				Optional:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"true",
					"false",
				}, false),
			},
			"rules": {
				Description: "Set of conditions targeting the plan.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			"description": {
				Description: "A description of the plan.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"default_time_limitation": {
				Description: "Default time limitation in days for auto-assigned plans.",
				Type:        schema.TypeInt,
				Optional:    true,
			},
			"assign_on_signup": {
				Description: "Whether the plan is assigned automatically upon signup.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"feature_keys": {
				Description: "Array of feature keys to be applied on the plan.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"vendor_id": {
				Description: "The vendor ID for the plan.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "When the plan was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "When the plan was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggPlanSerialize(d *schema.ResourceData) fronteggPlan {
	var rules []map[string]interface{}
	if v, ok := d.GetOk("rules"); ok {
		rawRules := v.([]interface{})
		rules = make([]map[string]interface{}, len(rawRules))
		for i, r := range rawRules {
			rules[i] = r.(map[string]interface{})
		}
	}

	var featureKeys []string
	if v, ok := d.GetOk("feature_keys"); ok {
		rawFeatureKeys := v.([]interface{})
		featureKeys = make([]string, len(rawFeatureKeys))
		for i, k := range rawFeatureKeys {
			featureKeys[i] = k.(string)
		}
	}

	return fronteggPlan{
		Name:                  d.Get("name").(string),
		DefaultTreatment:      d.Get("default_treatment").(string),
		Rules:                 rules,
		Description:           d.Get("description").(string),
		DefaultTimeLimitation: d.Get("default_time_limitation").(int),
		AssignOnSignup:        d.Get("assign_on_signup").(bool),
		FeatureKeys:           featureKeys,
	}
}

func resourceFronteggPlanDeserialize(d *schema.ResourceData, f fronteggPlan) error {
	d.SetId(f.ID)
	if err := d.Set("name", f.Name); err != nil {
		return err
	}
	if err := d.Set("default_treatment", f.DefaultTreatment); err != nil {
		return err
	}
	if err := d.Set("description", f.Description); err != nil {
		return err
	}
	if err := d.Set("default_time_limitation", f.DefaultTimeLimitation); err != nil {
		return err
	}
	if err := d.Set("assign_on_signup", f.AssignOnSignup); err != nil {
		return err
	}
	if err := d.Set("vendor_id", f.VendorID); err != nil {
		return err
	}
	if err := d.Set("created_at", f.CreatedAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", f.UpdatedAt); err != nil {
		return err
	}

	// Handle rules which is a slice of maps
	if f.Rules != nil {
		if err := d.Set("rules", f.Rules); err != nil {
			return err
		}
	}

	// Feature keys are not returned in the GET response format, so we need to
	// preserve the value that's in the state

	return nil
}

func resourceFronteggPlanCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := resourceFronteggPlanSerialize(d)

	// Check if a plan with this name already exists
	planName := d.Get("name").(string)
	existingPlan, err := fetchFronteggPlanByName(ctx, planName, clientHolder)
	if err != nil {
		return diag.FromErr(err)
	}

	// If plan already exists, set the ID and update it
	if existingPlan != nil {
		d.SetId(existingPlan.ID)
		return resourceFronteggPlanUpdate(ctx, d, meta)
	}

	var out fronteggPlan
	if err := clientHolder.ApiClient.Post(ctx, fronteggPlanPath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPlanDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func fetchFronteggPlan(ctx context.Context, planID string, clientHolder *restclient.ClientHolder) (*fronteggPlan, error) {
	var out fronteggPlan
	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggPlanPath, planID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchAllFronteggPlans(ctx context.Context, clientHolder *restclient.ClientHolder) ([]fronteggPlan, error) {
	// Response structure from entitlements API
	type pageResponse struct {
		Items   []fronteggPlan `json:"items"`
		HasNext bool           `json:"hasNext"`
	}

	var allPlans []fronteggPlan
	offset := 0
	limit := 10

	for {
		var response pageResponse
		url := fmt.Sprintf("%s?offset=%d&limit=%d", fronteggPlanPath, offset, limit)

		if err := clientHolder.ApiClient.Get(ctx, url, &response); err != nil {
			return nil, err
		}

		allPlans = append(allPlans, response.Items...)

		if !response.HasNext {
			break
		}

		offset += limit
	}

	return allPlans, nil
}

func fetchFronteggPlanByName(ctx context.Context, planName string, clientHolder *restclient.ClientHolder) (*fronteggPlan, error) {
	plans, err := fetchAllFronteggPlans(ctx, clientHolder)
	if err != nil {
		return nil, err
	}

	for _, plan := range plans {
		if plan.Name == planName {
			return &plan, nil
		}
	}

	return nil, nil // Plan not found
}

func resourceFronteggPlanRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	plan, err := fetchFronteggPlan(ctx, d.Id(), clientHolder)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPlanDeserialize(d, *plan); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggPlanUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggPlanSerialize(d)
	var out fronteggPlan
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPlanPath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggPlanDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggPlanDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPlanPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
