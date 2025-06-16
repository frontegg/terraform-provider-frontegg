package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func stringSetToList(set *schema.Set) []string {
	return stringSetToListWithRightTrim(set, "")
}

func stringSetToListWithRightTrim(set *schema.Set, trimRight string) []string {
	out := make([]string, 0, set.Len())
	for _, v := range set.List() {
		trimmed := strings.TrimRight(v.(string), trimRight)
		out = append(out, trimmed)
	}
	return out
}

// trimRightFromStringSlice trims the specified suffix from each string in the slice.
func trimRightFromStringSlice(slice []string, trimRight string) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		trimmed := strings.TrimRight(s, trimRight)
		out = append(out, trimmed)
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
