package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggPrehookPath = "/prehooks/resources/configurations/v1"
const fronteggPrehookCustomCodePath = fronteggPrehookPath + "/custom-code"
const fronteggCustomCodePath = "/custom-code/resources/codes/v1"

const (
	fronteggPrehookTypeAPI        = "API"
	fronteggPrehookTypeCustomCode = "CUSTOM_CODE"
)

const fronteggPrehookDefaultRuntime = "NODE_20"
const fronteggPrehookDefaultTimeout = 10

// A custom code prehook's executor (a serverless function) provisions
// asynchronously, so writes made immediately after creation can briefly return
// 5xx until it is ready. Retry those within this window.
const fronteggPrehookProvisionTimeout = 2 * time.Minute

type fronteggPrehook struct {
	ID                 string   `json:"id,omitempty"`
	Type               string   `json:"type,omitempty"`
	IsActive           bool     `json:"isActive"`
	DisplayName        string   `json:"displayName,omitempty"`
	Description        string   `json:"description,omitempty"`
	URL                string   `json:"url,omitempty"`
	Secret             string   `json:"secret,omitempty"`
	EventKeys          []string `json:"eventKeys,omitempty"`
	EventKey           string   `json:"eventKey,omitempty"`
	FailMethod         string   `json:"failMethod,omitempty"` // Can be "OPEN" or "CLOSE"
	Timeout            int      `json:"timeout,omitempty"`
	Runtime            string   `json:"runtime,omitempty"`
	Code               string   `json:"code,omitempty"`
	ExecutorIdentifier string   `json:"executorIdentifier,omitempty"`
}

type fronteggCustomCode struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

func resourceFronteggPrehook() *schema.Resource {
	return &schema.Resource{
		Description: `Configures a Frontegg prehook.

A prehook subscribes to an event and either sends it to an external URL (` + "`type = \"API\"`" + `) or runs
custom JavaScript hosted by Frontegg (` + "`type = \"CUSTOM_CODE\"`" + `). Frontegg allows only a single prehook
per event, regardless of type.`,
		CreateContext: resourceFronteggPrehookCreate,
		ReadContext:   resourceFronteggPrehookRead,
		UpdateContext: resourceFronteggPrehookUpdate,
		DeleteContext: resourceFronteggPrehookDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: resourceFronteggPrehookCustomizeDiff,

		Schema: map[string]*schema.Schema{
			"enabled": {
				Description: "Whether the prehook is enabled.",
				Type:        schema.TypeBool,
				Required:    true,
			},
			"name": {
				Description: "A human-readable name for the prehook.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"description": {
				Description: "A human-readable description of the prehook.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"type": {
				Description: "The prehook type. `API` sends events to `url`; `CUSTOM_CODE` runs `code` on Frontegg.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     fronteggPrehookTypeAPI,
				ForceNew:    true,
				ValidateFunc: validation.StringInSlice([]string{
					fronteggPrehookTypeAPI,
					fronteggPrehookTypeCustomCode,
				}, false),
			},
			"url": {
				Description: "The URL to send events to. Required when `type` is `API`.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"secret": {
				Description: "A secret to validate the event with. Required when `type` is `API`.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"code": {
				Description: "The JavaScript source that handles the event. It must define and export an `onEvent` handler. Required when `type` is `CUSTOM_CODE`.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"runtime": {
				Description: "The runtime to execute the code with (e.g. `NODE_20`). Only used when `type` is `CUSTOM_CODE`.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"timeout": {
				Description:  "The execution timeout in seconds (max 10). Only used when `type` is `CUSTOM_CODE`.",
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(1, 10),
			},
			"executor_identifier": {
				Description: "The identifier of the custom code executor backing this prehook.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"events": {
				Description: "The name of the event to subscribe to.",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: true,
			},
			"fail_method": {
				Description: "The action to take when the prehook fails.",
				Type:        schema.TypeString,
				Required:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"OPEN",
					"CLOSE",
				}, false),
			},
		},
	}
}

// fronteggFieldGetter is satisfied by both *schema.ResourceData and
// *schema.ResourceDiff, letting the same validation run at plan and in tests.
type fronteggFieldGetter interface {
	Get(key string) interface{}
}

func resourceFronteggPrehookCustomizeDiff(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
	return validateFronteggPrehookFields(d)
}

func validateFronteggPrehookFields(d fronteggFieldGetter) error {
	switch d.Get("type").(string) {
	case fronteggPrehookTypeCustomCode:
		if d.Get("code").(string) == "" {
			return fmt.Errorf("code is required when type is %s", fronteggPrehookTypeCustomCode)
		}
	default:
		if d.Get("url").(string) == "" {
			return fmt.Errorf("url is required when type is %s", fronteggPrehookTypeAPI)
		}
		if d.Get("secret").(string) == "" {
			return fmt.Errorf("secret is required when type is %s", fronteggPrehookTypeAPI)
		}
	}
	return nil
}

func resourceFronteggPrehookIsCustomCode(d *schema.ResourceData) bool {
	return d.Get("type").(string) == fronteggPrehookTypeCustomCode
}

// fronteggPrehookIsTransientError reports whether err is a retryable server-side
// failure, such as those returned while a custom code executor is provisioning.
func fronteggPrehookIsTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, status := range []string{
		"500 Internal Server Error",
		"502 Bad Gateway",
		"503 Service Unavailable",
		"504 Gateway Timeout",
	} {
		if strings.Contains(msg, status) {
			return true
		}
	}
	return false
}

// fronteggPrehookRetry runs fn, retrying transient server errors until the
// executor provisioning window elapses.
func fronteggPrehookRetry(ctx context.Context, fn func() error) error {
	return retry.RetryContext(ctx, fronteggPrehookProvisionTimeout, func() *retry.RetryError {
		err := fn()
		if err == nil {
			return nil
		}
		if fronteggPrehookIsTransientError(err) {
			return retry.RetryableError(err)
		}
		return retry.NonRetryableError(err)
	})
}

func resourceFronteggPrehookSerialize(d *schema.ResourceData) fronteggPrehook {
	events := stringSetToList(d.Get("events").(*schema.Set))
	prehook := fronteggPrehook{
		Type:        d.Get("type").(string),
		IsActive:    d.Get("enabled").(bool),
		DisplayName: d.Get("name").(string),
		Description: d.Get("description").(string),
		EventKeys:   events,
		FailMethod:  d.Get("fail_method").(string),
	}
	if prehook.Type == "" {
		prehook.Type = fronteggPrehookTypeAPI
	}

	if prehook.Type == fronteggPrehookTypeCustomCode {
		if len(events) > 0 {
			prehook.EventKey = events[0]
		}
		prehook.Code = d.Get("code").(string)
		prehook.Runtime = d.Get("runtime").(string)
		if prehook.Runtime == "" {
			prehook.Runtime = fronteggPrehookDefaultRuntime
		}
		prehook.Timeout = d.Get("timeout").(int)
		if prehook.Timeout == 0 {
			prehook.Timeout = fronteggPrehookDefaultTimeout
		}
	} else {
		prehook.URL = d.Get("url").(string)
		prehook.Secret = d.Get("secret").(string)
	}

	return prehook
}

func resourceFronteggPrehookDeserialize(d *schema.ResourceData, prehook fronteggPrehook, code *fronteggCustomCode) error {
	d.SetId(prehook.ID)

	prehookType := prehook.Type
	if prehookType == "" {
		prehookType = fronteggPrehookTypeAPI
	}
	if err := d.Set("type", prehookType); err != nil {
		return err
	}
	if err := d.Set("enabled", prehook.IsActive); err != nil {
		return err
	}
	if err := d.Set("name", prehook.DisplayName); err != nil {
		return err
	}
	if err := d.Set("description", prehook.Description); err != nil {
		return err
	}
	if err := d.Set("events", prehook.EventKeys); err != nil {
		return err
	}
	if err := d.Set("fail_method", prehook.FailMethod); err != nil {
		return err
	}

	if prehookType == fronteggPrehookTypeCustomCode {
		runtime := prehook.Runtime
		content := prehook.Code
		if code != nil {
			if code.Runtime != "" {
				runtime = code.Runtime
			}
			if code.Content != "" {
				content = code.Content
			}
		}
		if err := d.Set("runtime", runtime); err != nil {
			return err
		}
		if err := d.Set("timeout", prehook.Timeout); err != nil {
			return err
		}
		if err := d.Set("code", content); err != nil {
			return err
		}
		if err := d.Set("executor_identifier", prehook.ExecutorIdentifier); err != nil {
			return err
		}
	} else {
		if err := d.Set("url", prehook.URL); err != nil {
			return err
		}
		if err := d.Set("secret", prehook.Secret); err != nil {
			return err
		}
	}

	return nil
}

// resourceFronteggPrehookCheckEventConflict enforces Frontegg's rule that only a
// single prehook may exist per event. It returns an error if any existing prehook
// (other than excludeID) is already subscribed to one of the given events.
func resourceFronteggPrehookCheckEventConflict(ctx context.Context, clientHolder *restclient.ClientHolder, events []string, excludeID string) error {
	var existing []fronteggPrehook
	if err := clientHolder.ApiClient.Get(ctx, fronteggPrehookPath, &existing); err != nil {
		return err
	}
	wanted := make(map[string]struct{}, len(events))
	for _, e := range events {
		wanted[e] = struct{}{}
	}
	for _, p := range existing {
		if p.ID == excludeID {
			continue
		}
		for _, ek := range p.EventKeys {
			if _, ok := wanted[ek]; ok {
				return fmt.Errorf("a prehook already exists for event %q (id %s); Frontegg allows only one prehook per event", ek, p.ID)
			}
		}
	}
	return nil
}

// resourceFronteggPrehookFinalize writes the API response into state, fetching the
// custom code content (runtime + source) when needed, since the prehook create,
// update, and list responses carry only the executorIdentifier, not the code.
func resourceFronteggPrehookFinalize(ctx context.Context, clientHolder *restclient.ClientHolder, d *schema.ResourceData, out fronteggPrehook) diag.Diagnostics {
	if out.Type == "" {
		out.Type = d.Get("type").(string)
	}
	var code *fronteggCustomCode
	if out.Type == fronteggPrehookTypeCustomCode && out.ExecutorIdentifier != "" {
		var fetched fronteggCustomCode
		if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggCustomCodePath, out.ExecutorIdentifier), &fetched); err != nil {
			return diag.FromErr(err)
		}
		code = &fetched
	}
	if err := resourceFronteggPrehookDeserialize(d, out, code); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggPrehookCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggPrehookSerialize(d)

	if err := resourceFronteggPrehookCheckEventConflict(ctx, clientHolder, in.EventKeys, ""); err != nil {
		return diag.FromErr(err)
	}

	var out fronteggPrehook
	if resourceFronteggPrehookIsCustomCode(d) {
		in.ID = "create"
		if err := fronteggPrehookRetry(ctx, func() error {
			return clientHolder.ApiClient.Post(ctx, fronteggPrehookCustomCodePath, in, &out)
		}); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := clientHolder.ApiClient.Post(ctx, fronteggPrehookPath, in, &out); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceFronteggPrehookFinalize(ctx, clientHolder, d, out)
}

func resourceFronteggPrehookRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out []fronteggPrehook

	if err := clientHolder.ApiClient.Get(ctx, fronteggPrehookPath, &out); err != nil {
		return diag.FromErr(err)
	}

	for _, c := range out {
		if c.ID == d.Id() {
			var code *fronteggCustomCode
			if c.Type == fronteggPrehookTypeCustomCode && c.ExecutorIdentifier != "" {
				var fetched fronteggCustomCode
				if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggCustomCodePath, c.ExecutorIdentifier), &fetched); err != nil {
					return diag.FromErr(err)
				}
				code = &fetched
			}
			if err := resourceFronteggPrehookDeserialize(d, c, code); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggPrehookUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggPrehookSerialize(d)

	if d.HasChange("events") {
		if err := resourceFronteggPrehookCheckEventConflict(ctx, clientHolder, in.EventKeys, d.Id()); err != nil {
			return diag.FromErr(err)
		}
	}

	var out fronteggPrehook
	if resourceFronteggPrehookIsCustomCode(d) {
		if err := fronteggPrehookRetry(ctx, func() error {
			return clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPrehookCustomCodePath, d.Id()), in, &out)
		}); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggPrehookPath, d.Id()), in, &out); err != nil {
			return diag.FromErr(err)
		}
	}

	if out.ID == "" {
		out.ID = d.Id()
	}
	return resourceFronteggPrehookFinalize(ctx, clientHolder, d, out)
}

func resourceFronteggPrehookDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPrehookPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
