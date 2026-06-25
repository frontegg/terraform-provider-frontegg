package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceFronteggJWTTemplateTargetingSerialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplateTargeting().Schema, map[string]interface{}{
		"rule": []interface{}{
			map[string]interface{}{
				"condition_logic": "and",
				"treatment":       "enterprise",
				"condition": []interface{}{
					map[string]interface{}{
						"attribute": "tenantId",
						"op":        "in_list",
						"values":    []interface{}{"t1", "t2"},
						"negate":    false,
					},
				},
			},
		},
	})

	got := resourceFronteggJWTTemplateTargetingSerialize(d)
	if len(got.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(got.Rules))
	}
	r := got.Rules[0]
	if r.ConditionLogic != "and" || r.Treatment != "enterprise" {
		t.Errorf("unexpected rule: %+v", r)
	}
	if len(r.Conditions) != 1 {
		t.Fatalf("conditions = %d, want 1", len(r.Conditions))
	}
	c := r.Conditions[0]
	if c.Attribute != "tenantId" || c.Op != "in_list" || c.Negate {
		t.Errorf("unexpected condition: %+v", c)
	}
	if !reflect.DeepEqual(c.Value.List, []string{"t1", "t2"}) {
		t.Errorf("condition values = %v, want [t1 t2]", c.Value.List)
	}
}

// TestFronteggJWTTargetingConditionWireFormat asserts condition values are
// serialized as {"list": [...]}, matching the Frontegg entitlements SDK's
// ListStringOperationPayload.
func TestFronteggJWTTargetingConditionWireFormat(t *testing.T) {
	b, err := json.Marshal(fronteggJWTTargetingCondition{
		Attribute: "tenantId",
		Op:        "in_list",
		Value:     fronteggJWTTargetingConditionValue{List: []string{"t1", "t2"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	val, ok := m["value"].(map[string]interface{})
	if !ok {
		t.Fatalf("value is not an object: %s", b)
	}
	list, ok := val["list"].([]interface{})
	if !ok || len(list) != 2 || list[0] != "t1" {
		t.Errorf(`expected value.list = ["t1","t2"], got: %s`, b)
	}
}

func TestResourceFronteggJWTTemplateTargetingDeserialize(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplateTargeting().Schema, map[string]interface{}{})

	in := fronteggJWTTemplateTargeting{
		ID:        "cfg-1",
		CreatedAt: "c",
		UpdatedAt: "u",
		Targeting: fronteggJWTTemplateTargetingObject{
			Rules: []fronteggJWTTargetingRule{
				{
					ConditionLogic: "and",
					Treatment:      "internal",
					Conditions: []fronteggJWTTargetingCondition{
						{
							Attribute: "userEmail",
							Op:        "ends_with",
							Negate:    true,
							Value:     fronteggJWTTargetingConditionValue{List: []string{"@x.com"}},
						},
					},
				},
			},
		},
	}
	if err := resourceFronteggJWTTemplateTargetingDeserialize(d, in); err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	if d.Id() != "cfg-1" {
		t.Errorf("id = %q, want cfg-1", d.Id())
	}
	if d.Get("created_at") != "c" || d.Get("updated_at") != "u" {
		t.Errorf("timestamps = %q/%q", d.Get("created_at"), d.Get("updated_at"))
	}
	rules := d.Get("rule").([]interface{})
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	rule := rules[0].(map[string]interface{})
	if rule["condition_logic"] != "and" || rule["treatment"] != "internal" {
		t.Errorf("unexpected rule: %+v", rule)
	}
	conds := rule["condition"].([]interface{})
	if len(conds) != 1 {
		t.Fatalf("conditions = %d, want 1", len(conds))
	}
	cond := conds[0].(map[string]interface{})
	if cond["attribute"] != "userEmail" || cond["op"] != "ends_with" || cond["negate"] != true {
		t.Errorf("unexpected condition: %+v", cond)
	}
	vals := cond["values"].([]interface{})
	if len(vals) != 1 || vals[0] != "@x.com" {
		t.Errorf("condition values = %v, want [@x.com]", vals)
	}
}

// TestResourceFronteggJWTTemplateTargetingUpdateReadsBackViaGet is a regression
// test for the targeting Update path: the targeting PUT returns an empty body,
// so Update must read the result back with a follow-up GET rather than trying to
// unmarshal the empty PUT response (which would error).
func TestResourceFronteggJWTTemplateTargetingUpdateReadsBackViaGet(t *testing.T) {
	var sawPut, sawGet bool
	var putBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fronteggJWTTemplateTargetingPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodPut:
			sawPut = true
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
			// The real targeting PUT responds with an empty body.
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			sawGet = true
			_ = json.NewEncoder(w).Encode(fronteggJWTTemplateTargeting{
				ID:        "cfg-1",
				CreatedAt: "c",
				UpdatedAt: "u",
				Targeting: fronteggJWTTemplateTargetingObject{
					Rules: []fronteggJWTTargetingRule{
						{
							ConditionLogic: "and",
							Treatment:      "enterprise",
							Conditions: []fronteggJWTTargetingCondition{
								{
									Attribute: "tenantId",
									Op:        "in_list",
									Value:     fronteggJWTTargetingConditionValue{List: []string{"t1"}},
								},
							},
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(srv.URL, "", "")}
	holder.ApiClient.Authenticate("test-token")

	d := schema.TestResourceDataRaw(t, resourceFronteggJWTTemplateTargeting().Schema, map[string]interface{}{
		"rule": []interface{}{
			map[string]interface{}{
				"condition_logic": "and",
				"treatment":       "enterprise",
				"condition": []interface{}{
					map[string]interface{}{
						"attribute": "tenantId",
						"op":        "in_list",
						"values":    []interface{}{"t1"},
						"negate":    false,
					},
				},
			},
		},
	})
	d.SetId("cfg-1")

	diags := resourceFronteggJWTTemplateTargetingUpdate(context.Background(), d, holder)
	if diags.HasError() {
		t.Fatalf("update returned diagnostics: %+v", diags)
	}
	if !sawPut {
		t.Error("expected a PUT request")
	}
	if !sawGet {
		t.Error("expected a follow-up GET request")
	}
	if !strings.Contains(putBody, `"treatment":"enterprise"`) {
		t.Errorf("PUT body did not contain the serialized rules: %s", putBody)
	}
	// State must come from the GET response.
	rules := d.Get("rule").([]interface{})
	if len(rules) != 1 {
		t.Fatalf("rules after update = %d, want 1", len(rules))
	}
	if rules[0].(map[string]interface{})["treatment"] != "enterprise" {
		t.Errorf("unexpected rule after update: %+v", rules[0])
	}
}
