package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func stringSetToList(set *schema.Set) []string {
	out := make([]string, 0, set.Len())
	for _, v := range set.List() {
		out = append(out, v.(string))
	}
	return out
}
