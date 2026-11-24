package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestEmailTemplateDeserializePrefersPatternFields verifies that reading a
// template prefers the raw redirectURLPattern/successRedirectUrlPattern fields
// over the rendered redirectURL/successRedirectUrl fields, so configurations
// using template variables (e.g. "{{APP_ID}}") don't produce a permanent diff.
func TestEmailTemplateDeserializePrefersPatternFields(t *testing.T) {
	tests := []struct {
		name                   string
		in                     fronteggEmailTemplateResource
		wantRedirectURL        string
		wantSuccessRedirectURL string
	}{
		{
			name: "pattern fields present take precedence over rendered values",
			in: fronteggEmailTemplateResource{
				Type:                      "ActivateUser",
				RedirectURL:               "https://example.com/activate?appId=",
				RedirectURLPattern:        "https://example.com/activate?appId={{APP_ID}}",
				SuccessRedirectURL:        "http://localhost:3000/",
				SuccessRedirectURLPattern: "{{APP_URL}}",
			},
			wantRedirectURL:        "https://example.com/activate?appId={{APP_ID}}",
			wantSuccessRedirectURL: "{{APP_URL}}",
		},
		{
			name: "rendered values used when pattern fields are absent",
			in: fronteggEmailTemplateResource{
				Type:               "ResetPassword",
				RedirectURL:        "https://example.com/reset-password",
				SuccessRedirectURL: "https://example.com/done",
			},
			wantRedirectURL:        "https://example.com/reset-password",
			wantSuccessRedirectURL: "https://example.com/done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resourceFronteggEmailTemplate().Schema, map[string]interface{}{})
			if err := resourceFronteggEmailTemplateDeserialize(d, tt.in, false); err != nil {
				t.Fatalf("resourceFronteggEmailTemplateDeserialize() error = %v", err)
			}
			if got := d.Get("redirect_url").(string); got != tt.wantRedirectURL {
				t.Errorf("redirect_url = %q, want %q", got, tt.wantRedirectURL)
			}
			if got := d.Get("success_redirect_url").(string); got != tt.wantSuccessRedirectURL {
				t.Errorf("success_redirect_url = %q, want %q", got, tt.wantSuccessRedirectURL)
			}
		})
	}
}

func TestAccEmailTemplateImportRead(t *testing.T) {
	testAccPreCheck(t)

	p := New("test")()
	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{
		"client_id":  os.Getenv("FRONTEGG_CLIENT_ID"),
		"secret_key": os.Getenv("FRONTEGG_SECRET_KEY"),
	})
	if diags := p.Configure(context.Background(), cfg); diags.HasError() {
		t.Fatalf("configure provider: %v", diags)
	}

	d := schema.TestResourceDataRaw(t, resourceFronteggEmailTemplate().Schema, map[string]interface{}{})
	d.SetId("ResetPassword")

	if diags := resourceFronteggEmailTemplateRead(context.Background(), d, p.Meta()); diags.HasError() {
		t.Fatalf("read: %v", diags)
	}
	if d.Id() == "" {
		t.Fatal("read cleared the ID, so import would fail")
	}
	if got := d.Get("template_type").(string); got != "ResetPassword" {
		t.Errorf("template_type = %q, want ResetPassword", got)
	}
}

func TestEmailTemplateAcceptsDashboardTypes(t *testing.T) {
	templateType := resourceFronteggEmailTemplate().Schema["template_type"]
	for _, name := range []string{
		"UnlockUser",
		"UnlockUserSuccess",
		"ActivateUserWithCode",
		"InviteToTenantWithCode",
		"VerifyNewEmail",
		"EmailAddressChanged",
		"ApprovalFlowApprove",
		"CountryRestriction",
	} {
		t.Run(name, func(t *testing.T) {
			if _, errs := templateType.ValidateFunc(name, "template_type"); len(errs) > 0 {
				t.Errorf("template_type %q rejected: %v", name, errs)
			}
			if !strings.Contains(templateType.Description, `"`+name+`"`) {
				t.Errorf("template_type description does not list %q", name)
			}
		})
	}
}

// fakeSanitize mimics what Frontegg's sanitizer does to common HTML: it adds the DOCTYPE and
// closes void tags.
func fakeSanitize(html string) string {
	html = strings.ReplaceAll(html, "<br>", "<br />")
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		html = "<!DOCTYPE html>" + html
	}
	return html
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestEmailTemplateReadHTMLTemplate(t *testing.T) {
	const configured = `<html><body><p>Reset:<br><a href="{{redirectURL}}">here</a></p></body></html>`
	stored := fakeSanitize(configured)
	edited := `<!DOCTYPE html><html><body><p>Edited in the console <a href="{{redirectURL}}">here</a></p></body></html>`

	for _, tc := range []struct {
		name       string
		state      map[string]interface{}
		stored     string
		afterWrite bool
		wantHTML   string
	}{
		{
			name:     "import reads the stored HTML exactly",
			state:    map[string]interface{}{},
			stored:   stored,
			wantHTML: stored,
		},
		{
			name:       "after Terraform's write the configured HTML is kept",
			state:      map[string]interface{}{"html_template": configured},
			stored:     stored,
			afterWrite: true,
			wantHTML:   configured,
		},
		{
			name:     "refresh keeps the configured HTML while the stored HTML is unchanged",
			state:    map[string]interface{}{"html_template": configured, "html_template_sha256": sha256Hex(stored)},
			stored:   stored,
			wantHTML: configured,
		},
		{
			name:     "refresh reads HTML changed outside Terraform",
			state:    map[string]interface{}{"html_template": configured, "html_template_sha256": sha256Hex(stored)},
			stored:   edited,
			wantHTML: edited,
		},
		{
			name:     "state from an older provider adopts the stored HTML as the baseline",
			state:    map[string]interface{}{"html_template": configured},
			stored:   stored,
			wantHTML: configured,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, resourceFronteggEmailTemplate().Schema, tc.state)
			if err := resourceFronteggEmailTemplateDeserialize(d, fronteggEmailTemplateResource{Type: "ResetPassword", HTMLTemplate: tc.stored}, tc.afterWrite); err != nil {
				t.Fatalf("deserialize: %v", err)
			}
			if got := d.Get("html_template").(string); got != tc.wantHTML {
				t.Errorf("html_template = %q, want %q", got, tc.wantHTML)
			}
			if got := d.Get("html_template_sha256").(string); got != sha256Hex(tc.stored) {
				t.Errorf("html_template_sha256 = %q, want the SHA-256 of the stored HTML", got)
			}
		})
	}
}

// fakeMailAPI mirrors identity-service's template endpoint: a POST without htmlTemplate keeps
// the stored HTML, and the HTML is sanitized on write while sanitize is on.
type fakeMailAPI struct {
	mu        sync.Mutex
	templates map[string]fronteggEmailTemplateResource
	sanitize  bool
	reject    bool
	posts     []map[string]interface{}
}

func newFakeMailAPI(t *testing.T) *fakeMailAPI {
	t.Helper()
	api := &fakeMailAPI{templates: map[string]fronteggEmailTemplateResource{}, sanitize: true}
	srv := httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(srv.Close)
	t.Setenv("FRONTEGG_API_BASE_URL", srv.URL)
	t.Setenv("FRONTEGG_CLIENT_ID", "unit-test")
	t.Setenv("FRONTEGG_SECRET_KEY", "unit-test")
	return api
}

func (api *fakeMailAPI) serve(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/auth/vendor":
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
	case r.Method == http.MethodPost && r.URL.Path == fronteggEmailTemplateURL:
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		api.posts = append(api.posts, body)
		if api.reject {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string][]string{"errors": {"HTML template could not be sanitized"}})
			return
		}
		var in fronteggEmailTemplateResource
		_ = json.Unmarshal(raw, &in)
		if _, sent := body["htmlTemplate"]; !sent {
			in.HTMLTemplate = api.templates[in.Type].HTMLTemplate
		} else if api.sanitize && in.HTMLTemplate != "" {
			in.HTMLTemplate = fakeSanitize(in.HTMLTemplate)
		}
		api.templates[in.Type] = in
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && r.URL.Path == fronteggEmailTemplateURL:
		out := []fronteggEmailTemplateResource{}
		for _, tpl := range api.templates {
			out = append(out, tpl)
		}
		_ = json.NewEncoder(w).Encode(out)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (api *fakeMailAPI) storedHTML() string {
	api.mu.Lock()
	defer api.mu.Unlock()
	return api.templates["ResetPassword"].HTMLTemplate
}

func emailTemplateUnitConfig(subject, html string) string {
	return fmt.Sprintf(`
resource "frontegg_email_template" "t" {
  template_type = "ResetPassword"
  from_address  = "noreply@example.com"
  from_name     = "Example"
  subject       = %q
  html_template = %q
}
`, subject, html)
}

func TestEmailTemplateHTMLDriftAgainstSanitizingAPI(t *testing.T) {
	api := newFakeMailAPI(t)
	const configured = `<html><body>Reset:<br><a href="{{redirectURL}}">here</a></body></html>`
	config := emailTemplateUnitConfig("Reset your password", configured)
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				// Sanitized on write: the apply must not leave a diff behind.
				Config: config,
				Check:  resource.TestCheckResourceAttr("frontegg_email_template.t", "html_template", configured),
			},
			{
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					tpl := api.templates["ResetPassword"]
					tpl.HTMLTemplate = `<!DOCTYPE html><html><body>Edited in the console</body></html>`
					api.templates["ResetPassword"] = tpl
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				Check: func(*terraform.State) error {
					if got := api.storedHTML(); got != fakeSanitize(configured) {
						return fmt.Errorf("stored HTML = %q, want the configured HTML restored", got)
					}
					return nil
				},
			},
		},
	})
}

// A rejected HTML change must stay planned, so the next apply retries it.
func TestEmailTemplateRejectedHTMLIsRetried(t *testing.T) {
	api := newFakeMailAPI(t)
	const first = `<html><body>first</body></html>`
	const second = `<html><body>second</body></html>`
	setReject := func(reject bool) func() {
		return func() {
			api.mu.Lock()
			defer api.mu.Unlock()
			api.reject = reject
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: emailTemplateUnitConfig("Reset", first)},
			{
				PreConfig:   setReject(true),
				Config:      emailTemplateUnitConfig("Reset", second),
				ExpectError: regexp.MustCompile(`could not be sanitized`),
			},
			{
				PreConfig:          setReject(false),
				Config:             emailTemplateUnitConfig("Reset", second),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: emailTemplateUnitConfig("Reset", second),
				Check: func(*terraform.State) error {
					if got := api.storedHTML(); got != fakeSanitize(second) {
						return fmt.Errorf("stored HTML = %q, want the retried HTML", got)
					}
					return nil
				},
			},
		},
	})
}

// Empty HTML must be sent, on create as on update: Frontegg keeps the stored HTML when
// htmlTemplate is left out, so the apply would otherwise change nothing.
func TestEmailTemplateEmptyHTMLIsSent(t *testing.T) {
	api := newFakeMailAPI(t)
	api.templates["ResetPassword"] = fronteggEmailTemplateResource{Type: "ResetPassword", HTMLTemplate: "<p>stored</p>"}
	cleared := func(*terraform.State) error {
		if got := api.storedHTML(); got != "" {
			return fmt.Errorf("stored HTML = %q, want it cleared", got)
		}
		return nil
	}
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: emailTemplateUnitConfig("Reset", ""), Check: cleared},
			{Config: emailTemplateUnitConfig("Reset", `<html><body>first</body></html>`)},
			{Config: emailTemplateUnitConfig("Reset", ""), Check: cleared},
		},
	})
}

// An update that leaves html_template alone must not send it: Frontegg would sanitize it again,
// changing the stored HTML and its hash although the plan showed no HTML change.
func TestEmailTemplateSubjectChangeKeepsStoredHTML(t *testing.T) {
	api := newFakeMailAPI(t)
	api.sanitize = false // stored before sanitizing was enabled
	const configured = `<html><body>Reset:<br>here</body></html>`
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{Config: emailTemplateUnitConfig("Reset", configured)},
			{
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.sanitize = true
					api.posts = nil
				},
				Config: emailTemplateUnitConfig("Reset your password", configured),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("frontegg_email_template.t", "subject", "Reset your password"),
					resource.TestCheckResourceAttr("frontegg_email_template.t", "html_template_sha256", sha256Hex(configured)),
					func(*terraform.State) error {
						api.mu.Lock()
						defer api.mu.Unlock()
						for _, body := range api.posts {
							if _, sent := body["htmlTemplate"]; sent {
								return fmt.Errorf("POST body %v sends htmlTemplate for a subject change", body)
							}
						}
						if got := api.templates["ResetPassword"].HTMLTemplate; got != configured {
							return fmt.Errorf("stored HTML = %q, want it untouched", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// An update that does not send the HTML does not know what Frontegg stored, so a console edit
// made after the plan's refresh must still show as drift.
func TestEmailTemplateUpdateWithoutHTMLReportsConsoleEdit(t *testing.T) {
	api := newFakeMailAPI(t)
	const configured = `<!DOCTYPE html><html><body>configured</body></html>`
	const edited = `<!DOCTYPE html><html><body>edited in the console</body></html>`
	api.templates["ResetPassword"] = fronteggEmailTemplateResource{Type: "ResetPassword", Subject: "Reset", HTMLTemplate: edited}
	holder := &restclient.ClientHolder{ApiClient: restclient.MakeRestClient(os.Getenv("FRONTEGG_API_BASE_URL"), "", "")}
	holder.ApiClient.Authenticate("test-token")

	res := resourceFronteggEmailTemplate()
	prior := &terraform.InstanceState{ID: "ResetPassword", Attributes: map[string]string{
		"id": "ResetPassword", "template_type": "ResetPassword", "active": "true",
		"from_address": "noreply@example.com", "from_name": "Example", "subject": "Reset",
		"html_template": configured, "html_template_sha256": sha256Hex(configured),
		"redirect_url": "", "success_redirect_url": "",
	}}
	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{
		"template_type": "ResetPassword", "from_address": "noreply@example.com", "from_name": "Example",
		"subject": "Reset your password", "html_template": configured,
	})
	diff, err := res.Diff(context.Background(), prior, cfg, holder)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	got, diags := res.Apply(context.Background(), prior, diff, holder)
	if diags.HasError() {
		t.Fatalf("apply: %v", diags)
	}
	if got.Attributes["html_template"] != edited {
		t.Errorf("html_template = %q, want the console edit %q so the next plan shows it", got.Attributes["html_template"], edited)
	}
}

// hclString quotes s as an HCL string literal, escaping template sequences.
func hclString(s string) string {
	s = strings.ReplaceAll(s, "${", "$${")
	s = strings.ReplaceAll(s, "%{", "%%{")
	return fmt.Sprintf("%q", s)
}

// testAccEmailTemplate reads a template type from the API; the acceptance test restores it.
func testAccEmailTemplate(t *testing.T, client *restclient.Client, templateType string) fronteggEmailTemplateResource {
	t.Helper()
	var out []fronteggEmailTemplateResource
	if err := client.Get(context.Background(), fronteggEmailTemplateURL, &out); err != nil {
		t.Fatalf("listing email templates: %v", err)
	}
	for _, tpl := range out {
		if tpl.Type == templateType {
			return tpl
		}
	}
	t.Fatalf("email template %s not found", templateType)
	return fronteggEmailTemplateResource{}
}

func testAccWriteEmailTemplate(t *testing.T, client *restclient.Client, tpl fronteggEmailTemplateResource) {
	t.Helper()
	if tpl.RedirectURLPattern != "" {
		tpl.RedirectURL = tpl.RedirectURLPattern
	}
	if tpl.SuccessRedirectURLPattern != "" {
		tpl.SuccessRedirectURL = tpl.SuccessRedirectURLPattern
	}
	tpl.RedirectURLPattern, tpl.SuccessRedirectURLPattern = "", ""
	if err := client.Post(context.Background(), fronteggEmailTemplateURL, tpl, nil); err != nil {
		t.Fatalf("writing email template %s: %v", tpl.Type, err)
	}
}

func TestAccFronteggEmailTemplate_htmlTemplate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
	testAccPreCheck(t)
	base := os.Getenv("FRONTEGG_API_BASE_URL")
	if base == "" {
		base = "https://api.frontegg.com"
	}
	client := restclient.MakeRestClient(base, "", os.Getenv("FRONTEGG_APPLICATION_ID"))
	client.Authenticate(fronteggVendorTokenNoT(base))

	const templateType = "ResetPassword"
	original := testAccEmailTemplate(t, &client, templateType)
	t.Cleanup(func() { testAccWriteEmailTemplate(t, &client, original) })

	fromName := original.FromName
	if fromName == "" {
		// Default templates have no from name, which the schema requires.
		fromName = "Terraform acceptance test"
	}
	configFor := func(subject, html string) string {
		optional := ""
		if original.RedirectURLPattern != "" {
			optional += "  redirect_url  = " + hclString(original.RedirectURLPattern) + "\n"
		}
		if original.SuccessRedirectURLPattern != "" {
			optional += "  success_redirect_url = " + hclString(original.SuccessRedirectURLPattern) + "\n"
		}
		return fmt.Sprintf(`
resource "frontegg_email_template" "t" {
  template_type = %q
  active        = %t
  from_address  = %s
  from_name     = %s
  subject       = %s
  html_template = %s
%s}
`, templateType, original.Active, hclString(original.SenderEmail), hclString(fromName), hclString(subject), hclString(html), optional)
	}
	config := func(html string) string { return configFor(original.Subject, html) }
	changedSubject := original.Subject + " (tf-acc)"
	var storedHash string
	// Left unchanged by Frontegg's sanitizer.
	const canonical = `<!DOCTYPE html><html><body><p>Reset your password: <a href="{{redirectURL}}">reset</a></p></body></html>`
	// Rewritten by the sanitizer: DOCTYPE added, <br> closed, onclick and the comment dropped.
	const sanitized = "<html>\n  <body>\n    <p>Reset:<br><a href=\"{{redirectURL}}\" onclick=\"x()\">reset</a></p>\n    <!-- footer -->\n  </body>\n</html>"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(canonical),
				Check:  resource.TestCheckResourceAttr("frontegg_email_template.t", "html_template", canonical),
			},
			{
				ResourceName:      "frontegg_email_template.t",
				ImportState:       true,
				ImportStateId:     templateType,
				ImportStateVerify: true,
			},
			{
				Config:   config(canonical),
				PlanOnly: true,
			},
			{
				// The stored HTML differs from the configured one; the plan must stay clean.
				Config: config(sanitized),
				Check:  resource.TestCheckResourceAttr("frontegg_email_template.t", "html_template", sanitized),
			},
			{
				PreConfig: func() {
					edited := testAccEmailTemplate(t, &client, templateType)
					edited.HTMLTemplate = `<!DOCTYPE html><html><body><p>Changed outside Terraform: <a href="{{redirectURL}}">reset</a></p></body></html>`
					testAccWriteEmailTemplate(t, &client, edited)
				},
				Config:             config(sanitized),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config(sanitized),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("frontegg_email_template.t", "html_template_sha256"),
					func(s *terraform.State) error {
						storedHash = s.RootModule().Resources["frontegg_email_template.t"].Primary.Attributes["html_template_sha256"]
						return nil
					},
				),
			},
			{
				// A subject change leaves the stored HTML, and so its hash, alone.
				Config: configFor(changedSubject, sanitized),
				Check:  resource.TestCheckResourceAttrPtr("frontegg_email_template.t", "html_template_sha256", &storedHash),
			},
			{
				// Rejected regardless of sanitizing: Frontegg compiles the HTML as Handlebars.
				Config:      configFor(changedSubject, `<!DOCTYPE html><html><body>{{#if x}}unclosed</body></html>`),
				ExpectError: regexp.MustCompile(`Invalid HTML template`),
			},
			{
				// The rejected change is still planned, so the next apply retries it.
				Config:             configFor(changedSubject, `<!DOCTYPE html><html><body>{{#if x}}unclosed</body></html>`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:   configFor(changedSubject, sanitized),
				PlanOnly: true,
			},
		},
	})
}
