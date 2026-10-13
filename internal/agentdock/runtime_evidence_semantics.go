package agentdock

import "strings"

func validRuntimeOpaqueID(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+16 {
		return false
	}
	for _, ch := range value[len(prefix):] {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return false
		}
	}
	return true
}
