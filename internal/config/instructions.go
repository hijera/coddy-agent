package config

import "strings"

// Instructions configures which files from the session CWD are read as user-provided
// instructions and appended to the system prompt. Compatible with the AGENTS.md convention
// used by other AI coding agents.
type Instructions struct {
	// Files is the list of filenames (relative to session CWD) to read.
	// Defaults to ["AGENTS.md"] when empty.
	Files []string `yaml:"files"`
}

// ApplyDefaults sets the default file list when empty.
func (c *Instructions) ApplyDefaults() {
	if len(c.Files) == 0 {
		c.Files = []string{"AGENTS.md"}
	}
}

// Validate normalises the file list.
func (c *Instructions) Validate() error {
	for i := range c.Files {
		c.Files[i] = strings.TrimSpace(c.Files[i])
	}
	return nil
}
