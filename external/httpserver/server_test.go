//go:build http

package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EvilFreelancer/coddy-agent/internal/acp"
	"github.com/EvilFreelancer/coddy-agent/internal/config"
	"github.com/EvilFreelancer/coddy-agent/internal/session"
	"github.com/EvilFreelancer/coddy-agent/internal/version"
	"gopkg.in/yaml.v3"
)

type noopSender struct{}

func (noopSender) SendSessionUpdate(string, interface{}) error { return nil }

func (noopSender) RequestPermission(context.Context, acp.PermissionRequestParams) (*acp.PermissionResult, error) {
	return &acp.PermissionResult{Outcome: "allow", OptionID: "allow"}, nil
}

func TestGETModels(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelEntry{{Model: "openai/gpt-4o", MaxTokens: 100, Temperature: 0.2}},
	}
	runner := func(context.Context, *session.State, []acp.ContentBlock, acp.UpdateSender) (string, error) { return "", nil }
	mgr := session.NewManager(cfg, noopSender{}, runner, slog.Default(), "/tmp", nil)
	srv := New(cfg, mgr, slog.Default(), "/tmp")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Object != "list" || len(body.Data) != 2 {
		t.Fatalf("unexpected body %+v", body)
	}
	seen := map[string]bool{}
	for _, item := range body.Data {
		seen[item.ID] = true
		if item.ID != "agent" && item.ID != "plan" {
			t.Fatalf("unexpected model id %q", item.ID)
		}
		if item.Object != "model" || item.OwnedBy != "coddy-mode" {
			t.Fatalf("unexpected meta on %q %+v", item.ID, item)
		}
	}
	if !seen["agent"] || !seen["plan"] {
		t.Fatalf("want agent and plan, got %+v", body.Data)
	}
}

func TestOpenAPISpecPathsAndVersion(t *testing.T) {
	doc := openAPISpec()
	if doc["openapi"] != "3.0.3" {
		t.Fatalf("openapi field %v", doc["openapi"])
	}
	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		t.Fatal("missing info map")
	}
	if info["version"] != version.Get() {
		t.Fatalf("spec version want %q got %v", version.Get(), info["version"])
	}
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing paths map")
	}
	for _, must := range []string{"/v1/models", "/v1/chat/completions", "/v1/responses", "/v1/responses/{id}"} {
		if _, ok := paths[must]; !ok {
			t.Fatalf("paths missing key %s", must)
		}
	}
}

func TestGETOpenAPIServed(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelEntry{{Model: "openai/gpt-4o", MaxTokens: 100, Temperature: 0.2}},
	}
	runner := func(context.Context, *session.State, []acp.ContentBlock, acp.UpdateSender) (string, error) {
		return "", nil
	}
	mgr := session.NewManager(cfg, noopSender{}, runner, slog.Default(), "/tmp", nil)
	srv := New(cfg, mgr, slog.Default(), "/tmp")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, path := range []string{"/openapi.yaml", "/openapi.json"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s %v", path, err)
		}
		body, err := ioReadAllClose(res.Body)
		if err != nil {
			t.Fatalf("%s read body %v", path, err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d body %s", path, res.StatusCode, body)
		}
		switch path {
		case "/openapi.yaml":
			var root map[string]interface{}
			if err := yaml.Unmarshal([]byte(body), &root); err != nil {
				t.Fatalf("yaml decode %v", err)
			}
			if root["openapi"] != "3.0.3" {
				t.Fatalf("openapi field %+v", root["openapi"])
			}
			if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "yaml") {
				t.Fatalf("Content-Type %q", ct)
			}
			if disp := res.Header.Get("Content-Disposition"); !strings.Contains(strings.ToLower(disp), "inline") {
				t.Fatalf("Content-Disposition %q want inline", disp)
			}
		case "/openapi.json":
			var root map[string]interface{}
			if err := json.Unmarshal([]byte(body), &root); err != nil {
				t.Fatalf("json decode %v", err)
			}
			if root["openapi"] != "3.0.3" {
				t.Fatalf("openapi field %+v", root["openapi"])
			}
			if disp := res.Header.Get("Content-Disposition"); !strings.Contains(strings.ToLower(disp), "inline") {
				t.Fatalf("Content-Disposition %q want inline", disp)
			}
		}
	}

	res, err := http.Get(ts.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	htmlb, err := ioReadAllClose(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("docs status %d", res.StatusCode)
	}
	html := string(htmlb)
	if !strings.Contains(html, "swagger-ui-bundle.js") || !strings.Contains(html, "/openapi.yaml") {
		t.Fatalf("docs page missing Swagger UI refs snippet %s", shorten(html, 200))
	}
}

func ioReadAllClose(b io.ReadCloser) ([]byte, error) {
	defer b.Close()
	return io.ReadAll(b)
}

func shorten(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
