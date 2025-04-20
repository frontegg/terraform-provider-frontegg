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

	// Use plan ID directly as the resource ID
	planID := d.Id()

	// Set the plan ID in the state
	if err := d.Set("plan_id", planID); err != nil {
		return diag.FromErr(err)
	}

	// Get the list of features for the plan - using only plan ID as per OpenAPI spec
	var features struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}

	if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("/entitlements/resources/plans/v1/%s/features", planID), &features); err != nil {
		return diag.FromErr(err)
	}

	// Get the current feature IDs from the state
	featureIDsSet := d.Get("feature_ids").(*schema.Set)
	stateFeatureIDs := make([]string, 0, featureIDsSet.Len())
	for _, v := range featureIDsSet.List() {
		stateFeatureIDs = append(stateFeatureIDs, v.(string))
	}

	// Create a map of feature IDs from the API response
	existingFeatureIDs := make(map[string]bool)
	for _, feature := range features.Items {
		existingFeatureIDs[feature.ID] = true
	}

	// Check if all the features exist in the list
	allFeaturesFound := true
	for _, featureID := range stateFeatureIDs {
		if !existingFeatureIDs[featureID] {
			allFeaturesFound = false
			break
		}
	}

	// If not all features are found, mark the resource as gone
	if !allFeaturesFound {
		d.SetId("")
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
		// Check if the error is a 404 with "Feature Bundle not found" message
		if err.Error() != "" && strings.Contains(err.Error(), "Feature Bundle not found") {
			// This is the specific 404 error we want to treat as success
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
