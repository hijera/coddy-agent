package session

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadInstructions reads the configured instruction files from cwd and concatenates their contents.
// Files that don't exist are silently skipped (matching other-agent AGENTS.md convention).
func LoadInstructions(cwd string, files []string) string {
	var parts []string
	for _, name := range files {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		path := filepath.Join(cwd, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		body := strings.TrimSpace(string(data))
		if body == "" {
			continue
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}
