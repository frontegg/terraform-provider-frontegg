package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const fronteggUserRolesPathV3 = "/identity/resources/users/v3/roles"

type fronteggEnvironmentRoles struct {
	EnvironmentID string   `json:"environmentId"`
	RoleIDs       []string `json:"roleIds"`
}

type fronteggUserRoleAssignments struct {
	RoleIDs          []string                   `json:"roleIds,omitempty"`
	EnvironmentRoles []fronteggEnvironmentRoles `json:"environmentRoles,omitempty"`
}

type fronteggUserRoles struct {
	UserID           string                     `json:"userId"`
	RoleIDs          []string                   `json:"roleIds"`
	EnvironmentRoles []fronteggEnvironmentRoles `json:"environmentRoles"`
}

type userRoleAssignment struct {
	RoleID        string
	EnvironmentID string
}

func (a userRoleAssignment) block() map[string]interface{} {
	return map[string]interface{}{"role_id": a.RoleID, "environment_id": a.EnvironmentID}
}

func (a userRoleAssignment) String() string {
	if a.EnvironmentID == "" {
		return fmt.Sprintf("global role %s", a.RoleID)
	}
	return fmt.Sprintf("role %s in environment %s", a.RoleID, a.EnvironmentID)
}

func resourceFronteggUserEnvRole() *schema.Resource {
	return &schema.Resource{
		Description: `Assigns roles to a Frontegg user, either globally or scoped to a single environment.

Each ` + "`assignment`" + ` block with an ` + "`environment_id`" + ` gives the user the role only in that
environment. A block without ` + "`environment_id`" + ` assigns the role globally, in every environment.
Environment-scoped assignments need the ` + "`env_role_assignment`" + ` feature flag enabled on the account.

The resource manages only the assignments it declares. Destroying it removes those assignments and
leaves the user and its other roles in place. A global role assigned here also appears in the
` + "`role_ids`" + ` of a ` + "`frontegg_user`" + ` that manages the same user, so assign each global role in one place only.

Import this resource using the ` + "`tenant_id:user_id`" + ` format. An import reads every role the user has, so the
first apply after an import removes any of them the configuration does not declare.`,

		CreateContext: resourceFronteggUserEnvRoleCreate,
		ReadContext:   resourceFronteggUserEnvRoleRead,
		UpdateContext: resourceFronteggUserEnvRoleUpdate,
		DeleteContext: resourceFronteggUserEnvRoleDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggUserEnvRoleImport,
		},

		Schema: map[string]*schema.Schema{
			"tenant_id": {
				Description: "The ID of the tenant the user belongs to.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"user_id": {
				Description: "The ID of the user.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"assignment": {
				Description: "A role assigned to the user.",
				Type:        schema.TypeSet,
				Required:    true,
				MinItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_id": {
							Description: "The ID of the role.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"environment_id": {
							Description: "The ID of the environment the role applies in. Omit it to assign the role globally.",
							Type:        schema.TypeString,
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

func userRoleAssignmentsFromSet(set *schema.Set) []userRoleAssignment {
	assignments := []userRoleAssignment{}
	for _, item := range set.List() {
		block := item.(map[string]interface{})
		assignments = append(assignments, userRoleAssignment{
			RoleID:        block["role_id"].(string),
			EnvironmentID: block["environment_id"].(string),
		})
	}
	return assignments
}

func userRoleAssignmentsRequest(assignments []userRoleAssignment) fronteggUserRoleAssignments {
	request := fronteggUserRoleAssignments{}
	roleIDsByEnvironment := map[string][]string{}
	for _, assignment := range assignments {
		if assignment.EnvironmentID == "" {
			request.RoleIDs = append(request.RoleIDs, assignment.RoleID)
			continue
		}
		roleIDsByEnvironment[assignment.EnvironmentID] = append(roleIDsByEnvironment[assignment.EnvironmentID], assignment.RoleID)
	}
	environmentIDs := make([]string, 0, len(roleIDsByEnvironment))
	for environmentID := range roleIDsByEnvironment {
		environmentIDs = append(environmentIDs, environmentID)
	}
	sort.Strings(environmentIDs)
	for _, environmentID := range environmentIDs {
		request.EnvironmentRoles = append(request.EnvironmentRoles, fronteggEnvironmentRoles{
			EnvironmentID: environmentID,
			RoleIDs:       roleIDsByEnvironment[environmentID],
		})
	}
	return request
}

func userRolesPath(userID string) string {
	return fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, userID)
}

func tenantHeaders(tenantID string) http.Header {
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", tenantID)
	return headers
}

func sendUserRoleAssignments(ctx context.Context, client *restclient.Client, method, tenantID, userID string, assignments []userRoleAssignment) error {
	if len(assignments) == 0 {
		return nil
	}
	return client.RequestWithHeaders(ctx, method, userRolesPath(userID), tenantHeaders(tenantID), userRoleAssignmentsRequest(assignments), nil)
}

func fetchUserRoleAssignments(ctx context.Context, client *restclient.Client, tenantID, userID string) (map[userRoleAssignment]bool, bool, error) {
	var users []fronteggUserRoles
	path := fmt.Sprintf("%s?ids=%s", fronteggUserRolesPathV3, url.QueryEscape(userID))
	if err := client.RequestWithHeaders(ctx, http.MethodGet, path, tenantHeaders(tenantID), nil, &users); err != nil {
		return nil, false, err
	}
	for _, user := range users {
		if user.UserID != userID {
			continue
		}
		current := map[userRoleAssignment]bool{}
		for _, roleID := range user.RoleIDs {
			current[userRoleAssignment{RoleID: roleID}] = true
		}
		for _, environmentRoles := range user.EnvironmentRoles {
			for _, roleID := range environmentRoles.RoleIDs {
				current[userRoleAssignment{RoleID: roleID, EnvironmentID: environmentRoles.EnvironmentID}] = true
			}
		}
		return current, true, nil
	}
	return nil, false, nil
}

func assignUserRoles(ctx context.Context, client *restclient.Client, tenantID, userID string, assignments []userRoleAssignment) error {
	if err := sendUserRoleAssignments(ctx, client, http.MethodPost, tenantID, userID, assignments); err != nil {
		return err
	}
	if len(assignments) == 0 {
		return nil
	}
	current, _, err := fetchUserRoleAssignments(ctx, client, tenantID, userID)
	if err != nil {
		return err
	}
	missing := []string{}
	for _, assignment := range assignments {
		if !current[assignment] {
			missing = append(missing, assignment.String())
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"the Frontegg API accepted the request but did not assign %s to user %s. "+
				"Environment-scoped role assignments need the env_role_assignment feature flag enabled on the account",
			strings.Join(missing, ", "), userID,
		)
	}
	return nil
}

func resourceFronteggUserEnvRoleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.ClientHolder).ApiClient
	tenantID := d.Get("tenant_id").(string)
	userID := d.Get("user_id").(string)
	d.SetId(fmt.Sprintf("%s:%s", tenantID, userID))
	assignments := userRoleAssignmentsFromSet(d.Get("assignment").(*schema.Set))
	if err := assignUserRoles(ctx, &client, tenantID, userID, assignments); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserEnvRoleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.ClientHolder).ApiClient
	current, found, err := fetchUserRoleAssignments(ctx, &client, d.Get("tenant_id").(string), d.Get("user_id").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	if !found {
		d.SetId("")
		return nil
	}
	kept := []interface{}{}
	for _, assignment := range userRoleAssignmentsFromSet(d.Get("assignment").(*schema.Set)) {
		if current[assignment] {
			kept = append(kept, assignment.block())
		}
	}
	if err := d.Set("assignment", kept); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserEnvRoleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.ClientHolder).ApiClient
	tenantID := d.Get("tenant_id").(string)
	userID := d.Get("user_id").(string)
	oldSet, newSet := d.GetChange("assignment")
	removed := userRoleAssignmentsFromSet(oldSet.(*schema.Set).Difference(newSet.(*schema.Set)))
	added := userRoleAssignmentsFromSet(newSet.(*schema.Set).Difference(oldSet.(*schema.Set)))
	if err := sendUserRoleAssignments(ctx, &client, http.MethodDelete, tenantID, userID, removed); err != nil {
		return diag.FromErr(err)
	}
	if err := assignUserRoles(ctx, &client, tenantID, userID, added); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserEnvRoleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*restclient.ClientHolder).ApiClient
	client.Ignore404()
	assignments := userRoleAssignmentsFromSet(d.Get("assignment").(*schema.Set))
	if err := sendUserRoleAssignments(ctx, &client, http.MethodDelete, d.Get("tenant_id").(string), d.Get("user_id").(string), assignments); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserEnvRoleImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:user_id, got: %s", d.Id())
	}
	tenantID, userID := parts[0], parts[1]
	client := meta.(*restclient.ClientHolder).ApiClient
	current, found, err := fetchUserRoleAssignments(ctx, &client, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("user %s was not found in tenant %s", userID, tenantID)
	}
	blocks := []interface{}{}
	for assignment := range current {
		blocks = append(blocks, assignment.block())
	}
	if err := d.Set("tenant_id", tenantID); err != nil {
		return nil, err
	}
	if err := d.Set("user_id", userID); err != nil {
		return nil, err
	}
	if err := d.Set("assignment", blocks); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
