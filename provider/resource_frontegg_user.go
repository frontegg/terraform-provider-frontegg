package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	fronteggErrEmailInUse         = "ER-01200"
	fronteggErrUserExists         = "ER-01086"
	userCreateEmailRetries        = 20
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
	resource := &schema.Resource{
		Description: `Configures a Frontegg user.

Global roles go in ` + "`role_ids`" + `. Roles that apply in a single environment go in
` + "`environment_role`" + ` blocks, which need the ` + "`env_role_assignment`" + ` feature flag enabled on the account.
Set at least one of the two. When ` + "`role_ids`" + ` is omitted, the provider leaves the user's global roles
unmanaged, such as the default roles the first user of a tenant gets. Manage a user's environment roles
either here or in ` + "`frontegg_user_env_role`" + `, not both.

Destroying the resource, or changing ` + "`tenant_id`" + `, removes the user from its tenant only. Frontegg deletes
the user when it belongs to no other tenant. A user that is also in other tenants keeps its ID and password
when it moves to another tenant.

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
		CustomizeDiff: rejectUserPasswordRotation,
	}
	addCreateOnlyPassword(resource.Schema)
	resource.Schema["password"].Description += " When the current password is not in state, after an import or after using `password_wo`, " +
		"a new value is recorded but not sent, because Frontegg needs the current password to change it."
	return resource
}

func addCreateOnlyPassword(attrs map[string]*schema.Schema) {
	addWriteOnlySecret(attrs, "password")
	attrs["password_wo"].Description = "Write-only alternative to `password` that is never stored in the plan or state. Accepts ephemeral values. " +
		"Only sent when the user is created. Requires Terraform 1.11 or later and must be set together with `password_wo_version`."
	attrs["password_wo_version"].Description = "Version of `password_wo`. Changing it after the user is created fails at plan, because Frontegg " +
		"needs the current password to change it. Change the password in Frontegg instead. Adding `password_wo` to an existing user, " +
		"including after removing it, never changes the password."
}

func rejectUserPasswordRotation(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
	oldVersion, newVersion := d.GetChange("password_wo_version")
	if d.Id() == "" || oldVersion.(int) == 0 || newVersion.(int) == 0 || oldVersion == newVersion {
		return nil
	}
	return fmt.Errorf(
		"password_wo is only sent when the user is created: Frontegg needs the current password to change it, "+
			"and a write-only password is not kept. Change the password in Frontegg and set password_wo_version back to %d",
		oldVersion,
	)
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
			Password:        secretValue(d, "password"),
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

var userCreateEmailRetryDelay = 500 * time.Millisecond

func createFronteggUser(ctx context.Context, client *restclient.Client, headers http.Header, in interface{}, out *fronteggUser) error {
	for attempt := 1; ; attempt++ {
		err := client.RequestWithHeaders(ctx, "POST", fronteggUserPath, headers, in, out)
		if err == nil || !strings.Contains(err.Error(), fronteggErrEmailInUse) || attempt == userCreateEmailRetries {
			return err
		}
		log.Printf("[WARN] email still in use, as after a recent delete; retrying user create (%d/%d)", attempt, userCreateEmailRetries)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(userCreateEmailRetryDelay):
		}
	}
}

func fronteggUserExistsError(ctx context.Context, client *restclient.Client, tenantID, email string, err error) error {
	if !strings.Contains(err.Error(), fronteggErrUserExists) {
		return err
	}
	importID := tenantID + ":<user_id>"
	var existing fronteggUser
	if lookupErr := client.Get(ctx, fmt.Sprintf("%s/email?email=%s", fronteggUserPathV1, url.QueryEscape(email)), &existing); lookupErr == nil && existing.Key != "" {
		importID = tenantID + ":" + existing.Key
	}
	return fmt.Errorf(
		"user %s already exists in tenant %s. Import it with the ID %s instead of creating it, "+
			"or remove it from the tenant first.\n\noriginal error: %w",
		email, tenantID, importID, err,
	)
}

func resourceFronteggUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggUserSerialize(d)
	var out fronteggUser
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := createFronteggUser(ctx, &clientHolder.ApiClient, headers, in, &out); err != nil {
		err = fronteggUserExistsError(ctx, &clientHolder.ApiClient, d.Get("tenant_id").(string), in.Email, err)
		return diag.FromErr(fronteggUserApplicationError(err))
	}

	if err := resourceFronteggUserDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}

	requestedRoles := append(globalRoleAssignments(in.CreateRoleIDs), userRoleAssignmentsFromSet(d.Get("environment_role").(*schema.Set))...)
	diags := unassignedRolesWarning(ctx, &clientHolder.ApiClient, d.Get("tenant_id").(string), out.Key, requestedRoles)

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

func globalRoleAssignments(roleIDs []interface{}) []userRoleAssignment {
	assignments := []userRoleAssignment{}
	for _, roleID := range roleIDs {
		assignments = append(assignments, userRoleAssignment{RoleID: roleID.(string)})
	}
	return assignments
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
	client := meta.(*restclient.ClientHolder).ApiClient
	client.Ignore404()
	headers := http.Header{}
	headers.Add("frontegg-tenant-id", d.Get("tenant_id").(string))
	if err := client.DeleteWithHeaders(ctx, fmt.Sprintf("%s/%s", fronteggUserPathV1, d.Id()), headers, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func updateUserRoles(ctx context.Context, client *restclient.Client, d *schema.ResourceData) error {
	tenantID := d.Get("tenant_id").(string)
	oldRoleIDs, newRoleIDs := d.GetChange("role_ids")
	oldEnvironmentRoles, newEnvironmentRoles := d.GetChange("environment_role")
	added := append(
		globalRoleAssignments(newRoleIDs.(*schema.Set).Difference(oldRoleIDs.(*schema.Set)).List()),
		userRoleAssignmentsFromSet(newEnvironmentRoles.(*schema.Set).Difference(oldEnvironmentRoles.(*schema.Set)))...,
	)
	removed := append(
		globalRoleAssignments(oldRoleIDs.(*schema.Set).Difference(newRoleIDs.(*schema.Set)).List()),
		userRoleAssignmentsFromSet(oldEnvironmentRoles.(*schema.Set).Difference(newEnvironmentRoles.(*schema.Set)))...,
	)
	if err := assignUserRoles(ctx, client, tenantID, d.Id(), added); err != nil {
		return err
	}
	if err := sendUserRoleAssignments(ctx, client, http.MethodDelete, tenantID, d.Id(), removed); err != nil {
		return err
	}
	return d.Set("role_ids", newRoleIDs)
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
	oldI, newI := d.GetChange("password")
	oldPw := oldI.(string)
	newPw := newI.(string)
	if d.HasChange("password") && oldPw != "" && newPw != "" {
		headers := http.Header{}
		headers.Add("frontegg-user-id", d.Id())

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

	if d.HasChanges("role_ids", "environment_role") {
		if err := updateUserRoles(ctx, &clientHolder.ApiClient, d); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(d.Id())
	return nil

}
