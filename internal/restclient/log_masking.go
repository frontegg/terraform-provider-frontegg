package restclient

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var sensitiveLogFieldKeys = []string{
	"authorization",
	"token",
	"access_token",
	"refresh_token",
	"secret",
	"secret_key",
	"client_secret",
	"password",
	"api_key",
}

var bearerTokenPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`)

func withSecretMasking(ctx context.Context) context.Context {
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, sensitiveLogFieldKeys...)
	return tflog.MaskLogRegexes(ctx, bearerTokenPattern)
}

func logFailedResponseHeaders(ctx context.Context, headers http.Header) {
	tflog.Trace(ctx, "response headers for failed request", map[string]interface{}{"response_headers": fmt.Sprint(headers)})
}
