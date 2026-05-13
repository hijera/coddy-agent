package tools

import (
	"testing"
)

func TestRegistryIncludesWriteTextFile(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("write_text_file"); !ok {
		t.Fatal("write_text_file should be registered")
	}
}

func TestAllToolDefinitionsIncludesReadAndWriteText(t *testing.T) {
	r := NewRegistry()
	names := make(map[string]bool)
	for _, d := range r.AllToolDefinitions() {
		names[d.Name] = true
	}
	if !names["read_file"] || !names["list_dir"] || !names["write_text_file"] {
		t.Fatalf("expected read_file, list_dir, write_text_file in full set: missing from %+v", names)
	}
}
