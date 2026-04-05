package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceFronteggPlanFeature() *schema.Resource {
	return &schema.Resource{
		Description: `Links features to a Frontegg plan.`,

		CreateContext: resourceFronteggPlanFeatureCreate,
		ReadContext:   resourceFronteggPlanFeatureRead,
		DeleteContext: resourceFronteggPlanFeatureDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"plan_id": {
				Description: "The ID of the plan.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"feature_ids": {
				Description: "The IDs of the features to link to the plan.",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceFronteggPlanFeatureCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	planID := d.Get("plan_id").(string)
	featureIDsSet := d.Get("feature_ids").(*schema.Set)
	featureIDs := make([]string, 0, featureIDsSet.Len())

	for _, v := range featureIDsSet.List() {
		featureIDs = append(featureIDs, v.(string))
	}

	// API expects an array of feature IDs
	in := struct {
		FeatureIds []string `json:"featuresIds"`
	}{
		FeatureIds: featureIDs,
	}

	// Link the features to the plan
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("/entitlements/resources/plans/v1/%s/features/link", planID), in, nil); err != nil {
		return diag.FromErr(err)
	}

	// Set only the plan ID as the resource ID
	d.SetId(planID)

	return nil
}

func resourceFronteggPlanFeatureRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	planID := d.Id()

	if err := d.Set("plan_id", planID); err != nil {
		return diag.FromErr(err)
	}

	type pageResponse struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		HasNext bool `json:"hasNext"`
	}

	var allFeatureIDs []string
	offset := 0
	limit := 10

	for {
		var response pageResponse
		url := fmt.Sprintf("/entitlements/resources/plans/v1/%s/features?offset=%d&limit=%d", planID, offset, limit)
		if err := clientHolder.ApiClient.Get(ctx, url, &response); err != nil {
			return diag.FromErr(err)
		}
		for _, f := range response.Items {
			allFeatureIDs = append(allFeatureIDs, f.ID)
		}
		if !response.HasNext {
			break
		}
		offset += limit
	}

	if len(allFeatureIDs) == 0 {
		d.SetId("")
		return nil
	}

	if err := d.Set("feature_ids", allFeatureIDs); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggPlanFeatureDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	// Use plan ID directly as the resource ID
	planID := d.Id()

	// Get feature IDs from the state
	featureIDsSet := d.Get("feature_ids").(*schema.Set)
	featureIDs := make([]string, 0, featureIDsSet.Len())
	for _, v := range featureIDsSet.List() {
		featureIDs = append(featureIDs, v.(string))
	}

	// API expects an array of feature IDs
	in := struct {
		FeatureIds []string `json:"featuresIds"`
	}{
		FeatureIds: featureIDs,
	}

	// Unlink the features from the plan
	err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("/entitlements/resources/plans/v1/%s/features/unlink", planID), in, nil)
	if err != nil {
		if err.Error() != "" && strings.Contains(err.Error(), "Feature Bundle not found") {
			return nil
		}
		return diag.FromErr(err)
	}

	return nil
}

// These functions are no longer needed since we're using only the plan ID
// Keeping them commented out in case they need to be referenced later
/*
// formatPlanFeaturesID creates a composite ID in the format planID:featureID1,featureID2,...
func formatPlanFeaturesID(planID string, featureIDs []string) string {
	return fmt.Sprintf("%s:%s", planID, strings.Join(featureIDs, ","))
}

// parsePlanFeaturesID parses the composite ID (planID:featureID1,featureID2,...)
func parsePlanFeaturesID(id string) (string, []string, error) {
	// Split the ID by colon
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid ID format: %s (expected plan_id:feature_id1,feature_id2,...)", id)
	}

	planID := parts[0]
	featureIDs := strings.Split(parts[1], ",")

	return planID, featureIDs, nil
}
*/
