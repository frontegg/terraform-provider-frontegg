package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/frontegg/terraform-provider-frontegg/internal/restclient"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func normalizeRedirectURL(url string) string {
	return strings.TrimRight(url, "/")
}

func readFronteggAllowedRedirectURLs(ctx context.Context, client *restclient.Client, d *schema.ResourceData) ([]string, error) {
	var out fronteggOAuthRedirectURIs
	client.Ignore404()
	if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &out); err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	for _, r := range out.RedirectURIs {
		registered[normalizeRedirectURL(r.RedirectURI)] = true
	}
	urls := []string{}
	covered := map[string]bool{}
	for _, v := range d.Get("hosted_login.0.allowed_redirect_urls").(*schema.Set).List() {
		url := v.(string)
		if registered[normalizeRedirectURL(url)] {
			urls = append(urls, url)
			covered[normalizeRedirectURL(url)] = true
		}
	}
	for _, r := range out.RedirectURIs {
		url := normalizeRedirectURL(r.RedirectURI)
		if !covered[url] {
			urls = append(urls, url)
			covered[url] = true
		}
	}
	return urls, nil
}

func updateFronteggAllowedRedirectURLs(ctx context.Context, client *restclient.Client, d *schema.ResourceData) error {
	wanted := []string{}
	isWanted := map[string]bool{}
	for _, url := range stringSetToListWithRightTrim(d.Get("hosted_login.0.allowed_redirect_urls").(*schema.Set), "/") {
		if !isWanted[url] {
			wanted = append(wanted, url)
			isWanted[url] = true
		}
	}
	var out fronteggOAuthRedirectURIs
	if err := client.Get(ctx, fronteggOAuthRedirectURIsURL, &out); err != nil {
		return err
	}
	registered := map[string]bool{}
	for _, r := range out.RedirectURIs {
		url := normalizeRedirectURL(r.RedirectURI)
		if !isWanted[url] {
			if err := client.Delete(ctx, fmt.Sprintf("%s/%s", fronteggOAuthRedirectURIsURL, r.ID), nil); err != nil {
				return err
			}
			continue
		}
		registered[url] = true
	}
	for _, url := range wanted {
		if registered[url] {
			continue
		}
		if err := client.Post(ctx, fronteggOAuthRedirectURIsURL, fronteggOAuthRedirectURI{RedirectURI: url}, nil); err != nil {
			return err
		}
		registered[url] = true
	}
	return nil
}
