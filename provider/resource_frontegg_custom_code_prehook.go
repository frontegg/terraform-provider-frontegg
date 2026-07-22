package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggCustomCodePrehookPath = "/prehooks/resources/configurations/v1/custom-code"
const fronteggCustomCodePath = "/custom-code/resources/codes/v1"

type fronteggCustomCodePrehook struct {
	ID                 string   `json:"id,omitempty"`
	Type               string   `json:"type,omitempty"`
	IsActive           bool     `json:"isActive"`
	DisplayName        string   `json:"displayName,omitempty"`
	Description        string   `json:"description,omitempty"`
	EventKeys          []string `json:"eventKeys,omitempty"`
	EventKey           string   `json:"eventKey,omitempty"`
	FailMethod         string   `json:"failMethod,omitempty"`
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

func resourceFronteggCustomCodePrehook() *schema.Resource {
	return &schema.Resource{
		Description:   `Configures a Frontegg custom code prehook that runs JavaScript on Frontegg's side.`,
		CreateContext: resourceFronteggCustomCodePrehookCreate,
		ReadContext:   resourceFronteggCustomCodePrehookRead,
		UpdateContext: resourceFronteggCustomCodePrehookUpdate,
		DeleteContext: resourceFronteggCustomCodePrehookDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

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
				Optional:    true,
			},
			"code": {
				Description: "The JavaScript source that handles the event. It must define and export an `onEvent` handler.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"runtime": {
				Description: "The runtime to execute the code with (e.g. `NODE_20`).",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "NODE_20",
			},
			"timeout": {
				Description: "The execution timeout in seconds.",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     10,
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
			"executor_identifier": {
				Description: "The identifier of the custom code executor backing this prehook.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggCustomCodePrehookSerialize(d *schema.ResourceData, id string) fronteggCustomCodePrehook {
	events := stringSetToList(d.Get("events").(*schema.Set))
	eventKey := ""
	if len(events) > 0 {
		eventKey = events[0]
	}
	return fronteggCustomCodePrehook{
		ID:                 id,
		Type:               "CUSTOM_CODE",
		IsActive:           d.Get("enabled").(bool),
		DisplayName:        d.Get("name").(string),
		Description:        d.Get("description").(string),
		EventKeys:          events,
		EventKey:           eventKey,
		FailMethod:         d.Get("fail_method").(string),
		Timeout:            d.Get("timeout").(int),
		Runtime:            d.Get("runtime").(string),
		Code:               d.Get("code").(string),
		ExecutorIdentifier: d.Get("executor_identifier").(string),
	}
}

func resourceFronteggCustomCodePrehookDeserialize(d *schema.ResourceData, prehook fronteggCustomCodePrehook, code *fronteggCustomCode) error {
	d.SetId(prehook.ID)

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
	if prehook.Timeout != 0 {
		if err := d.Set("timeout", prehook.Timeout); err != nil {
			return err
		}
	}
	runtime := prehook.Runtime
	if code != nil && code.Runtime != "" {
		runtime = code.Runtime
	}
	if runtime != "" {
		if err := d.Set("runtime", runtime); err != nil {
			return err
		}
	}
	content := prehook.Code
	if code != nil && code.Content != "" {
		content = code.Content
	}
	if content != "" {
		if err := d.Set("code", content); err != nil {
			return err
		}
	}
	if prehook.ExecutorIdentifier != "" {
		if err := d.Set("executor_identifier", prehook.ExecutorIdentifier); err != nil {
			return err
		}
	}
	return nil
}

func resourceFronteggCustomCodePrehookCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggCustomCodePrehookSerialize(d, "create")
	var out fronteggCustomCodePrehook

	if err := clientHolder.ApiClient.Post(ctx, fronteggCustomCodePrehookPath, in, &out); err != nil {
		return diag.FromErr(err)
	}

	if err := resourceFronteggCustomCodePrehookDeserialize(d, out, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggCustomCodePrehookRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out []fronteggCustomCodePrehook

	if err := clientHolder.ApiClient.Get(ctx, fronteggPrehookPath, &out); err != nil {
		return diag.FromErr(err)
	}

	for _, c := range out {
		if c.ID == d.Id() {
			var code *fronteggCustomCode
			if c.ExecutorIdentifier != "" {
				var fetched fronteggCustomCode
				if err := clientHolder.ApiClient.Get(ctx, fmt.Sprintf("%s/%s", fronteggCustomCodePath, c.ExecutorIdentifier), &fetched); err != nil {
					return diag.FromErr(err)
				}
				code = &fetched
			}
			if err := resourceFronteggCustomCodePrehookDeserialize(d, c, code); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}
	d.SetId("")
	return nil
}

func resourceFronteggCustomCodePrehookUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggCustomCodePrehookSerialize(d, "")
	var out fronteggCustomCodePrehook

	if err := clientHolder.ApiClient.Patch(ctx, fmt.Sprintf("%s/%s", fronteggCustomCodePrehookPath, d.Id()), in, &out); err != nil {
		return diag.FromErr(err)
	}

	if out.ID == "" {
		out.ID = d.Id()
	}
	if err := resourceFronteggCustomCodePrehookDeserialize(d, out, nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggCustomCodePrehookDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)

	if err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggPrehookPath, d.Id()), nil); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
