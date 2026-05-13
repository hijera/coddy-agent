package web

import (
	"context"
	"strings"
	"testing"
)

func TestValidateFetchURLRejectsPrivateAndLoopback(t *testing.T) {
	ctx := context.Background()
	cases := []string{
		"http://127.0.0.1/x",
		"http://10.0.0.1/x",
		"http://192.168.1.1/",
		"http://[::1]/",
		"file:///etc/passwd",
		"ftp://example.com/",
		"http://user:pass@example.com/",
	}
	for _, raw := range cases {
		if _, err := ValidateFetchURL(ctx, raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestHTMLToMarkdownBasic(t *testing.T) {
	out, err := HTMLToMarkdown("<p>Hello <strong>world</strong></p>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "hello") {
		t.Fatalf("unexpected markdown: %q", out)
	}
}
