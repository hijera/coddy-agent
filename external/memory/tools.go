package memory

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EvilFreelancer/coddy-agent/internal/config"
	"github.com/EvilFreelancer/coddy-agent/internal/llm"
)

// RecallToolDefinitions contains read-only recall tools (navigation + search + read).
func RecallToolDefinitions() []llm.ToolDefinition {
	all := ToolDefinitions()
	readOnly := map[string]bool{
		"coddy_memory_search": true,
		"coddy_memory_read": true,
		"coddy_memory_list": true,
	}
	out := make([]llm.ToolDefinition, 0, len(readOnly))
	for _, t := range all {
		if readOnly[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

// PersistToolDefinitions is the curator tool list (mutation + diagnostics).
func PersistToolDefinitions() []llm.ToolDefinition {
	return ToolDefinitions()
}

// ToolDefinitions returns tool schemas only for the memory copilot (never exposed to the main agent).
func ToolDefinitions() []llm.ToolDefinition {
	schemaSearch := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "Search query text"},
			"scope": map[string]any{"type": "string", "enum": []any{"global", "project", "both"}, "description": "Which memory roots to search"},
		},
		"required": []any{"query"},
	}
	schemaRead := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Path as scope:relative, e.g. global:preferences.md or global:notes/habits.md"},
		},
		"required": []any{"path"},
	}
	schemaList := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Directory as scope:relative, e.g. global: or global:design (list one level)"},
		},
		"required": []any{"path"},
	}
	schemaMkdir := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Directory to create under scope, e.g. global:preferences or project:architecture/api"},
		},
		"required": []any{"path"},
	}
	schemaSave := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string", "description": "Short title; used for default flat filename when relative_path is omitted"},
			"body":  map[string]any{"type": "string", "description": "Markdown or plain text body to store"},
			"scope": map[string]any{"type": "string", "enum": []any{"global", "project"}},
			"relative_path": map[string]any{
				"type":        "string",
				"description": "Optional path under scope root with .md or .txt extension, e.g. design/auth-flow.md. When omitted, a slug from title is written at scope root.",
			},
		},
		"required": []any{"title", "body", "scope"},
	}
	schemaDelete := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "scope:relative path to delete"},
		},
		"required": []any{"path"},
	}
	return []llm.ToolDefinition{
		{Name: "coddy_memory_search", Description: "Search all memory files under the scope roots; use hits as entry points, then open files with coddy_memory_read and follow scope:relative or Markdown links inside bodies when you need more context.", InputSchema: schemaSearch},
		{Name: "coddy_memory_list", Description: "List directories and memory files (.md/.txt) one level under a scope-relative path. Use coddy_memory_mkdir before saving into a new folder.", InputSchema: schemaList},
		{Name: "coddy_memory_read", Description: "Read one memory file by scope:relative path.", InputSchema: schemaRead},
		{Name: "coddy_memory_mkdir", Description: "Create nested directories under a memory scope (idempotent). Use thematic folders before first save.", InputSchema: schemaMkdir},
		{Name: "coddy_memory_save", Description: "Write or overwrite a distilled memory note. Prefer relative_path with folders for reusable organization.", InputSchema: schemaSave},
		{Name: "coddy_memory_delete", Description: "Delete a memory file the user asked to forget or that is obsolete. Empty dirs are not removed.", InputSchema: schemaDelete},
	}
}

func parseScopeColonPath(full string) (scopeKey, inner string, err error) {
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

func execTool(store *Store, mem *config.MemoryConfig, name, inputJSON string) (string, error) {
	switch name {
	case "coddy_memory_search":
		var args struct {
			Query string `json:"query"`
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		hits, err := store.Search(args.Query, args.Scope, mem.MaxSearchHits)
		if err != nil {
			return "", err
		}
		if len(hits) == 0 {
			return "No matching memory files.", nil
		}
		var b strings.Builder
		for i, h := range hits {
			_, _ = fmt.Fprintf(&b, "### Hit %d (%s score=%d path=%s)\n%s\n\n", i+1, h.Scope, h.Score, h.Path, h.Snippet)
		}
		return b.String(), nil
	case "coddy_memory_list":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		scopeKey, inner, err := parseScopeColonPath(args.Path)
		if err != nil {
			return "", err
		}
		nodes, err := store.ListOneLevel(scopeKey, inner)
		if err != nil {
			return "", err
		}
		if len(nodes) == 0 {
			return "(empty directory)", nil
		}
		var b strings.Builder
		for _, n := range nodes {
			line := fmt.Sprintf("- %s (%s)", n.Name, n.Kind)
			if n.Kind == "file" && n.Size > 0 {
				line += fmt.Sprintf(" size=%d", n.Size)
			}
			if n.Modified != "" {
				line += " modified=" + n.Modified
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
		return b.String(), nil
	case "coddy_memory_read":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		return store.Read(args.Path)
	case "coddy_memory_mkdir":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		scopeKey, inner, err := parseScopeColonPath(args.Path)
		if err != nil {
			return "", err
		}
		if err := store.Mkdir(scopeKey, inner); err != nil {
			return "", err
		}
		return "created or exists: " + strings.TrimSpace(args.Path), nil
	case "coddy_memory_save":
		var args struct {
			Title        string `json:"title"`
			Body         string `json:"body"`
			Scope        string `json:"scope"`
			RelativePath string `json:"relative_path"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		body := strings.TrimSpace(args.Body)
		if len(body) > 900 {
			body = body[:900] + "\n..."
		}
		rel := strings.TrimSpace(args.RelativePath)
		p, err := store.WriteFlexible(args.Scope, args.Title, rel, body)
		if err != nil {
			return "", err
		}
		return "saved as " + p, nil
	case "coddy_memory_delete":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
			return "", err
		}
		if err := store.Delete(args.Path); err != nil {
			return "", err
		}
		return "deleted " + args.Path, nil
	default:
		return "", fmt.Errorf("unknown memory tool %q", name)
	}
}
