//go:build memory

package memtools

import (
	"fmt"
	"strings"
)

// ParseScopeColonPath splits scope:relative into scope key and inner path.
func ParseScopeColonPath(full string) (scopeKey, inner string, err error) {
	full = strings.TrimSpace(full)
	full = strings.ReplaceAll(full, "\\", "/")
	parts := strings.SplitN(full, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("path must be scope:relative, got %q", full)
	}
	scopeKey = strings.ToLower(strings.TrimSpace(parts[0]))
	inner = strings.TrimSpace(parts[1])
	inner = strings.TrimPrefix(inner, "/")
	return scopeKey, inner, nil
}
