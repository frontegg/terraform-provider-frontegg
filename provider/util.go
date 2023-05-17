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

func getTenantIdHeaders(d *schema.ResourceData) http.Header {
	headers := http.Header{}
	tenant_id := d.Get("tenant_id").(string)
	if tenant_id != "" {
		headers.Add("frontegg-tenant-id", tenant_id)
	} else {
		headers = nil
	}
	return headers
}
