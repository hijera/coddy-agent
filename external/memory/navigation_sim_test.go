package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/EvilFreelancer/coddy-agent/internal/config"
)

// extractMemoryLinkTargets finds scope:relative references suitable for coddy_memory_read.
// Matches bare paths (global:x.md) and Markdown links [](global:x.md).
func extractMemoryLinkTargets(body string) []string {
	raw := regexp.MustCompile(`\b(global|project):([a-zA-Z0-9_./\-]+\.(?:md|txt))\b`)
	mdHref := regexp.MustCompile(`\[[^\]]*]\(((?:global|project):[a-zA-Z0-9_./\-]+\.(?:md|txt))\)`)
	var out []string
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		if strings.Contains(s, "..") {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, sm := range raw.FindAllStringSubmatch(body, -1) {
		add(sm[1] + ":" + sm[2])
	}
	for _, sm := range mdHref.FindAllStringSubmatch(body, -1) {
		add(strings.Trim(sm[1], `"'`))
	}
	return out
}

func sliceContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// memoryTreeFixture builds a hierarchical global memory tree with cross-linked markdown notes.
func memoryTreeFixture(t *testing.T, globalRoot string) {
	t.Helper()
	layout := []struct {
		relPath string
		body    string
	}{
		{
			"index.md",
			"# Coddy memory hub\n\nStart here after recall search for hub navigation.\n\nSee [architecture index](global:docs/arch/overview.md).\nBare link global:guides/quickstart.txt for plain references.\n",
		},
		{
			"guides/quickstart.txt",
			"Quick checklist. Dive into global:topics/services/api-map.md.\n",
		},
		{
			"docs/arch/overview.md",
			"## Services overview\n\nHigh-level sketch. Related [API map](global:topics/services/api-map.md).\n",
		},
		{
			"topics/services/api-map.md",
			"Routes table. Secrets live in vault note: [vault](global:topics/secrets/vault.md).\n",
		},
		{
			"topics/secrets/vault.md",
			"## Vault naming\nANSWER_UNIQUE_TOKEN_XYZZY_42\nStored only here; indexing pages must not duplicate this literal.\n",
		},
	}
	for _, f := range layout {
		dir := filepath.Dir(f.relPath)
		if dir != "." {
			fullDir := filepath.Join(globalRoot, dir)
			if err := os.MkdirAll(fullDir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		p := filepath.Join(globalRoot, f.relPath)
		if err := os.WriteFile(p, []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// simulateLinkWalkRecall walks memory like a deterministic recall sub-agent without an LLM:
// breadth-first reads from seeds, parses links from bodies, repeats until substring is found or maxReads.
func simulateLinkWalkRecall(st *Store, seeds []string, maxReads int, wantSubstring string) (pathOrder []string, found bool, err error) {
	queued := append([]string(nil), seeds...)
	sort.Strings(queued)
	seenRead := make(map[string]bool)
	readCount := 0
	var blobs []string
	for len(queued) > 0 && readCount < maxReads {
		p := queued[0]
		queued = queued[1:]
		if seenRead[p] {
			continue
		}
		seenRead[p] = true
		body, readErr := st.Read(p)
		if readErr != nil {
			return pathOrder, false, readErr
		}
		readCount++
		pathOrder = append(pathOrder, p)
		blobs = append(blobs, body)
		if strings.Contains(body, wantSubstring) {
			return pathOrder, true, nil
		}
		for _, tgt := range extractMemoryLinkTargets(body) {
			if seenRead[tgt] {
				continue
			}
			queued = append(queued, tgt)
		}
		sort.Strings(queued)
	}
	found = strings.Contains(strings.Join(blobs, "\n"), wantSubstring)
	return pathOrder, found, nil
}

func testMemoryConfigPtr() *config.MemoryConfig {
	m := &config.MemoryConfig{}
	m.ApplyDefaults()
	return m
}

func TestExtractMemoryLinkTargets(t *testing.T) {
	body := "[a](global:docs/x.md) and bare global:guides/y.txt.\nAlso [b](global:topics/z.md) tail.\n"
	got := extractMemoryLinkTargets(body)
	want := []string{"global:docs/x.md", "global:guides/y.txt", "global:topics/z.md"}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
	if extract := extractMemoryLinkTargets(`bad global:../x.md skip`); len(extract) != 0 {
		t.Fatalf("expected skip .. paths, got %v", extract)
	}
}

func TestSearchBootstrapsTreeEntryThenLinkedWalkFindsBuriedLeaf(t *testing.T) {
	const secret = "ANSWER_UNIQUE_TOKEN_XYZZY_42"
	tmp := t.TempDir()
	g := filepath.Join(tmp, "memglobal")
	proj := filepath.Join(tmp, "memproj")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	memoryTreeFixture(t, g)
	st := &Store{globalRoot: g, projectRoot: proj}

	hits, err := st.Search("hub navigation coddy start recall", "global", 10)
	if err != nil {
		t.Fatal(err)
	}
	seeds := make([]string, 0, len(hits))
	for _, h := range hits {
		seeds = append(seeds, h.Path)
	}
	if len(seeds) == 0 {
		t.Fatal("search returned no bootstrap hits")
	}
	if hits[0].Path != "global:index.md" {
		t.Fatalf("expected index as top hit for bootstrap query, got %v", hits[0])
	}

	order, ok, err := simulateLinkWalkRecall(st, seeds, 20, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("never found secret after walking links; reads=%v", order)
	}
	if !sliceContains(order, "global:index.md") {
		t.Fatalf("expected index in walk order, got %v", order)
	}
	last := order[len(order)-1]
	if last != "global:topics/secrets/vault.md" {
		t.Fatalf("want final read at vault leaf, got %q order=%v", last, order)
	}
}

func TestSearchIntermediateQueryThenWalkReachesVault(t *testing.T) {
	const secret = "ANSWER_UNIQUE_TOKEN_XYZZY_42"
	tmp := t.TempDir()
	g := filepath.Join(tmp, "g")
	p := filepath.Join(tmp, "p")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	memoryTreeFixture(t, g)
	st := &Store{globalRoot: g, projectRoot: p}

	firstHits, err := st.Search("services overview routes sketch", "global", 5)
	if err != nil {
		t.Fatal(err)
	}
	seeds := make([]string, 0, len(firstHits))
	for _, h := range firstHits {
		seeds = append(seeds, h.Path)
	}
	order, ok, err := simulateLinkWalkRecall(st, seeds, 20, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("linked walk failed from intermediate search seeds")
	}
	if !sliceContains(order, "global:topics/services/api-map.md") {
		t.Fatalf("expected api-map in pathOrder=%v", order)
	}
}

// TestSequentialSearchReadChain emulates alternating search and read hops an LLM could take.
func TestSequentialSearchReadChain(t *testing.T) {
	const secret = "ANSWER_UNIQUE_TOKEN_XYZZY_42"
	cfg := testMemoryConfigPtr()
	tmp := t.TempDir()
	g := filepath.Join(tmp, "g")
	p := filepath.Join(tmp, "p")
	if err := os.MkdirAll(g, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	memoryTreeFixture(t, g)
	st := &Store{globalRoot: g, projectRoot: p}

	toolHits, err := execTool(st, cfg, "coddy_memory_search", `{"query":"hub navigation coddy","scope":"global"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolHits, "global:index.md") {
		t.Fatalf("search tool: %s", toolHits)
	}
	idxBody, err := execTool(st, cfg, "coddy_memory_read", `{"path":"global:index.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	pathsIdx := extractMemoryLinkTargets(idxBody)
	if len(pathsIdx) == 0 {
		t.Fatal("index should expose outbound links")
	}
	var overviewPath string
	for _, pth := range pathsIdx {
		if strings.Contains(pth, "overview.md") {
			overviewPath = pth
			break
		}
	}
	if overviewPath == "" {
		t.Fatalf("no overview in %v", pathsIdx)
	}
	ovPayload, err := json.Marshal(map[string]string{"path": overviewPath})
	if err != nil {
		t.Fatal(err)
	}
	ovBody, err := execTool(st, cfg, "coddy_memory_read", string(ovPayload))
	if err != nil {
		t.Fatal(err)
	}
	var apiPath string
	for _, pth := range extractMemoryLinkTargets(ovBody) {
		if strings.Contains(pth, "api-map.md") {
			apiPath = pth
			break
		}
	}
	if apiPath == "" {
		t.Fatalf("overview should link to api map, body=%s", ovBody)
	}
	apiPayload, err := json.Marshal(map[string]string{"path": apiPath})
	if err != nil {
		t.Fatal(err)
	}
	apiBody, err := execTool(st, cfg, "coddy_memory_read", string(apiPayload))
	if err != nil {
		t.Fatal(err)
	}
	gotVault := false
	for _, pth := range extractMemoryLinkTargets(apiBody) {
		if pth != "global:topics/secrets/vault.md" {
			continue
		}
		gotVault = true
		pl, jerr := json.Marshal(map[string]string{"path": pth})
		if jerr != nil {
			t.Fatal(jerr)
		}
		vaultBody, rerr := execTool(st, cfg, "coddy_memory_read", string(pl))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if !strings.Contains(vaultBody, secret) {
			t.Fatalf("vault body missing secret: %q", vaultBody)
		}
	}
	if !gotVault {
		t.Fatalf("api-map should reference vault.md, body=%s", apiBody)
	}
}
