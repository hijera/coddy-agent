package skills

import (
	"os"
	"path/filepath"
	"strings"
)

const disabledFileName = ".disabled"

// disabledFilePath returns the path to the disabled-skills list file.
func disabledFilePath(installDir string) string {
	return filepath.Join(installDir, disabledFileName)
}

// ReadDisabled returns the set of skill names listed in the disabled file.
func ReadDisabled(installDir string) map[string]struct{} {
	data, err := os.ReadFile(disabledFilePath(installDir))
	if err != nil {
		return nil
	}
	out := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && !strings.HasPrefix(name, "#") {
			out[name] = struct{}{}
		}
	}
	return out
}

// DisableSkill adds name to the disabled file. Idempotent.
func DisableSkill(installDir, name string) error {
	disabled := ReadDisabled(installDir)
	if disabled == nil {
		disabled = make(map[string]struct{})
	}
	disabled[name] = struct{}{}
	return writeDisabled(installDir, disabled)
}

// EnableSkill removes name from the disabled file. Idempotent.
func EnableSkill(installDir, name string) error {
	disabled := ReadDisabled(installDir)
	delete(disabled, name)
	return writeDisabled(installDir, disabled)
}

// IsDisabled reports whether name is in the disabled set.
func IsDisabled(disabled map[string]struct{}, name string) bool {
	if disabled == nil {
		return false
	}
	_, ok := disabled[name]
	return ok
}

func writeDisabled(installDir string, names map[string]struct{}) error {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	var lines []string
	for name := range names {
		if name != "" {
			lines = append(lines, name)
		}
	}
	// Deterministic order via manual sort.
	sortStrings(lines)
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	return os.WriteFile(disabledFilePath(installDir), []byte(content), 0o644)
}

// sortStrings sorts a string slice in-place (insertion sort, small lists only).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
