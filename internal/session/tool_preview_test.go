package session

import (
	"fmt"
	"strings"
	"testing"
)

func TestPreviewToolOutputForHTTPUser_truncatesAt19PlusEllipsisRow(t *testing.T) {
	var b strings.Builder
	for i := range 21 {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "L%d", i)
	}
	prev, total, trunc := PreviewToolOutputForHTTPUser(b.String())
	if !trunc || total != 21 {
		t.Fatalf("trunc=%v total=%d", trunc, total)
	}
	lastNL := strings.LastIndex(prev, "\n")
	after := prev[lastNL+1:]
	if after != "..." {
		t.Fatalf("last row want ... got %q full %q", after, prev)
	}
	body := prev[:lastNL]
	if strings.Count(body, "\n") != ToolHTTPUserPreviewContentLines-1 {
		t.Fatalf("want %d newline-separated content rows, body=%q", ToolHTTPUserPreviewContentLines-1, body)
	}
}

func TestPreviewToolOutputForHTTPUser_exact19NoTruncate(t *testing.T) {
	var b strings.Builder
	for i := range 19 {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte(byte('z'))
	}
	s := b.String()
	prev, n, trunc := PreviewToolOutputForHTTPUser(s)
	if trunc || n != 19 || prev != s {
		t.Fatalf("got trunc=%v n=%d prev=%q", trunc, n, prev)
	}
}

func TestPreviewToolResultForSessionUpdate_metaWhenTruncated(t *testing.T) {
	var b strings.Builder
	for i := range 22 {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('y')
	}
	got, meta := PreviewToolResultForSessionUpdate("x", b.String())
	if !strings.HasSuffix(got, "\n...") || meta == nil {
		t.Fatalf("got %q meta=%v", got, meta)
	}
}
