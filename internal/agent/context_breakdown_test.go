package agent

import (
	"strings"
	"testing"

	"github.com/EvilFreelancer/coddy-agent/internal/config"
	"github.com/EvilFreelancer/coddy-agent/internal/llm"
	"github.com/EvilFreelancer/coddy-agent/internal/session"
)

func TestComputeContextBreakdownSystemPromptNonZero(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.ApplyDefaults()
	cfg.Prompts.ApplyDefaults()
	st := &session.State{ID: "t", CWD: t.TempDir(), Mode: session.ModeAgent}
	a := NewAgent(cfg, st, nil, nil)
	toolsMD := "## Tools\n\ntool_a: does things"
	_ = toolsMD
	_ = a.buildSystemPrompt("agent", nil, []llm.ToolDefinition{{Name: "tool_a", Description: "does things"}}, "", nil)
	b := st.GetLastContextBreakdown()
	if b == nil {
		t.Fatal("expected breakdown")
	}
	if b.SystemPrompt <= 0 {
		t.Fatalf("expected system prompt tokens > 0, got %+v", b)
	}
	if b.ToolDefinitions <= 0 {
		t.Fatalf("expected tool definition tokens > 0, got %+v", b)
	}
	// Sanity: system includes agent.md body text.
	if b.SystemPrompt < 100 {
		t.Fatalf("system prompt estimate too small: %d", b.SystemPrompt)
	}
}

func TestComputeContextBreakdownSubtractsParts(t *testing.T) {
	full := strings.Repeat("x", 400) + "\n\n" + strings.Repeat("y", 200)
	skills := strings.Repeat("s", 100)
	tools := strings.Repeat("t", 80)
	rules := strings.Repeat("r", 40)
	b := computeContextBreakdown(full, skills, tools, rules, nil, nil)
	if b.SystemPrompt <= 0 {
		t.Fatalf("system tokens: %d", b.SystemPrompt)
	}
	if b.Skills != session.EstimateTokens(skills) {
		t.Fatalf("skills: got %d", b.Skills)
	}
}
