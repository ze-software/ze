// Design: docs/architecture/core-design.md — config sanitization for support bundle

package support

import "strings"

const redactedValue = "REDACTED"

var sensitiveLeafNames = map[string]bool{
	"password":           true,
	"plaintext-password": true,
	"tacacs-password":    true,
	"secret":             true,
	"shared-secret":      true,
	"pre-shared-secret":  true,
	"radius-secret":      true,
	"pre-shared-key":     true,
	"private-key":        true,
	"key":                true,
	"auth-key":           true,
	"passphrase":         true,
	"token":              true,
	"ssh-password-hash":  true,
}

func isSensitiveKey(key string) bool {
	return sensitiveLeafNames[strings.ToLower(key)]
}

// sanitizeConfig walks a config tree (map[string]any) and replaces
// values of sensitive keys with REDACTED. Returns a new map; the
// original is not modified.
func sanitizeConfig(tree map[string]any) map[string]any {
	result := make(map[string]any, len(tree))
	for k, v := range tree {
		switch val := v.(type) {
		case map[string]any:
			result[k] = sanitizeConfig(val)
		case []any:
			result[k] = sanitizeSlice(k, val)
		default:
			if isSensitiveKey(k) {
				result[k] = redactedValue
			} else {
				result[k] = v
			}
		}
	}
	return result
}

func sanitizeSlice(parentKey string, items []any) []any {
	result := make([]any, len(items))
	for i, item := range items {
		switch val := item.(type) {
		case map[string]any:
			result[i] = sanitizeConfig(val)
		default:
			if isSensitiveKey(parentKey) {
				result[i] = redactedValue
			} else {
				result[i] = val
			}
		}
	}
	return result
}
