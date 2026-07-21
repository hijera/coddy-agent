package agent

// Context compaction: summarize older conversation history with an LLM call
// and insert the summary into the transcript so later prompts replay only the
// summary plus the most recent turns (see session.MessagesForLLM).

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/EvilFreelancer/coddy-agent/internal/llm"
	"github.com/EvilFreelancer/coddy-agent/internal/session"
)

// ErrNothingToCompact is returned when the history has no full user turn to
// fold away before the keep-recent boundary.
var ErrNothingToCompact = errors.New("nothing to compact")

// CompactionResult reports what a successful compaction did.
type CompactionResult struct {
	// Summary is the generated summary text (without the transcript preamble).
	Summary string
	// CompactedMessages is how many history messages were folded into the summary.
	CompactedMessages int
	// KeptMessages is how many messages after the summary stayed verbatim.
	KeptMessages int
	// Model is the models[].model that produced the summary.
	Model string
}

// compactionSystemPrompt instructs the summarizer model.
const compactionSystemPrompt = `You are compacting the conversation history of a coding agent so the session can continue in a smaller context window.

Write a dense summary of the transcript you are given. Preserve, in this order:
1. The user's goals, requirements, and constraints (including exact wording of still-relevant instructions).
2. Decisions made and their reasons; approaches that were rejected.
3. Current state of the work: what is done, what is in progress, what failed.
4. Exact file paths, function/type names, commands, and configuration values that matter for continuing.
5. Unresolved questions and concrete next steps.

Output plain markdown, no preamble and no closing remarks. Do not invent facts that are not in the transcript.`

// CompactSession summarizes history older than the keep-recent boundary and
// inserts the summary row at that boundary. instructions optionally augments
// the summarization request (from the manual compact command arguments).
func (a *Agent) CompactSession(ctx context.Context, instructions string) (*CompactionResult, error) {
	if !a.cfg.Compaction.IsEnabled() {
		return nil, fmt.Errorf("compaction is disabled (compaction.enabled)")
	}

	msgs := a.state.GetMessages()
	keep := a.cfg.Compaction.EffectiveKeepRecentTurns()
	splitIdx, ok := session.CompactionSplitIndex(msgs, keep)
	if !ok {
		return nil, ErrNothingToCompact
	}
	head := session.MessagesForLLM(msgs[:splitIdx])

	provider, modelID, err := a.compactionProvider()
	if err != nil {
		return nil, fmt.Errorf("compaction model: %w", err)
	}

	resp, err := provider.Complete(ctx, buildCompactionRequest(head, instructions), nil)
	if err != nil {
		return nil, fmt.Errorf("compaction LLM call: %w", err)
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return nil, fmt.Errorf("compaction produced an empty summary")
	}

	a.state.InsertCompactionSummary(splitIdx, session.NewCompactionSummaryMessage(summary, modelID))

	return &CompactionResult{
		Summary:           summary,
		CompactedMessages: len(head),
		KeptMessages:      len(msgs) - splitIdx,
		Model:             modelID,
	}, nil
}

// compactionProvider resolves the summarizer provider: compaction.model when
// set, otherwise the session's effective model.
func (a *Agent) compactionProvider() (llm.Provider, string, error) {
	modelID := strings.TrimSpace(a.cfg.Compaction.Model)
	if modelID == "" {
		modelID = a.state.EffectiveModelID(a.cfg)
	}
	if modelID == "" {
		return nil, "", fmt.Errorf("no model configured")
	}
	rm, err := a.cfg.ResolveLLM(modelID)
	if err != nil {
		return nil, "", err
	}
	mk := a.providerFactory
	if mk == nil {
		mk = llm.NewProvider
	}
	provider, err := mk(a.llmProviderInput(rm))
	if err != nil {
		return nil, "", err
	}
	return provider, modelID, nil
}

// buildCompactionRequest flattens the head of the conversation into a single
// summarization request. Tool calls and results are rendered as labeled lines
// so the summarizer sees what happened without replaying structured calls.
func buildCompactionRequest(head []llm.Message, instructions string) []llm.Message {
	var b strings.Builder
	b.WriteString("Summarize the following conversation transcript.\n\n<transcript>\n")
	for _, m := range head {
		if m.PlanDocument != nil && strings.TrimSpace(m.Content) == "" && len(m.ToolCalls) == 0 {
			continue
		}
		b.WriteString(string(m.Role))
		if m.CompactionSummary {
			b.WriteString(" (earlier summary)")
		}
		b.WriteString(":\n")
		if strings.TrimSpace(m.Content) != "" {
			b.WriteString(m.Content)
			b.WriteString("\n")
		}
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&b, "[tool call] %s %s\n", tc.Name, tc.InputJSON)
		}
		b.WriteString("\n")
	}
	b.WriteString("</transcript>")
	if s := strings.TrimSpace(instructions); s != "" {
		b.WriteString("\n\nAdditional instructions from the user for this summary:\n")
		b.WriteString(s)
	}
	return []llm.Message{
		{Role: llm.RoleSystem, Content: compactionSystemPrompt},
		{Role: llm.RoleUser, Content: b.String()},
	}
}
