package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type fronteggSuperUser struct {
	SuperUser bool `json:"superUser"`
}

type fronteggUserRole struct {
	Id  string `json:"id,omitempty"`
	Key string `json:"key,omitempty"`
}

type fronteggUser struct {
	Key             string             `json:"id,omitempty"`
	Email           string             `json:"email,omitempty"`
	Password        string             `json:"password,omitempty"`
	CreateRoleIDs   []interface{}      `json:"roleIds,omitempty"`
	ReadRoleIDs     []fronteggUserRole `json:"roles,omitempty"`
	SkipInviteEmail bool               `json:"skipInviteEmail,omitempty"`
	Verified        bool               `json:"verified,omitempty"`
	SuperUser       bool               `json:"superUser,omitempty"`
}

type fronteggUserCreateRequest struct {
	fronteggUser
	EnvironmentRoles []fronteggEnvironmentRoles `json:"environmentRoles,omitempty"`
}

const fronteggUserPath = "/identity/resources/users/v2"
const fronteggUserPathV1 = "/identity/resources/users/v1"

const (
	fronteggErrNoApplicationID    = "ER-00008"
	fronteggErrTenantNotInAppCode = "ER-01008"
)

func fronteggUserApplicationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case strings.Contains(err.Error(), fronteggErrNoApplicationID):
		return fmt.Errorf(
			"creating a Frontegg user requires an application, and this environment has no default one. "+
				"Set `application_id` on the provider (or the FRONTEGG_APPLICATION_ID environment variable) "+
				"to the application the user should belong to.\n\noriginal error: %w",
			err,
		)
	case strings.Contains(err.Error(), fronteggErrTenantNotInAppCode):
		return fmt.Errorf(
			"the tenant is not assigned to the application named by `application_id`. "+
				"Assign it with a `frontegg_application_tenant_assignment` resource, and make the user "+
				"depend on that assignment so it is created first.\n\noriginal error: %w",
			err,
		)
	}
	return err
}

func resourceFronteggUser() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg user.

Global roles go in ` + "`role_ids`" + `. Roles that apply in a single environment go in
` + "`environment_role`" + ` blocks, which need the ` + "`env_role_assignment`" + ` feature flag enabled on the account.
Set at least one of the two. When ` + "`role_ids`" + ` is omitted, the provider leaves the user's global roles
unmanaged, such as the default roles the first user of a tenant gets. Manage a user's environment roles
either here or in ` + "`frontegg_user_env_role`" + `, not both.

Import this resource using the ` + "`tenant_id:user_id`" + ` format, since the
tenant cannot be recovered from the user ID alone. An import does not read environment roles.`,

		CreateContext: resourceFronteggUserCreate,
		ReadContext:   resourceFronteggUserRead,
		DeleteContext: resourceFronteggUserDelete,
		UpdateContext: resourceFronteggUserUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggUserImport,
		},

		Schema: map[string]*schema.Schema{
			"email": {
				Description: "The user's email address.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"password": {
				Description: "The user's login password.",
				Type:        schema.TypeString,
				Sensitive:   true,
				Optional:    true,
			},
			"skip_invite_email": {
				Description: "Skip sending the invite email. If true, user is automatically verified on creation.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"automatically_verify": {
				Description: "Whether the user gets verified upon creation.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
			"role_ids": {
				Description: "List of the role IDs that the user has in the tenant, in every environment.",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				MinItems:     1,
				Optional:     true,
				Computed:     true,
				AtLeastOneOf: []string{"role_ids", "environment_role"},
			},
			"environment_role": {
				Description: "A role the user has only in one environment.",
				Type:        schema.TypeSet,
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"environment_id": {
							Description:  "The ID of the environment the role applies in.",
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"role_id": {
							Description:  "The ID of the role.",
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
					},
				},
			},
			"tenant_id": {
				Description: "The tenant ID for this user.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"superuser": {
				Description: "Whether the user is a super user.",
				Type:        schema.TypeBool,
				Optional:    true,
			},
		},
	}
}

func resourceFronteggUserImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid import format, expected tenant_id:user_id, got: %s", d.Id())
	}
	if err := d.Set("tenant_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}

func resourceFronteggUserSerialize(d *schema.ResourceData) fronteggUserCreateRequest {
	log.Printf("role IDs: %#v", d.Get("role_ids").(*schema.Set).List())
	return fronteggUserCreateRequest{
		fronteggUser: fronteggUser{
			Email:           d.Get("email").(string),
			Password:        d.Get("password").(string),
			SkipInviteEmail: d.Get("skip_invite_email").(bool),
			CreateRoleIDs:   d.Get("role_ids").(*schema.Set).List(),
			SuperUser:       d.Get("superuser").(bool),
		},
		EnvironmentRoles: userRoleAssignmentsRequest(userRoleAssignmentsFromSet(d.Get("environment_role").(*schema.Set))).EnvironmentRoles,
	}
}

func resourceFronteggUserDeserialize(d *schema.ResourceData, f fronteggUser) error {
	d.SetId(f.Key)
	if err := d.Set("email", f.Email); err != nil {
		return err
	}
	var roleIDs []string
	for _, roleID := range f.ReadRoleIDs {
		roleIDs = append(roleIDs, roleID.Id)
	}
	if err := d.Set("role_ids", roleIDs); err != nil {
		return err
	}
	return nil
}

func resourceFronteggUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggUserSerialize(d)
	var out fronteggUser
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fronteggUserPath, headers, in, &out); err != nil {
		return diag.FromErr(fronteggUserApplicationError(err))
	}

	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	diags, err := warnUnassignedEnvironmentRoles(ctx, d, &clientHolder.ApiClient, out.Key)
	if err != nil {
		return diag.FromErr(err)
	}

	superUser := d.Get("superuser").(bool)
	if superUser {
		in := fronteggSuperUser{
			SuperUser: superUser,
		}
		if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/superuser", fronteggUserPathV1, out.Key), in, nil); err != nil {
			return append(diags, diag.FromErr(err)...)
		}
	}

	if !d.Get("automatically_verify").(bool) {
		return diags
	}
	if err := clientHolder.ApiClient.Post(ctx, fmt.Sprintf("%s/%s/verify", fronteggUserPathV1, out.Key), nil, nil); err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	return diags
}

// A failed create would taint the user and recreate it on every apply, so missing roles are a warning and the next refresh plans their retry.
func warnUnassignedEnvironmentRoles(ctx context.Context, d *schema.ResourceData, client *restclient.Client, userID string) (diag.Diagnostics, error) {
	requested := userRoleAssignmentsFromSet(d.Get("environment_role").(*schema.Set))
	missing, err := missingUserRoleAssignments(ctx, client, d.Get("tenant_id").(string), userID, requested)
	if err != nil || len(missing) == 0 {
		return nil, err
	}
	return diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  "Environment roles were not assigned",
		Detail:   missingUserRoleAssignmentsError(userID, missing).Error() + ". The next apply retries them.",
	}}, nil
}

func resourceFronteggUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	client := clientHolder.ApiClient
	client.Ignore404()
	var out fronteggUser
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := client.RequestWithHeaders(ctx, "GET", fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), headers, nil, &out); err != nil {
		return diag.FromErr(err)
	}
	if out.Key == "" {
		d.SetId("")
		return nil
	}

	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	if d.Get("environment_role").(*schema.Set).Len() == 0 {
		return nil
	}
	current, _, err := fetchUserRoleAssignments(ctx, &client, d.Get("tenant_id").(string), d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	environmentRoles := []interface{}{}
	for assignment := range current {
		if assignment.EnvironmentID != "" {
			environmentRoles = append(environmentRoles, assignment.block())
		}
	}
	if err := d.Set("environment_role", environmentRoles); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	// TODO: fields like phone number and avatar URL need https://docs.frontegg.com/reference/userscontrollerv1_updateuser

	// Email address:
	if d.HasChange("email") {
		email := d.Get("email").(string)
		if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/email", fronteggUserPathV1, d.Id()), struct {
			Email string `json:"email"`
		}{email}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("email", email); err != nil {
			return diag.FromErr(err)
		}
	}

	// Password:
	if d.HasChange("password") {
		headers := http.Header{}
		headers.Add("frontegg-user-id", d.Id())

		oldI, newI := d.GetChange("password")
		oldPw := oldI.(string)
		newPw := newI.(string)

		if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/passwords/change", fronteggUserPathV1), headers, struct {
			OldPW string `json:"password"`
			NewPW string `json:"newPassword"`
		}{oldPw, newPw}, nil); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("password", newPw); err != nil {
			return diag.FromErr(err)
		}
	}

	// Super User:
	if d.HasChange("superuser") {
		superUser := d.Get("superuser").(bool)
		in := fronteggSuperUser{
			SuperUser: superUser,
		}
		if err := clientHolder.ApiClient.Put(ctx, fmt.Sprintf("%s/%s/superuser", fronteggUserPathV1, d.Id()), in, nil); err != nil {
			return diag.FromErr(err)
		}
	}

	// Roles:
	if d.HasChange("role_ids") {
		headers := http.Header{}
		headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))

		oldsI, newsI := d.GetChange("role_ids")
		olds := oldsI.(*schema.Set)
		news := newsI.(*schema.Set)

		toAddSet := news.Difference(olds)
		toDelSet := olds.Difference(news)

		var toAdd, toDel []string

		for _, add := range toAddSet.List() {
			toAdd = append(toAdd, add.(string))
		}
		for _, del := range toDelSet.List() {
			toDel = append(toDel, del.(string))
		}

		if len(toAdd) > 0 {
			if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "POST", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
				RoleIds []string `json:"roleIds"`
			}{toAdd}, nil); err != nil {
				return diag.FromErr(err)
			}
		}
		if len(toDel) > 0 {
			if err := clientHolder.ApiClient.RequestWithHeaders(ctx, "DELETE", fmt.Sprintf("%s/%s/roles", fronteggUserPathV1, d.Id()), headers, struct {
				RoleIds []string `json:"roleIds"`
			}{toDel}, nil); err != nil {
				return diag.FromErr(err)
			}
		}

		if err := d.Set("role_ids", news); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("environment_role") {
		oldSet, newSet := d.GetChange("environment_role")
		if err := replaceUserRoleAssignments(ctx, &clientHolder.ApiClient, d.Get("tenant_id").(string), d.Id(), oldSet.(*schema.Set), newSet.(*schema.Set)); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(d.Id())
	return nil

}
