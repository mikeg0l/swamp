package app

import "strings"

func findTag(tags []struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}, targetKey string) string {
	for _, t := range tags {
		if t.Key == targetKey {
			return t.Value
		}
	}
	return ""
}

func targetKey(t roleTarget) string {
	return t.AccountID + "|" + t.RoleName
}

func normalizeStartURL(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), "/")
}
