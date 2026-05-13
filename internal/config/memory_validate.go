//go:build memory

package config

import "fmt"

// Validate checks memory settings when enabled.
func (m *MemoryConfig) Validate(cfg *Config) error {
	if !m.Enabled {
		return nil
	}
	if m.Model != "" && cfg.FindModelEntry(m.Model) == nil {
		return fmt.Errorf("memory.model %q not found in models list", m.Model)
	}
	return nil
}
