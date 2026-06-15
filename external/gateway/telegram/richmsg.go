//go:build gateway || gateway.telegram

package telegram

import (
	"fmt"
	"strings"
)

// Rich Messages (Bot API 10.1) let the gateway send the agent's native Markdown
// almost verbatim — headings, tables, task lists, fenced code, footnotes and LaTeX
// all render natively, instead of being downgraded to Telegram legacy Markdown.
//
// Two builders produce the InputRichMessage.markdown string:
//   - buildRichMarkdown   → the final, persistent message (uses a collapsible
//     <details> block to list executed tools).
//   - buildRichDraftMarkdown → the ephemeral streaming preview sent via
//     sendRichMessageDraft (uses the draft-only <tg-thinking> block while a tool runs).

// toolCall is one tool execution captured during a turn: its name, the JSON args,
// the result preview, and whether it failed.
type toolCall struct {
	name   string
	args   string
	result string
	failed bool
}

// buildRichMarkdown assembles the final rich-message markdown from the accumulated
// LLM text and the tools that ran during the turn. The LLM text is passed through
// verbatim (it is already GitHub-flavored Markdown); each tool is appended as its own
// collapsed-by-default <details> block showing its output.
func buildRichMarkdown(llmText string, tools []toolCall) string {
	text := strings.TrimRight(llmText, " \t\n")
	details := richToolsDetails(tools)
	if details == "" {
		return strings.TrimSpace(text)
	}
	if strings.TrimSpace(text) == "" {
		return details
	}
	return text + "\n\n" + details
}

// richToolsDetails renders every executed tool as its own collapsed <details> block:
// the summary is the tool name (❌ on failure), the body is the tool's output (or its
// args when no output was captured) in a fenced code block. Returns "" when no tools ran.
func richToolsDetails(tools []toolCall) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range tools {
		icon := "🛠"
		if t.failed {
			icon = "❌"
		}
		name := strings.TrimSpace(t.name)
		if name == "" {
			name = "tool"
		}
		fmt.Fprintf(&b, "<details><summary>%s %s</summary>\n\n", icon, name)
		body := strings.TrimSpace(t.result)
		if body == "" {
			body = strings.TrimSpace(t.args)
		}
		if body != "" {
			// Keep messages within Telegram limits and never let the body's own
			// fences break out of the surrounding code block.
			body = strings.ReplaceAll(truncate(body, 1500), "```", "ʼʼʼ")
			fmt.Fprintf(&b, "```\n%s\n```\n\n", body)
		}
		b.WriteString("</details>\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildRichDraftMarkdown assembles the streaming-preview markdown. While a tool is
// executing, a draft-only <tg-thinking> placeholder is appended below the accumulated
// LLM text so the user sees a native "Thinking…" animation. When no tool is running,
// the accumulated text is returned as-is.
func buildRichDraftMarkdown(llmText, currentTool string) string {
	text := strings.TrimRight(llmText, " \t\n")
	if strings.TrimSpace(currentTool) == "" {
		return strings.TrimSpace(text)
	}
	thinking := "<tg-thinking>⚙️ " + currentTool + "…</tg-thinking>"
	if strings.TrimSpace(text) == "" {
		return thinking
	}
	return text + "\n\n" + thinking
}

// richFormattingHint is prepended to the first message of a new session when Rich
// Messages are enabled, so the agent leans into full Markdown instead of the
// restricted Telegram legacy subset used by telegramFormattingHint.
const richFormattingHint = `[System note – your replies are rendered as Telegram Rich Messages:
• Use the full GitHub-flavored Markdown you would use normally
• Headings (#, ##, ###), **bold**, _italic_, ~~strikethrough~~, ` + "`code`" + `
• Tables, ordered/unordered/task lists, > block quotes and fenced ` + "```lang" + ` code blocks all render natively
• Footnotes ([^1]) and LaTeX ($x^2$, $$E=mc^2$$) are supported
Do not avoid tables or headings — they look good here.
This note is invisible to the user.]

`
