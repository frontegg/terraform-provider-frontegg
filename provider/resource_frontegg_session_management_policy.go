package provider

import (
	"context"
	"log"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggSessionManagementPolicyURL = "/identity/resources/configurations/sessions/v1/vendor"

// sessionManagementPolicyID is the synthetic ID for this singleton resource.
const sessionManagementPolicyID = "session-management"

type fronteggSessionTimeoutConfig struct {
	IsActive bool `json:"isActive"`
	Timeout  int  `json:"timeout"`
}

type fronteggSessionConcurrentConfig struct {
	IsActive    bool `json:"isActive"`
	MaxSessions int  `json:"maxSessions"`
}

type fronteggSessionManagementPolicy struct {
	SessionIdleTimeoutConfiguration fronteggSessionTimeoutConfig    `json:"sessionIdleTimeoutConfiguration"`
	SessionTimeoutConfiguration     fronteggSessionTimeoutConfig    `json:"sessionTimeoutConfiguration"`
	SessionConcurrentConfiguration  fronteggSessionConcurrentConfig `json:"sessionConcurrentConfiguration"`
}

func resourceFronteggSessionManagementPolicy() *schema.Resource {
	return &schema.Resource{
		Description: `Configures the vendor (environment) default session management policy.

These settings appear in the Frontegg portal under Configurations → Security → Session Management.

This is a singleton resource. You must only create one frontegg_session_management_policy resource
per Frontegg provider.

**Note:** This resource cannot be deleted. When destroyed, Terraform will remove it from the state file, but the session management policy will remain in its last-applied state.`,

		CreateContext: resourceFronteggSessionManagementPolicyCreate,
		ReadContext:   resourceFronteggSessionManagementPolicyRead,
		UpdateContext: resourceFronteggSessionManagementPolicyUpdate,
		DeleteContext: resourceFronteggSessionManagementPolicyDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"idle_session_timeout_enabled": {
				Description: "Whether the idle session timeout is enforced. When disabled, the platform default of 24 hours applies.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"idle_session_timeout": {
				Description:  "How long, in seconds, a session may remain idle before it ends automatically. Only enforced when `idle_session_timeout_enabled` is true. For example, 86400 is 24 hours.",
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      86400,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"force_relogin_enabled": {
				Description: "Whether a maximum session duration (force re-login) is enforced. When disabled, sessions last indefinitely (subject to token expiration).",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"force_relogin_timeout": {
				Description:  "The maximum session duration, in seconds, regardless of activity, after which the user must log in again. Only enforced when `force_relogin_enabled` is true. For example, 604800 is 7 days.",
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      86400,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"max_concurrent_sessions_enabled": {
				Description: "Whether the number of concurrent sessions per user is limited. When disabled, the number of concurrent sessions is unlimited.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"max_concurrent_sessions": {
				Description:  "The maximum number of concurrent sessions a user may have. When exceeded, the oldest session is terminated. Only enforced when `max_concurrent_sessions_enabled` is true (recommended: 1-10).",
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1,
				ValidateFunc: validation.IntAtLeast(1),
			},
		},
	}
}

func resourceFronteggSessionManagementPolicySerialize(d *schema.ResourceData) fronteggSessionManagementPolicy {
	return fronteggSessionManagementPolicy{
		SessionIdleTimeoutConfiguration: fronteggSessionTimeoutConfig{
			IsActive: d.Get("idle_session_timeout_enabled").(bool),
			Timeout:  d.Get("idle_session_timeout").(int),
		},
		SessionTimeoutConfiguration: fronteggSessionTimeoutConfig{
			IsActive: d.Get("force_relogin_enabled").(bool),
			Timeout:  d.Get("force_relogin_timeout").(int),
		},
		SessionConcurrentConfiguration: fronteggSessionConcurrentConfig{
			IsActive:    d.Get("max_concurrent_sessions_enabled").(bool),
			MaxSessions: d.Get("max_concurrent_sessions").(int),
		},
	}
}

func resourceFronteggSessionManagementPolicyDeserialize(d *schema.ResourceData, out fronteggSessionManagementPolicy) error {
	d.SetId(sessionManagementPolicyID)
	if err := d.Set("idle_session_timeout_enabled", out.SessionIdleTimeoutConfiguration.IsActive); err != nil {
		return err
	}
	// Only sync the duration/count when the feature is active, so a disabled
	// feature's stored value does not produce perpetual plan diffs against the
	// configured default.
	if out.SessionIdleTimeoutConfiguration.IsActive {
		if err := d.Set("idle_session_timeout", out.SessionIdleTimeoutConfiguration.Timeout); err != nil {
			return err
		}
	}
	if err := d.Set("force_relogin_enabled", out.SessionTimeoutConfiguration.IsActive); err != nil {
		return err
	}
	if out.SessionTimeoutConfiguration.IsActive {
		if err := d.Set("force_relogin_timeout", out.SessionTimeoutConfiguration.Timeout); err != nil {
			return err
		}
	}
	if err := d.Set("max_concurrent_sessions_enabled", out.SessionConcurrentConfiguration.IsActive); err != nil {
		return err
	}
	if out.SessionConcurrentConfiguration.IsActive {
		if err := d.Set("max_concurrent_sessions", out.SessionConcurrentConfiguration.MaxSessions); err != nil {
			return err
		}
	}
	return nil
}

func resourceFronteggSessionManagementPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceFronteggSessionManagementPolicyUpdate(ctx, d, meta)
}

func resourceFronteggSessionManagementPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	var out fronteggSessionManagementPolicy
	if err := clientHolder.ApiClient.Get(ctx, fronteggSessionManagementPolicyURL, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggSessionManagementPolicyDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggSessionManagementPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	in := resourceFronteggSessionManagementPolicySerialize(d)
	if err := clientHolder.ApiClient.Post(ctx, fronteggSessionManagementPolicyURL, in, nil); err != nil {
		return diag.FromErr(err)
	}
	return resourceFronteggSessionManagementPolicyRead(ctx, d, meta)
}

func resourceFronteggSessionManagementPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[WARN] Cannot destroy session management policy. Terraform will remove this resource from the " +
		"state file, but the session management policy will remain in its last-applied state.")
	return nil
}
