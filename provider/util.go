package provider

import (
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func stringSetToList(set *schema.Set) []string {
	out := make([]string, 0, set.Len())
	for _, v := range set.List() {
		out = append(out, v.(string))
	}
	return out
}

func getEnvironmentHeaders(environment_id string) http.Header {
	headers := http.Header{}
	if environment_id != "" {
		headers.Add("frontegg-environment-id", environment_id)
	} else {
		headers = nil
	}
	return headers
}
