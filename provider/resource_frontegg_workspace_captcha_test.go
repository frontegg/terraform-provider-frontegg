package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type fakeBotDetection struct {
	mu     sync.Mutex
	policy map[string]interface{}
	posts  []map[string]interface{}
}

func (f *fakeBotDetection) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != fronteggBotDetectionPolicyURL {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(f.policy)
	case http.MethodPost:
		var in map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.posts = append(f.posts, in)
		for k, v := range in {
			f.policy[k] = v
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(f.policy)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func newFakeBotDetection(t *testing.T, policy map[string]interface{}) (*fakeBotDetection, *restclient.Client) {
	t.Helper()
	f := &fakeBotDetection{policy: policy}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	client := restclient.MakeRestClient(srv.URL, "", "")
	return f, &client
}

func challengePolicy() map[string]interface{} {
	return map[string]interface{}{
		"id":                    "policy-1",
		"enabled":               true,
		"type":                  "Recaptcha",
		"action":                "CHALLENGE",
		"failStrategy":          "ALLOW",
		"siteKey":               "old-site",
		"secretKey":             "old-secret",
		"minScore":              0.5,
		"ignoredEmails":         []interface{}{},
		"shouldSendUnlockEmail": true,
	}
}

func captchaResourceData(t *testing.T, captcha map[string]interface{}) *schema.ResourceData {
	t.Helper()
	raw := map[string]interface{}{}
	if captcha != nil {
		raw["captcha_policy"] = []interface{}{captcha}
	}
	return schema.TestResourceDataRaw(t, resourceFronteggWorkspace().Schema, raw)
}

func TestUpdateCaptchaPolicyKeepsActionWhenUnset(t *testing.T) {
	f, client := newFakeBotDetection(t, challengePolicy())
	d := captchaResourceData(t, map[string]interface{}{
		"site_key":   "new-site",
		"secret_key": "new-secret",
		"min_score":  0.7,
	})

	if err := updateFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if len(f.posts) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(f.posts))
	}
	got := f.posts[0]
	want := map[string]interface{}{
		"enabled":               true,
		"type":                  "Recaptcha",
		"action":                "CHALLENGE",
		"failStrategy":          "ALLOW",
		"siteKey":               "new-site",
		"secretKey":             "new-secret",
		"minScore":              0.7,
		"shouldSendUnlockEmail": true,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if emails, ok := got["ignoredEmails"].([]interface{}); !ok || len(emails) != 0 {
		t.Errorf("ignoredEmails = %#v, want []", got["ignoredEmails"])
	}
	if _, ok := got["id"]; ok {
		t.Errorf("id must not be sent")
	}
}

func TestUpdateCaptchaPolicySetsAction(t *testing.T) {
	policy := challengePolicy()
	policy["action"] = "BLOCK"
	f, client := newFakeBotDetection(t, policy)
	d := captchaResourceData(t, map[string]interface{}{
		"site_key":       "site",
		"secret_key":     "secret",
		"min_score":      0.5,
		"ignored_emails": []interface{}{"qa@example.com"},
		"action":         "CHALLENGE",
	})

	if err := updateFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if f.policy["action"] != "CHALLENGE" {
		t.Errorf("action = %v, want CHALLENGE", f.policy["action"])
	}
	emails, _ := f.posts[0]["ignoredEmails"].([]interface{})
	if len(emails) != 1 || emails[0] != "qa@example.com" {
		t.Errorf("ignoredEmails = %#v", f.posts[0]["ignoredEmails"])
	}
}

func TestUpdateCaptchaPolicyDisablesWithoutResettingAction(t *testing.T) {
	f, client := newFakeBotDetection(t, challengePolicy())
	d := captchaResourceData(t, nil)

	if err := updateFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if len(f.posts) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(f.posts))
	}
	if f.policy["enabled"] != false || f.policy["action"] != "CHALLENGE" || f.policy["siteKey"] != "old-site" {
		t.Errorf("policy after disable = %#v", f.policy)
	}
}

func TestUpdateCaptchaPolicySkipsWriteWhenAlreadyDisabled(t *testing.T) {
	policy := challengePolicy()
	policy["enabled"] = false
	f, client := newFakeBotDetection(t, policy)
	d := captchaResourceData(t, nil)

	if err := updateFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if len(f.posts) != 0 {
		t.Errorf("expected no POST, got %#v", f.posts)
	}
}

func TestReadCaptchaPolicySetsAction(t *testing.T) {
	_, client := newFakeBotDetection(t, challengePolicy())
	d := captchaResourceData(t, nil)

	if err := readFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if got := d.Get("captcha_policy.0.action"); got != "CHALLENGE" {
		t.Errorf("action = %v, want CHALLENGE", got)
	}
	if got := d.Get("captcha_policy.0.site_key"); got != "old-site" {
		t.Errorf("site_key = %v", got)
	}
	if got := d.Get("captcha_policy.0.min_score"); got != 0.5 {
		t.Errorf("min_score = %v", got)
	}
}

func TestReadCaptchaPolicyDisabledClearsBlock(t *testing.T) {
	_, client := newFakeBotDetection(t, map[string]interface{}{
		"enabled":       false,
		"type":          "Internal",
		"action":        "BLOCK",
		"siteKey":       nil,
		"secretKey":     nil,
		"minScore":      nil,
		"ignoredEmails": []interface{}{},
	})
	d := captchaResourceData(t, map[string]interface{}{
		"site_key":   "site",
		"secret_key": "secret",
		"min_score":  0.5,
	})

	if err := readFronteggCaptchaPolicy(context.Background(), client, d); err != nil {
		t.Fatal(err)
	}

	if got := d.Get("captcha_policy").([]interface{}); len(got) != 0 {
		t.Errorf("captcha_policy = %#v, want empty", got)
	}
}
