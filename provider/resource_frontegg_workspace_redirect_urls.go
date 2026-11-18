package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func normalizeRedirectURL(redirectURL string) string {
	parsed, err := url.Parse(redirectURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return redirectURL
	}
	if parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.HasSuffix(redirectURL, "?") || strings.HasSuffix(redirectURL, "#") {
		return redirectURL
	}
	return strings.TrimSuffix(redirectURL, "/")
}

func legacyRedirectURLKey(redirectURL string) string {
	return strings.TrimRight(redirectURL, "/")
}

func matchRegisteredRedirectURLs(wanted []string, registered []fronteggOAuthRedirectURI) (map[int]bool, map[string]bool) {
	isWanted := map[string]bool{}
	for _, url := range wanted {
		isWanted[url] = true
	}
	kept := map[int]bool{}
	satisfied := map[string]bool{}
	for i, r := range registered {
		if url := normalizeRedirectURL(r.RedirectURI); isWanted[url] {
			kept[i] = true
			satisfied[url] = true
		}
	}
	for _, url := range wanted {
		if satisfied[url] {
			continue
		}
		for i, r := range registered {
			if !kept[i] && legacyRedirectURLKey(r.RedirectURI) == legacyRedirectURLKey(url) {
				kept[i] = true
				satisfied[url] = true
				break
			}
		}
	}
	return kept, satisfied
}

func wantedRedirectURLs(d *schema.ResourceData) []string {
	wanted := []string{}
	seen := map[string]bool{}
	for _, v := range d.Get("hosted_login.0.allowed_redirect_urls").(*schema.Set).List() {
		url := normalizeRedirectURL(v.(string))
		if !seen[url] {
			wanted = append(wanted, url)
			seen[url] = true
		}
	}
	return wanted
}

func readFronteggAllowedRedirectURLs(ctx context.Context, client *restclient.Client, d *schema.ResourceData) ([]string, error) {
	var out fronteggOAuthRedirectURIs
	client.Ignore404()
	if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &out); err != nil {
		return nil, err
	}
	kept, satisfied := matchRegisteredRedirectURLs(wantedRedirectURLs(d), out.RedirectURIs)
	urls := []string{}
	listed := map[string]bool{}
	for _, v := range d.Get("hosted_login.0.allowed_redirect_urls").(*schema.Set).List() {
		url := v.(string)
		if satisfied[normalizeRedirectURL(url)] {
			urls = append(urls, url)
			listed[normalizeRedirectURL(url)] = true
		}
	}
	for i, r := range out.RedirectURIs {
		url := normalizeRedirectURL(r.RedirectURI)
		if !kept[i] && !listed[url] {
			urls = append(urls, url)
			listed[url] = true
		}
	}
	return urls, nil
}

func updateFronteggAllowedRedirectURLs(ctx context.Context, client *restclient.Client, d *schema.ResourceData) error {
	wanted := wantedRedirectURLs(d)
	var out fronteggOAuthRedirectURIs
	if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &out); err != nil {
		return err
	}
	kept, satisfied := matchRegisteredRedirectURLs(wanted, out.RedirectURIs)
	for i, r := range out.RedirectURIs {
		if kept[i] {
			continue
		}
		if err := client.Delete(ctx, fmt.Sprintf("%s/%s", fronteggOAuthRedirectURIsURL, r.ID), nil); err != nil {
			return err
		}
	}
	for _, url := range wanted {
		if satisfied[url] {
			continue
		}
		if err := client.Post(ctx, fronteggOAuthRedirectURIsURL, fronteggOAuthRedirectURI{RedirectURI: url}, nil); err != nil {
			return err
		}
	}
	return nil
}
