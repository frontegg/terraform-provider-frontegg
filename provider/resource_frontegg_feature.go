package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggFeaturePathPrefix = "/entitlements/resources/features"
const fronteggFeaturePathV1 = fronteggFeaturePathPrefix + "/v1"
const fronteggFeaturePathV2 = fronteggFeaturePathPrefix + "/v2"

type fronteggFeature struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Key         string                 `json:"key"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Description string                 `json:"description,omitempty"`
	CreatedAt   string                 `json:"createdAt,omitempty"`
	UpdatedAt   string                 `json:"updatedAt,omitempty"`
	FeatureFlag *featureFlagThin       `json:"featureFlag,omitempty"`
}

type fronteggFeatureV2 struct {
	fronteggFeature
	Permissions []permissionObject `json:"permissions,omitempty"`
}

type fronteggFeatureV1 struct {
	fronteggFeature
	Permissions []string `json:"permissions,omitempty"`
}

type permissionObject struct {
	PermissionKey string `json:"permissionKey"`
	PermissionID  string `json:"permissionId"`
}

type featureFlagThin struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	On               bool   `json:"on"`
	OffTreatment     string `json:"offTreatment"`
	DefaultTreatment string `json:"defaultTreatment"`
	Description      string `json:"description,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	CreatedAt        string `json:"createdAt,omitempty"`
}

func resourceFronteggFeature() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg feature.`,

		CreateContext: resourceFronteggFeatureCreate,
		ReadContext:   resourceFronteggFeatureRead,
		UpdateContext: resourceFronteggFeatureUpdate,
		DeleteContext: resourceFronteggFeatureDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID of the feature (UUID).",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"name": {
				Description: "The name of the feature.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "The key of the feature.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "A description of the feature.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"permissions": {
				Description: "The permissions for the feature.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"permission_key": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The key of the permission",
						},
						"permission_id": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The ID of the permission",
						},
					},
				},
			},
			"metadata": {
				Description: "Metadata for the feature.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"created_at": {
				Description: "When the feature was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "When the feature was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggFeatureSerialize(d *schema.ResourceData) fronteggFeatureV2 {
	feature := fronteggFeatureV2{
		fronteggFeature: fronteggFeature{
			Name:        d.Get("name").(string),
			Key:         d.Get("key").(string),
			Description: d.Get("description").(string),
		},
	}

	// Handle permissions
	if permissions, ok := d.GetOk("permissions"); ok {
		permissionsList := permissions.([]interface{})
		feature.Permissions = make([]permissionObject, len(permissionsList))
		for i, perm := range permissionsList {
			permObj := perm.(map[string]interface{})
			feature.Permissions[i] = permissionObject{
				PermissionKey: permObj["permission_key"].(string),
				PermissionID:  permObj["permission_id"].(string),
			}
		}
		// Sort permissions by key for consistent ordering
		sort.Slice(feature.Permissions, func(i, j int) bool {
			return feature.Permissions[i].PermissionKey < feature.Permissions[j].PermissionKey
		})
	}

	// Handle metadata
	if metadata, ok := d.GetOk("metadata"); ok {
		metadataStr := metadata.(string)
		if metadataStr != "" {
			feature.Metadata = make(map[string]interface{})
		}
	}

	return feature
}

func resourceFronteggFeatureDeserializeCommon(d *schema.ResourceData, name, key, description, createdAt, updatedAt string) error {
	d.SetId(d.Id())
	if err := d.Set("name", name); err != nil {
		return err
	}
	if err := d.Set("key", key); err != nil {
		return err
	}
	if err := d.Set("description", description); err != nil {
		return err
	}
	if err := d.Set("created_at", createdAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", updatedAt); err != nil {
		return err
	}
	return nil
}

// getPermissionsData fetches all permissions and returns a map of key to permission.
func getPermissionsData(ctx context.Context, client *restclient.ClientHolder) (map[string]fronteggPermission, error) {
	var permissions []fronteggPermission
	if err := client.ApiClient.Get(ctx, fronteggPermissionPath, &permissions); err != nil {
		return nil, err
	}

	// Create a map of permission key to permission data
	permissionsMap := make(map[string]fronteggPermission)
	for _, p := range permissions {
		permissionsMap[p.Key] = p
	}
	return permissionsMap, nil
}

func resourceFronteggFeatureDeserializeV1(d *schema.ResourceData, f fronteggFeatureV1, client *restclient.ClientHolder, ctx context.Context) error {
	if err := resourceFronteggFeatureDeserializeCommon(d, f.Name, f.Key, f.Description, f.CreatedAt, f.UpdatedAt); err != nil {
		return err
	}

	// Handle permissions
	if len(f.Permissions) > 0 {
		// Get current permissions data
		permissionsMap, err := getPermissionsData(ctx, client)
		if err != nil {
			return err
		}

		// Get existing permissions from resource to maintain order
		existingPerms := d.Get("permissions").([]interface{})
		permissions := make([]map[string]interface{}, len(f.Permissions))
		for i, perm := range existingPerms {
			permMap := perm.(map[string]interface{})
			//add only if exists in f.Permissions
			if _, ok := permissionsMap[permMap["permission_key"].(string)]; ok {
				permissions[i] = permMap
			}
		}

		if err := d.Set("permissions", permissions); err != nil {
			return err
		}
	}

	// Handle metadata
	if f.Metadata != nil {
		if err := d.Set("metadata", f.Metadata); err != nil {
			return err
		}
	}

	return nil
}

// findFeatureByKey attempts to find a feature by its key.
func findFeatureByKey(ctx context.Context, client *restclient.ClientHolder, key string) (*fronteggFeatureV1, error) {
	type pageResponse struct {
		Items   []fronteggFeatureV1 `json:"items"`
		HasNext bool                `json:"hasNext"`
	}

	url := fmt.Sprintf("%s?key=%s&limit=1", fronteggFeaturePathV1, key)

	var searchResult pageResponse
	if err := client.ApiClient.Get(ctx, url, &searchResult); err != nil {
		return nil, err
	}

	if len(searchResult.Items) == 0 {
		return nil, nil
	}

	return &searchResult.Items[0], nil
}

func resourceFronteggFeatureCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	key := d.Get("key").(string)

	// Check if feature exists
	existingFeature, err := findFeatureByKey(ctx, clientHolder, key)
	if err != nil {
		return diag.FromErr(err)
	}

	// If feature exists, update it
	if existingFeature != nil {
		d.SetId(existingFeature.ID)

		// Update the existing feature
		in := resourceFronteggFeatureSerialize(d)
		if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggFeaturePathV2, existingFeature.ID), in, nil); err != nil {
			return diag.FromErr(err)
		}

		return nil
	}

	// Feature doesn't exist, create new one
	in := resourceFronteggFeatureSerialize(d)
	var out fronteggFeatureV1
	if err := clientHolder.ApiClient.Post(ctx, fronteggFeaturePathV2, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggFeatureDeserializeV1(d, out, clientHolder, ctx); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggFeatureRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	// Create a struct to hold the paginated response
	type pageResponse struct {
		Items   []fronteggFeatureV1 `json:"items"`
		HasNext bool                `json:"hasNext"`
	}

	// Build the URL with query parameters
	url := fmt.Sprintf("%s?featureIds=%s&limit=1", fronteggFeaturePathV1, d.Id())

	var out pageResponse
	if err := clientHolder.ApiClient.Get(ctx, url, &out); err != nil {
		return diag.FromErr(err)
	}

	// Check if we found the feature
	if len(out.Items) == 0 {
		return diag.Errorf("Feature with ID %s not found", d.Id())
	}

	// Deserialize the first (and should be only) item
	if err := resourceFronteggFeatureDeserializeV1(d, out.Items[0], clientHolder, ctx); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggFeatureUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	in := resourceFronteggFeatureSerialize(d)
	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggFeaturePathV2, d.Id()), in, nil); err != nil {
		return diag.FromErr(err)
	}

	// Refresh state by reading the resource after update
	return resourceFronteggFeatureRead(ctx, d, meta)
}

func resourceFronteggFeatureDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggFeaturePathV1, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
