package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const fronteggJWTTemplateTargetingPath = "/identity/resources/configurations/v1/jwt-template-targeting"

type fronteggJWTTargetingConditionValue struct {
	List []string `json:"list"`
}

type fronteggJWTTargetingCondition struct {
	Attribute string                             `json:"attribute"`
	Op        string                             `json:"op"`
	Value     fronteggJWTTargetingConditionValue `json:"value"`
	Negate    bool                               `json:"negate"`
}

type fronteggJWTTargetingRule struct {
	ConditionLogic string                          `json:"conditionLogic"`
	Conditions     []fronteggJWTTargetingCondition `json:"conditions"`
	Treatment      string                          `json:"treatment"`
}

type fronteggJWTTemplateTargetingRequest struct {
	Rules []fronteggJWTTargetingRule `json:"rules"`
}

type fronteggJWTTemplateTargetingObject struct {
	Rules []fronteggJWTTargetingRule `json:"rules,omitempty"`
}

type fronteggJWTTemplateTargeting struct {
	ID        string                             `json:"id,omitempty"`
	CreatedAt string                             `json:"createdAt,omitempty"`
	UpdatedAt string                             `json:"updatedAt,omitempty"`
	Targeting fronteggJWTTemplateTargetingObject `json:"targeting"`
}

func resourceFronteggJWTTemplateTargeting() *schema.Resource {
	return &schema.Resource{
		Description: `Configures Frontegg JWT template targeting rules.

Targeting rules determine which JWT template is applied based on attributes of
the requesting user, tenant, or application. There is one targeting configuration
per environment; creating this resource more than once will conflict.`,
		CreateContext: resourceFronteggJWTTemplateTargetingCreate,
		ReadContext:   resourceFronteggJWTTemplateTargetingRead,
		UpdateContext: resourceFronteggJWTTemplateTargetingUpdate,
		DeleteContext: resourceFronteggJWTTemplateTargetingDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"rule": {
				Description: "One or more targeting rules evaluated in order. The first matching rule's template is applied.",
				Type:        schema.TypeList,
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"condition_logic": {
							Description:  "The logic used to combine conditions within this rule. Currently only `and` is supported.",
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"and"}, false),
						},
						"treatment": {
							Description: "The key of the JWT template to use when all conditions in this rule match.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"condition": {
							Description: "One or more conditions that must all be satisfied for this rule to apply.",
							Type:        schema.TypeList,
							Required:    true,
							MinItems:    1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"attribute": {
										Description: "The attribute to evaluate.",
										Type:        schema.TypeString,
										Required:    true,
										ValidateFunc: validation.StringInSlice([]string{
											"userId",
											"applicationId",
											"tenantId",
											"roleIds",
											"tokenType",
											"userEmail",
										}, false),
									},
									"op": {
										Description: "The comparison operation to apply.",
										Type:        schema.TypeString,
										Required:    true,
										ValidateFunc: validation.StringInSlice([]string{
											"in_list",
											"contains",
											"ends_with",
										}, false),
									},
									"values": {
										Description: "The value(s) to compare the attribute against.",
										Type:        schema.TypeList,
										Required:    true,
										MinItems:    1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
									"negate": {
										Description: "When true, the condition result is negated.",
										Type:        schema.TypeBool,
										Required:    true,
									},
								},
							},
						},
					},
				},
			},
			"created_at": {
				Description: "The timestamp at which the targeting configuration was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "The timestamp at which the targeting configuration was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceFronteggJWTTemplateTargetingSerialize(d *schema.ResourceData) fronteggJWTTemplateTargetingRequest {
	rawRules := d.Get("rule").([]interface{})
	rules := make([]fronteggJWTTargetingRule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		ruleMap := rawRule.(map[string]interface{})
		rawConditions := ruleMap["condition"].([]interface{})
		conditions := make([]fronteggJWTTargetingCondition, 0, len(rawConditions))
		for _, rawCond := range rawConditions {
			condMap := rawCond.(map[string]interface{})
			rawValues := condMap["values"].([]interface{})
			values := make([]string, 0, len(rawValues))
			for _, v := range rawValues {
				values = append(values, v.(string))
			}
			conditions = append(conditions, fronteggJWTTargetingCondition{
				Attribute: condMap["attribute"].(string),
				Op:        condMap["op"].(string),
				Value:     fronteggJWTTargetingConditionValue{List: values},
				Negate:    condMap["negate"].(bool),
			})
		}
		rules = append(rules, fronteggJWTTargetingRule{
			ConditionLogic: ruleMap["condition_logic"].(string),
			Conditions:     conditions,
			Treatment:      ruleMap["treatment"].(string),
		})
	}
	return fronteggJWTTemplateTargetingRequest{Rules: rules}
}

func resourceFronteggJWTTemplateTargetingDeserialize(d *schema.ResourceData, t fronteggJWTTemplateTargeting) error {
	d.SetId(t.ID)
	if err := d.Set("created_at", t.CreatedAt); err != nil {
		return err
	}
	if err := d.Set("updated_at", t.UpdatedAt); err != nil {
		return err
	}
	rules := make([]interface{}, 0, len(t.Targeting.Rules))
	for _, r := range t.Targeting.Rules {
		conditions := make([]interface{}, 0, len(r.Conditions))
		for _, c := range r.Conditions {
			values := c.Value.List
			conditions = append(conditions, map[string]interface{}{
				"attribute": c.Attribute,
				"op":        c.Op,
				"values":    values,
				"negate":    c.Negate,
			})
		}
		rules = append(rules, map[string]interface{}{
			"condition_logic": r.ConditionLogic,
			"treatment":       r.Treatment,
			"condition":       conditions,
		})
	}
	if err := d.Set("rule", rules); err != nil {
		return err
	}
	return nil
}

func resourceFronteggJWTTemplateTargetingCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	clientHolder.ApiClient.ConflictRetryMethod("PUT")
	in := resourceFronteggJWTTemplateTargetingSerialize(d)
	// Pass nil: POST returns a body on 201, but the 409→PUT retry returns an
	// empty body, causing json.Unmarshal to fail. Always GET afterwards instead.
	if err := clientHolder.ApiClient.Post(ctx, fronteggJWTTemplateTargetingPath, in, nil); err != nil {
		return diag.FromErr(err)
	}
	var out fronteggJWTTemplateTargeting
	if err := clientHolder.ApiClient.Get(ctx, fronteggJWTTemplateTargetingPath, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateTargetingDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateTargetingRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	var out fronteggJWTTemplateTargeting
	err := clientHolder.ApiClient.Get(ctx, fronteggJWTTemplateTargetingPath, &out)
	if err != nil {
		if restclient.IsNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	if out.ID == "" || out.ID != d.Id() {
		d.SetId("")
		return nil
	}
	if err := resourceFronteggJWTTemplateTargetingDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateTargetingUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	in := resourceFronteggJWTTemplateTargetingSerialize(d)
	var out fronteggJWTTemplateTargeting
	if err := clientHolder.ApiClient.Put(ctx, fronteggJWTTemplateTargetingPath, in, &out); err != nil {
		return diag.FromErr(err)
	}
	if err := resourceFronteggJWTTemplateTargetingDeserialize(d, out); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggJWTTemplateTargetingDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	clientHolder := m.(*restclient.ClientHolder)
	err := clientHolder.ApiClient.Delete(ctx, fmt.Sprintf("%s/%s", fronteggJWTTemplateTargetingPath, d.Id()), nil)
	if err != nil && !restclient.IsNotFound(err) {
		return diag.FromErr(err)
	}
	return nil
}
