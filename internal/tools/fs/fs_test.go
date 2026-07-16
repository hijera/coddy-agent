package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EvilFreelancer/coddy-agent/internal/tooling"
)

// --- edit.go: exact replacement with line-ending preservation --------------

func TestEditNormalizesLineEndingsAndPreservesFileStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		oldString  string
		newString  string
		replaceAll bool
		want       string
	}{
		{
			name:      "LF arguments edit CRLF file",
			content:   "before\r\n  function changePrBill(){\r\n\t\ttoggleBillColumns();\r\n\t}\r\nafter\r\n",
			oldString: "  function changePrBill(){\n\t\ttoggleBillColumns();\n\t}",
			newString: "  function changePrBill(){\n\t\ttoggleBillColumns();\n\t\trefreshBill();\n\t}",
			want:      "before\r\n  function changePrBill(){\r\n\t\ttoggleBillColumns();\r\n\t\trefreshBill();\r\n\t}\r\nafter\r\n",
		},
		{
			name:      "CRLF arguments edit LF file",
			content:   "alpha\nbeta\ngamma\n",
			oldString: "alpha\r\nbeta",
			newString: "alpha\r\nBETA",
			want:      "alpha\nBETA\ngamma\n",
		},
		{
			name:       "replace all preserves CRLF",
			content:    "alpha\r\nbeta\r\nalpha\r\nbeta\r\n",
			oldString:  "alpha\nbeta",
			newString:  "alpha\nBETA",
			replaceAll: true,
			want:       "alpha\r\nBETA\r\nalpha\r\nBETA\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "file.txt")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			args, err := json.Marshal(editArgs{
				Path:       path,
				OldString:  tt.oldString,
				NewString:  tt.newString,
				ReplaceAll: &tt.replaceAll,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := executeEdit(context.Background(), string(args), &tooling.Env{}); err != nil {
				t.Fatalf("executeEdit: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- patch.go: unified diff / v4a patch ------------------------------------

func TestApplyUnifiedDiff_hunkZeroOriginDoesNotPanic(t *testing.T) {
	t.Parallel()
	original := ""
	diff := strings.Join([]string{
		"--- a/file.txt",
		"+++ b/file.txt",
		"@@ -0,0 +1,2 @@",
		"+line one",
		"+line two",
	}, "\n")

	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	want := "line one\nline two"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyUnifiedDiff_replaceMiddleLine(t *testing.T) {
	t.Parallel()
	original := "line1\nline2\nline3\n"
	diff := "@@ -2,1 +2,1 @@\n-line2\n+newline2\n"

	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	want := "line1\nnewline2\nline3\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyV4APatch_codexEnvelope(t *testing.T) {
	t.Parallel()
	original := strings.Join([]string{
		"import splunklib.client as client",
		"import splunklib.results as results",
		"",
		"def splunk_search():",
		"    pass",
	}, "\n")
	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: project/src/api/auto_uw_api.py",
		"@@",
		"-import splunklib.client as client",
		"-import splunklib.results as results",
		"+try:",
		"+    import splunklib.client as client  # type: ignore",
		"+    import splunklib.results as results  # type: ignore",
		"+except Exception:",
		"+    client = None",
		"+    results = None",
		"*** End Patch",
	}, "\n")

	got, err := applyPatch(original, patch)
	if err != nil {
		t.Fatalf("applyPatch: %v", err)
	}
	if !strings.Contains(got, "try:") || !strings.Contains(got, "splunklib.client as client  # type: ignore") {
		t.Fatalf("patch not applied: %q", got)
	}
	if strings.Contains(got, "\nimport splunklib.client as client\n") {
		t.Fatalf("old imports should be removed: %q", got)
	}
}

func TestApplyV4APatch_bareHunkHeader(t *testing.T) {
	t.Parallel()
	original := "alpha\nbeta\ngamma\n"
	patch := "@@\n-beta\n+BETA\n"

	got, err := applyPatch(original, patch)
	if err != nil {
		t.Fatalf("applyPatch: %v", err)
	}
	want := "alpha\nBETA\ngamma\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyPatchPreservesCRLFAndFinalNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		patch string
	}{
		{
			name:  "unified diff",
			patch: "@@ -2,1 +2,1 @@\n-beta\n+BETA\n",
		},
		{
			name: "V4A patch",
			patch: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: file.txt",
				"@@",
				"-beta",
				"+BETA",
				"*** End Patch",
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := applyPatch("alpha\r\nbeta\r\ngamma\r\n", tt.patch)
			if err != nil {
				t.Fatalf("applyPatch: %v", err)
			}
			want := "alpha\r\nBETA\r\ngamma\r\n"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestApplyPatchPreservesTrailingBlankLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		patch string
	}{
		{name: "unified diff", patch: "@@ -2,1 +2,1 @@\n-beta\n+BETA\n"},
		{name: "V4A patch", patch: "@@\n-beta\n+BETA\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := applyPatch("alpha\r\nbeta\r\n\r\n", tt.patch)
			if err != nil {
				t.Fatalf("applyPatch: %v", err)
			}
			want := "alpha\r\nBETA\r\n\r\n"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestApplyUnifiedDiffRejectsMismatchedDeletedLine(t *testing.T) {
	t.Parallel()

	_, err := applyUnifiedDiff("alpha\nbeta\ngamma\n", "@@ -2,1 +2,1 @@\n-wrong\n+BETA\n")
	if err == nil {
		t.Fatal("expected mismatched deleted line to fail")
	}
	if !strings.Contains(err.Error(), "wrong") || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("error should show expected and actual lines, got: %v", err)
	}
}

func TestApplyUnifiedDiffMultipleHunksValidateContext(t *testing.T) {
	t.Parallel()

	original := "one\ntwo\nthree\nfour\nfive\n"
	diff := strings.Join([]string{
		"--- a/file.txt",
		"+++ b/file.txt",
		"@@ -1,2 +1,2 @@",
		" one",
		"-two",
		"+TWO",
		"@@ -4,2 +4,2 @@",
		" four",
		"-five",
		"+FIVE",
	}, "\n")

	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	want := "one\nTWO\nthree\nfour\nFIVE\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyUnifiedDiff_deleteOutOfRangeReturnsError(t *testing.T) {
	t.Parallel()
	original := "only\n"
	diff := "@@ -5,1 +5,0 @@\n-gone\n"

	_, err := applyUnifiedDiff(original, diff)
	if err == nil {
		t.Fatal("expected error for delete out of range")
	}
}

// --- paths.go / grep.go / glob.go: session-store hiding --------------------

// buildStoreTree lays out a workspace that also contains Coddy's own session store,
// mirroring the real ~/.coddy layout where config.yaml sits next to sessions/<id>/.
// It returns the root and the active session dir.
func buildStoreTree(t *testing.T) (root, sessionDir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("provider: moonshot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionDir = filepath.Join(root, "sessions", "sess_other")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Another session's transcript that mentions both the search term and an unrelated task.
	leak := "moonshot reference and find lyrics for another session\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "messages.json"), []byte(leak), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, sessionDir
}

func TestSessionStoreRootAndIsWithinDir(t *testing.T) {
	if got := sessionStoreRoot(""); got != "" {
		t.Fatalf("empty SessionDir should disable filtering, got %q", got)
	}
	root := sessionStoreRoot("/home/u/.coddy/sessions/sess_abc")
	if root != "/home/u/.coddy/sessions" {
		t.Fatalf("store root = %q", root)
	}
	if !isWithinDir("/home/u/.coddy/sessions/sess_x/messages.json", root) {
		t.Fatal("store file should be within store root")
	}
	if isWithinDir("/home/u/.coddy/config.yaml", root) {
		t.Fatal("sibling config must not be treated as within the store")
	}
	if isWithinDir("/home/u/.coddy/sessions-archive/x", root) {
		t.Fatal("prefix-similar sibling dir must not match")
	}
}

func TestDropStoreLinesKeepsNonPathLines(t *testing.T) {
	// No SessionDir → unchanged.
	if got := dropStoreLines("a\nb", ""); got != "a\nb" {
		t.Fatalf("expected passthrough, got %q", got)
	}
	in := "/work/sessions/sess_x/messages.json:1:leak\n/work/main.go:2:keep\nno matches found"
	out := dropStoreLines(in, "/work/sessions")
	if strings.Contains(out, "messages.json") {
		t.Fatalf("store line not dropped: %q", out)
	}
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "no matches found") {
		t.Fatalf("non-store lines must be kept: %q", out)
	}
}

func TestGrepHidesSessionStore(t *testing.T) {
	root, sessionDir := buildStoreTree(t)
	env := &tooling.Env{CWD: root, SessionDir: sessionDir}

	args, _ := json.Marshal(map[string]any{"pattern": "moonshot", "path": root})
	out, err := executeGrep(context.Background(), string(args), env)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "config.yaml") {
		t.Fatalf("expected a real config.yaml match, got: %q", out)
	}
	if strings.Contains(out, "messages.json") || strings.Contains(out, "find lyrics") {
		t.Fatalf("session store leaked into grep results: %q", out)
	}
}

func TestGlobHidesSessionStore(t *testing.T) {
	root, sessionDir := buildStoreTree(t)
	env := &tooling.Env{CWD: root, SessionDir: sessionDir}

	args, _ := json.Marshal(map[string]any{"pattern": "**/*.json", "path": root})
	out, err := executeGlob(context.Background(), string(args), env)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(out, "data.json") {
		t.Fatalf("expected the real data.json file, got: %q", out)
	}
	if strings.Contains(out, "messages.json") {
		t.Fatalf("session store leaked into glob results: %q", out)
	}
}
