package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Entitlements batch-actions endpoint. See server:
// github.com/frontegg/entitlements-service
//
//	apps/service/src/entitlements/api/v2/entitlements.v2.controller.ts (batchEntitlementsActions)
//	apps/service/src/entitlements/api/v2/dto/entitlements-actions.dto.ts
//
// Endpoint is @ApiExcludeEndpoint; swap-point is batchActions() below if it moves.
const (
	fronteggEntitlementBasePath     = "/entitlements/resources/entitlements/v2"
	fronteggEntitlementBatchActions = fronteggEntitlementBasePath + "/batch-actions"
	maxBatchActionsSize             = 50
)

type fronteggEntitlementAction struct {
	PlanID         string `json:"planId"`
	TenantID       string `json:"tenantId"`
	UserID         string `json:"userId,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
}

type fronteggEntitlementUpdate struct {
	ID             string `json:"id"`
	ExpirationDate string `json:"expirationDate,omitempty"`
}

type fronteggBatchActionsRequest struct {
	CreateActions []fronteggEntitlementAction `json:"createActions,omitempty"`
	UpdateActions []fronteggEntitlementUpdate `json:"updateActions,omitempty"`
	DeleteActions []string                    `json:"deleteActions,omitempty"`
}

type fronteggBatchActionsResponse struct {
	EntitlementIds []string `json:"entitlementIds"`
}

type fronteggEntitlement struct {
	ID             string `json:"id"`
	PlanID         string `json:"planId"`
	TenantID       string `json:"tenantId"`
	UserID         string `json:"userId,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

func resourceFronteggEntitlement() *schema.Resource {
	return &schema.Resource{
		Description: `Assigns Frontegg entitlement plans to tenants and/or users in bulk via a single batch-actions API call.

**Import caveat:** ` + "`by-tenant:<id>`" + ` and ` + "`by-plan:<id>`" + ` import formats absorb ALL matching entitlements server-side, regardless of source. Review the imported state before the next apply.

**Partial failure recovery:** If an apply exceeds ` + fmt.Sprintf("%d", maxBatchActionsSize) + ` total actions it is chunked (creates → updates → deletes across chunks). On partial failure, already-committed chunks are not rolled back; ` + "`terraform refresh`" + ` + re-apply reconciles.`,

		CreateContext: resourceFronteggEntitlementCreate,
		ReadContext:   resourceFronteggEntitlementRead,
		UpdateContext: resourceFronteggEntitlementUpdate,
		DeleteContext: resourceFronteggEntitlementDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceFronteggEntitlementImport,
		},
		CustomizeDiff: resourceFronteggEntitlementCustomizeDiff,

		Schema: map[string]*schema.Schema{
			"entitlement": {
				Description: "Set of entitlement assignments. Natural key is (plan_id, tenant_id, user_id); expiration_date is PATCH-eligible.",
				Type:        schema.TypeSet,
				Required:    true,
				MinItems:    1,
				Set:         entitlementSetHash,
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"plan_id": {
						Description: "The ID of the plan to grant.",
						Type:        schema.TypeString,
						Required:    true,
					},
					"tenant_id": {
						Description: "The ID of the tenant receiving the plan.",
						Type:        schema.TypeString,
						Required:    true,
					},
					"user_id": {
						Description: "Optional user ID to scope the entitlement to a single user within the tenant.",
						Type:        schema.TypeString,
						Optional:    true,
					},
					"expiration_date": {
						Description:      "Optional RFC3339 expiration timestamp. Changing this value issues a PATCH (no replacement).",
						Type:             schema.TypeString,
						Optional:         true,
						ValidateFunc:     validation.IsRFC3339Time,
						DiffSuppressFunc: suppressRFC3339EquivalentDiff,
					},
					"id": {
						Description: "Server-assigned entitlement ID.",
						Type:        schema.TypeString,
						Computed:    true,
					},
					"created_at": {
						Description: "When the entitlement was created.",
						Type:        schema.TypeString,
						Computed:    true,
					},
					"updated_at": {
						Description: "When the entitlement was last updated.",
						Type:        schema.TypeString,
						Computed:    true,
					},
				}},
			},
		},
	}
}

// suppressRFC3339EquivalentDiff treats two RFC3339 timestamps as equal when they parse
// to the same instant (e.g. server normalizes "2030-01-01T00:00:00Z" → "2030-01-01T00:00:00.000Z").
func suppressRFC3339EquivalentDiff(_, oldVal, newVal string, _ *schema.ResourceData) bool {
	if oldVal == newVal {
		return true
	}
	if oldVal == "" || newVal == "" {
		return false
	}
	to, err1 := time.Parse(time.RFC3339, oldVal)
	tn, err2 := time.Parse(time.RFC3339, newVal)
	if err1 != nil || err2 != nil {
		return false
	}
	return to.Equal(tn)
}

func entitlementSetHash(v interface{}) int {
	m := v.(map[string]interface{})
	// Hash includes expiration_date (normalized to RFC3339 UTC) so Terraform detects
	// expiration changes as set diffs, while server-side milliseconds padding does not
	// cause spurious differences.
	key := fmt.Sprintf("%s|%s|%s|%s",
		m["plan_id"].(string),
		m["tenant_id"].(string),
		m["user_id"].(string),
		normalizeRFC3339(m["expiration_date"].(string)),
	)
	return schema.HashString(key)
}

func normalizeRFC3339(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func entitlementNaturalKey(m map[string]interface{}) string {
	return fmt.Sprintf("%s|%s|%s",
		m["plan_id"].(string),
		m["tenant_id"].(string),
		m["user_id"].(string),
	)
}

func actionNaturalKey(a fronteggEntitlementAction) string {
	return fmt.Sprintf("%s|%s|%s", a.PlanID, a.TenantID, a.UserID)
}

func blockToAction(m map[string]interface{}) fronteggEntitlementAction {
	return fronteggEntitlementAction{
		PlanID:         m["plan_id"].(string),
		TenantID:       m["tenant_id"].(string),
		UserID:         m["user_id"].(string),
		ExpirationDate: m["expiration_date"].(string),
	}
}

// batchActions executes a single batch-actions call. Chunking is handled by the caller
// via executeBatchActions so cross-chunk ordering (creates → updates → deletes) is preserved.
func batchActions(ctx context.Context, c *restclient.Client, req fronteggBatchActionsRequest) ([]string, error) {
	if len(req.CreateActions)+len(req.UpdateActions)+len(req.DeleteActions) == 0 {
		return nil, nil
	}
	var out fronteggBatchActionsResponse
	if err := c.Post(ctx, fronteggEntitlementBatchActions, req, &out); err != nil {
		return nil, err
	}
	return out.EntitlementIds, nil
}

// executeBatchActions sends creates, updates, and deletes respecting the cross-chunk
// safety ordering: all create-chunks first, then update-chunks, then delete-chunks.
// Partial failure leaks orphan NEW rows rather than missing rows.
func executeBatchActions(ctx context.Context, c *restclient.Client, creates []fronteggEntitlementAction, updates []fronteggEntitlementUpdate, deletes []string) ([]string, error) {
	var createdIDs []string

	for start := 0; start < len(creates); start += maxBatchActionsSize {
		end := start + maxBatchActionsSize
		if end > len(creates) {
			end = len(creates)
		}
		ids, err := batchActions(ctx, c, fronteggBatchActionsRequest{CreateActions: creates[start:end]})
		if err != nil {
			return createdIDs, fmt.Errorf("create chunk [%d:%d]: %w (previously-created IDs: %v)", start, end, err, createdIDs)
		}
		createdIDs = append(createdIDs, ids...)
	}

	for start := 0; start < len(updates); start += maxBatchActionsSize {
		end := start + maxBatchActionsSize
		if end > len(updates) {
			end = len(updates)
		}
		if _, err := batchActions(ctx, c, fronteggBatchActionsRequest{UpdateActions: updates[start:end]}); err != nil {
			return createdIDs, fmt.Errorf("update chunk [%d:%d]: %w (created IDs so far: %v)", start, end, err, createdIDs)
		}
	}

	for start := 0; start < len(deletes); start += maxBatchActionsSize {
		end := start + maxBatchActionsSize
		if end > len(deletes) {
			end = len(deletes)
		}
		if _, err := batchActions(ctx, c, fronteggBatchActionsRequest{DeleteActions: deletes[start:end]}); err != nil {
			return createdIDs, fmt.Errorf("delete chunk [%d:%d]: %w (created IDs so far: %v)", start, end, err, createdIDs)
		}
	}

	return createdIDs, nil
}

// reconcileCreatedIDs maps returned entitlement IDs back to input actions by natural key.
// Happy path: positional (server returns IDs in createActions order). If lengths mismatch,
// falls back to per-ID GET to build tupleToID (AC19 fallback, WARN-logged by caller).
func reconcileCreatedIDs(ctx context.Context, c *restclient.Client, creates []fronteggEntitlementAction, ids []string) (map[string]string, error) {
	if len(creates) == len(ids) {
		out := make(map[string]string, len(creates))
		for i, a := range creates {
			out[actionNaturalKey(a)] = ids[i]
		}
		return out, nil
	}
	// Fallback: fetch each ID and match by tuple.
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		e, err := fetchFronteggEntitlement(ctx, c, id)
		if err != nil {
			return nil, fmt.Errorf("AC19 fallback reconciliation failed for id %s: %w", id, err)
		}
		key := fmt.Sprintf("%s|%s|%s", e.PlanID, e.TenantID, e.UserID)
		out[key] = id
	}
	return out, nil
}

func fetchFronteggEntitlement(ctx context.Context, c *restclient.Client, id string) (*fronteggEntitlement, error) {
	var out fronteggEntitlement
	if err := c.Get(ctx, fronteggEntitlementBasePath+"/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Terraform SDK placeholder for unknown-at-plan-time values. When an attribute will be
// computed from another resource in the same plan, the SDK surfaces this UUID to CustomizeDiff.
// We must skip collision validation for tuples containing this sentinel to avoid false positives.
const unknownValuePlaceholder = "74D93920-ED26-11E3-AC10-0800200C9A66"

func resourceFronteggEntitlementCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	raw := d.Get("entitlement")
	set, ok := raw.(*schema.Set)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for _, item := range set.List() {
		m := item.(map[string]interface{})
		planID := m["plan_id"].(string)
		tenantID := m["tenant_id"].(string)
		if planID == "" || tenantID == "" || planID == unknownValuePlaceholder || tenantID == unknownValuePlaceholder {
			// Natural key not fully known yet; collision check deferred to server-side duplicate detection.
			continue
		}
		userID := m["user_id"].(string)
		if userID == unknownValuePlaceholder {
			continue
		}
		k := entitlementNaturalKey(m)
		if seen[k] {
			return fmt.Errorf("duplicate entitlement natural key (plan_id=%q, tenant_id=%q, user_id=%q): each tuple may appear at most once in a frontegg_entitlement resource",
				planID, tenantID, userID)
		}
		seen[k] = true
	}
	return nil
}

func resourceFronteggEntitlementCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	set := d.Get("entitlement").(*schema.Set)

	creates := make([]fronteggEntitlementAction, 0, set.Len())
	for _, item := range set.List() {
		m := item.(map[string]interface{})
		if m["plan_id"].(string) == "" && m["tenant_id"].(string) == "" {
			continue
		}
		creates = append(creates, blockToAction(m))
	}

	ids, err := executeBatchActions(ctx, &clientHolder.ApiClient, creates, nil, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	tupleToID, err := reconcileCreatedIDs(ctx, &clientHolder.ApiClient, creates, ids)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id.UniqueId())
	return hydrateEntitlementSet(ctx, d, &clientHolder.ApiClient, tupleToID)
}

// hydrateEntitlementSet rebuilds the "entitlement" set from its current contents, enriching
// each block with the server-assigned id, created_at, updated_at. tupleToID provides
// natural-key → ID for newly-created rows; blocks already carrying an id in state keep it.
func hydrateEntitlementSet(ctx context.Context, d *schema.ResourceData, c *restclient.Client, tupleToID map[string]string) diag.Diagnostics {
	set := d.Get("entitlement").(*schema.Set)
	newBlocks := make([]interface{}, 0, set.Len())
	for _, item := range set.List() {
		m := item.(map[string]interface{})
		if m["plan_id"].(string) == "" && m["tenant_id"].(string) == "" {
			continue
		}
		key := entitlementNaturalKey(m)
		id, _ := m["id"].(string)
		if id == "" {
			if mapped, ok := tupleToID[key]; ok {
				id = mapped
			}
		}
		if id == "" {
			// Should not happen: block in state without ID and no mapping.
			return diag.Errorf("internal: no ID resolved for entitlement tuple %s", key)
		}
		e, err := fetchFronteggEntitlement(ctx, c, id)
		if err != nil {
			return diag.FromErr(err)
		}
		newBlocks = append(newBlocks, entitlementToBlock(e))
	}
	if err := d.Set("entitlement", newBlocks); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func entitlementToBlock(e *fronteggEntitlement) map[string]interface{} {
	return map[string]interface{}{
		"plan_id":         e.PlanID,
		"tenant_id":       e.TenantID,
		"user_id":         e.UserID,
		"expiration_date": e.ExpirationDate,
		"id":              e.ID,
		"created_at":      e.CreatedAt,
		"updated_at":      e.UpdatedAt,
	}
}

func resourceFronteggEntitlementRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	c := &clientHolder.ApiClient
	set := d.Get("entitlement").(*schema.Set)

	// Sequential fetches: Ignore404() mutates shared Client state, so parallel calls race.
	newBlocks := make([]interface{}, 0, set.Len())
	for _, item := range set.List() {
		m := item.(map[string]interface{})
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		e, err := fetchFronteggEntitlementIgnore404(ctx, c, id)
		if err != nil {
			return diag.FromErr(err)
		}
		if e == nil {
			continue
		}
		newBlocks = append(newBlocks, entitlementToBlock(e))
	}

	if len(newBlocks) == 0 {
		d.SetId("")
		return nil
	}

	if err := d.Set("entitlement", newBlocks); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func fetchFronteggEntitlementIgnore404(ctx context.Context, c *restclient.Client, id string) (*fronteggEntitlement, error) {
	var out fronteggEntitlement
	c.Ignore404()
	if err := c.Get(ctx, fronteggEntitlementBasePath+"/"+id, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, nil
	}
	return &out, nil
}

func resourceFronteggEntitlementUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	c := &clientHolder.ApiClient

	oldRaw, newRaw := d.GetChange("entitlement")
	oldSet := oldRaw.(*schema.Set)
	newSet := newRaw.(*schema.Set)

	// Index old blocks by natural key to recover IDs.
	oldByKey := map[string]map[string]interface{}{}
	for _, item := range oldSet.List() {
		m := item.(map[string]interface{})
		if m["plan_id"].(string) == "" && m["tenant_id"].(string) == "" {
			continue
		}
		oldByKey[entitlementNaturalKey(m)] = m
	}
	newByKey := map[string]map[string]interface{}{}
	for _, item := range newSet.List() {
		m := item.(map[string]interface{})
		// Skip zero-value placeholder blocks that Terraform can emit when a dynamic block
		// evaluates to an empty list but the SDK still surfaces an empty set entry.
		if m["plan_id"].(string) == "" && m["tenant_id"].(string) == "" {
			continue
		}
		newByKey[entitlementNaturalKey(m)] = m
	}

	var creates []fronteggEntitlementAction
	var updates []fronteggEntitlementUpdate
	var deletes []string

	// Added: in new but not in old.
	for k, m := range newByKey {
		if _, exists := oldByKey[k]; !exists {
			creates = append(creates, blockToAction(m))
		}
	}
	// Removed: in old but not in new.
	for k, m := range oldByKey {
		if _, exists := newByKey[k]; !exists {
			id, _ := m["id"].(string)
			if id != "" {
				deletes = append(deletes, id)
			}
		}
	}
	// Patched: in both, but expiration_date differs.
	for k, nm := range newByKey {
		om, exists := oldByKey[k]
		if !exists {
			continue
		}
		oldExp, _ := om["expiration_date"].(string)
		newExp, _ := nm["expiration_date"].(string)
		if oldExp != newExp {
			id, _ := om["id"].(string)
			if id == "" {
				continue
			}
			updates = append(updates, fronteggEntitlementUpdate{
				ID:             id,
				ExpirationDate: newExp,
			})
		}
	}

	ids, err := executeBatchActions(ctx, c, creates, updates, deletes)
	if err != nil {
		return diag.FromErr(err)
	}

	tupleToID, err := reconcileCreatedIDs(ctx, c, creates, ids)
	if err != nil {
		return diag.FromErr(err)
	}

	// Merge: preserved/patched blocks keep their existing IDs from oldByKey.
	// Fetch fresh server state for each surviving block directly by ID (avoids a roundtrip
	// through d.Set+d.Get that can surface stale entries mid-Update).
	hydrated := make([]interface{}, 0, newSet.Len())
	for _, item := range newSet.List() {
		m := item.(map[string]interface{})
		if m["plan_id"].(string) == "" && m["tenant_id"].(string) == "" {
			continue
		}
		k := entitlementNaturalKey(m)
		var id string
		if om, ok := oldByKey[k]; ok {
			id = om["id"].(string)
		} else if mapped, ok := tupleToID[k]; ok {
			id = mapped
		}
		if id == "" {
			return diag.Errorf("internal: no ID resolved for updated entitlement tuple %s", k)
		}
		e, err := fetchFronteggEntitlement(ctx, c, id)
		if err != nil {
			return diag.FromErr(err)
		}
		hydrated = append(hydrated, entitlementToBlock(e))
	}
	if err := d.Set("entitlement", hydrated); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceFronteggEntitlementDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	clientHolder := meta.(*restclient.ClientHolder)
	set := d.Get("entitlement").(*schema.Set)

	ids := make([]string, 0, set.Len())
	for _, item := range set.List() {
		m := item.(map[string]interface{})
		id, _ := m["id"].(string)
		if id != "" {
			ids = append(ids, id)
		}
	}

	if _, err := executeBatchActions(ctx, &clientHolder.ApiClient, nil, nil, ids); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceFronteggEntitlementImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	clientHolder := meta.(*restclient.ClientHolder)
	c := &clientHolder.ApiClient
	raw := d.Id()

	var ids []string
	var err error

	switch {
	case strings.HasPrefix(raw, "by-tenant:"):
		tenantID := strings.TrimPrefix(raw, "by-tenant:")
		if tenantID == "" {
			return nil, fmt.Errorf("import by-tenant requires a tenant id (by-tenant:<tenantId>)")
		}
		ids, err = listEntitlementIDs(ctx, c, url.Values{"tenantIds": {tenantID}})
	case strings.HasPrefix(raw, "by-plan:"):
		planID := strings.TrimPrefix(raw, "by-plan:")
		if planID == "" {
			return nil, fmt.Errorf("import by-plan requires a plan id (by-plan:<planId>)")
		}
		ids, err = listEntitlementIDs(ctx, c, url.Values{"planIds": {planID}})
	default:
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("import produced zero entitlement IDs for: %s", raw)
	}

	blocks := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		e, err := fetchFronteggEntitlement(ctx, c, id)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, entitlementToBlock(e))
	}
	if err := d.Set("entitlement", blocks); err != nil {
		return nil, err
	}
	d.SetId(id.UniqueId())
	return []*schema.ResourceData{d}, nil
}

type fronteggEntitlementListPage struct {
	Items   []fronteggEntitlement `json:"items"`
	HasNext bool                  `json:"hasNext"`
}

func listEntitlementIDs(ctx context.Context, c *restclient.Client, filters url.Values) ([]string, error) {
	entitlements, err := listEntitlements(ctx, c, filters)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entitlements))
	for _, e := range entitlements {
		ids = append(ids, e.ID)
	}
	return ids, nil
}

func listEntitlements(ctx context.Context, c *restclient.Client, filters url.Values) ([]fronteggEntitlement, error) {
	const limit = 10 // Server enforces limit<=10 (observed 400 Bad Request on limit=100).
	offset := 0
	var all []fronteggEntitlement
	for {
		q := url.Values{}
		for k, vs := range filters {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		q.Set("offset", fmt.Sprintf("%d", offset))
		q.Set("limit", fmt.Sprintf("%d", limit))

		var page fronteggEntitlementListPage
		if err := c.Get(ctx, fronteggEntitlementBasePath+"?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if !page.HasNext {
			break
		}
		offset += limit
	}
	return all, nil
}
