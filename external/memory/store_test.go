package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSearch(t *testing.T) {
	tmp := t.TempDir()
	g := filepath.Join(tmp, "g")
	p := filepath.Join(tmp, "p")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g, "prefs.md"), []byte("User prefers tabs and Go modules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "proj.md"), []byte("This repo uses Makefile for build"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &Store{globalRoot: g, projectRoot: p}
	hits, err := st.Search("tabs golang", "both", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 1 {
		t.Fatalf("expected hits, got %d", len(hits))
	}
}

func TestRecallToolDefinitionsReadOnly(t *testing.T) {
	defs := RecallToolDefinitions()
	want := map[string]bool{"coddy_memory_search": true, "coddy_memory_read": true, "coddy_memory_list": true}
	for _, d := range defs {
		if !want[d.Name] {
			t.Fatalf("unexpected recall tool %q", d.Name)
		}
		delete(want, d.Name)
	}
	if len(defs) != 3 || len(want) != 0 {
		t.Fatalf("expected exactly three recall tools, got %d missing %v", len(defs), want)
	}
}

func TestStoreWriteFlexibleNestedAndList(t *testing.T) {
	tmp := t.TempDir()
	g := filepath.Join(tmp, "g")
	p := filepath.Join(tmp, "p")
	st := &Store{globalRoot: g, projectRoot: p}

	written, err := st.WriteFlexible("global", "API", "design/auth-notes.md", "body line")
	if err != nil {
		t.Fatal(err)
	}
	if written != "global:design/auth-notes.md" {
		t.Fatalf("written %q", written)
	}
	nodes, err := st.ListOneLevel("global", "design")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range nodes {
		if n.Kind == "file" && n.Name == "auth-notes.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list nodes %#v", nodes)
	}
	if err := st.Mkdir("global", "preferences"); err != nil {
		t.Fatal(err)
	}
	nodes2, err := st.ListOneLevel("global", "")
	if err != nil {
		t.Fatal(err)
	}
	okDir := false
	for _, n := range nodes2 {
		if n.Name == "preferences" && n.Kind == "dir" {
			okDir = true
		}
	}
	if !okDir {
		t.Fatalf("expected preferences dir %#v", nodes2)
	}
}

func TestStoreRejectTraversalDotDot(t *testing.T) {
	g := filepath.Join(t.TempDir(), "gonly")
	st := &Store{globalRoot: g, projectRoot: filepath.Join(t.TempDir(), "ponly")}
	if _, err := st.WriteFlexible("global", "x", "../evil.md", "y"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := st.Read("global:../x"); err == nil {
		t.Fatal("expected error")
	}
	if err := st.Mkdir("global", ".."); err == nil {
		t.Fatal("expected error")
	}
}

func TestSlugify(t *testing.T) {
	if g := slugify("  Hello World!!  "); g != "hello-world" {
		t.Fatalf("got %q", g)
	}
	if g := slugify(""); g != "note" {
		t.Fatalf("got %q", g)
	}
}
