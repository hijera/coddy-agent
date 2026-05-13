//go:build !memory

package config

// Validate is a no-op when the binary is built without the memory tag (YAML still parses).
func (m *MemoryConfig) Validate(_ *Config) error {
	return nil
}
