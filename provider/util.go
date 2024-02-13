package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func stringSetToList(set *schema.Set) []string {
	out := make([]string, 0, set.Len())
	for _, v := range set.List() {
		out = append(out, v.(string))
	}
	return out
}

func castResourceStringMap(resourceMapValue interface{}) map[string]string {
	newStringMap := make(map[string]string)
	for key, val := range resourceMapValue.(map[string]interface{}) {
		newStringMap[key] = val.(string)
	}
	return newStringMap
}

func stringInSlice(target string, slice []string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}
