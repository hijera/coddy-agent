package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/EvilFreelancer/coddy-agent/internal/config"
	"github.com/EvilFreelancer/coddy-agent/internal/llm"
)

const recallSystem = `You are the memory retrieval step for a coding agent. You never speak to the end user directly.
Your only job is to load useful long-term notes from disk BEFORE the main assistant answers.

Tools are READ-ONLY in this phase: search, list, read. Do not save, mkdir, edit, or delete here; persistence is handled by the curator after the main reply.

Paths are always scope:relative where scope is global or project (example global:preferences.md or project:architecture/api.md).
Global memory uses memory.dir from config when set, otherwise $CODDY_HOME/memory (often ~/.coddy/memory). Project memory is cwd/memory.

Folders: use coddy_memory_list to inspect layout; thematic subfolders mirror how notes are organized on disk.

Cross-links inside note bodies use the same scope:relative form or Markdown links targeting that path.

Workflow (use several tool rounds if needed):
1. coddy_memory_search (and list when layout is unclear) to find entry files relevant to the current user message.
2. coddy_memory_read those files. When a body references other memory paths you still need, read those in follow-up rounds until you have enough context for this turn.
3. Finish with plain text only (no more tool calls). Structure the answer so the main assistant can tell:
   - "Already on disk" - short factual bullets grounded ONLY in what you actually read (no invention).
   - "Not in notes" - optional short bullets for user intent or facts that nothing you read covered, so the main model knows what is not yet in long-term memory (do not guess file contents you did not open).

Rules:
- Call coddy_memory_search first unless you already know exact paths.
- Call coddy_memory_list when you need the directory layout.
- Prefer short factual bullets. Do not expose raw tool JSON.
- Do not invent facts you did not read from files.
- If nothing is relevant, reply with a single line exactly: (no memory hits)
- When you finish gathering, answer in plain text without further tool calls.`

const persistSystem = `You are a strict memory curator for a coding agent. You never speak to the end user directly.

You MAY call coddy_memory_search, coddy_memory_list, coddy_memory_read, coddy_memory_mkdir, coddy_memory_save, and coddy_memory_delete during this phase.

Before writing, discover what is already stored:
- Use search, list, and a short chain of reads (including linked paths) like in recall so you see existing notes relevant to this user turn and assistant reply.

Then decide what is NET-NEW durable information compared to files you actually read:
- If the fact or preference already exists on disk (same meaning), skip coddy_memory_save unless the user clearly asked to revise or replace it.
- Save only substantive gaps: things the assistant stated that belong in long-term memory and are missing or outdated in what you read.

Prefer thematic folders: call coddy_memory_mkdir before the first coddy_memory_save under a new path.
Use scope-relative paths everywhere (global:... or project:...).

When linking between notes inside bodies, prefer scope:relative paths or Markdown with the same target.

Secrets: never store API keys, tokens, passwords, or one-off credentials in coddy_memory_save body.

coddy_memory_save ONLY when one of these applies AND the idea is not already adequately covered by an existing note you read:
- The user explicitly asked to remember / store / save something for later sessions, and the assistant agreed to a concrete fact.
- The assistant stated a durable preference or project fact (stack, coding style, naming, architecture decision) that will clearly help future turns.

Do NOT save transient debugging, one-off errors, task status, duplicates, filler, or chat that is not reusable. When unsure, skip save.

When deciding is done, respond with plain text only (no tool calls). Summarize: what you verified on disk, what you saved or skipped (and why).`

func clampProviderMax(rm *config.ResolvedLLM, cap int) {
	if rm == nil || cap <= 0 {
		return
	}
	if rm.MaxTokens <= 0 || rm.MaxTokens > cap {
		rm.MaxTokens = cap
	}
}

func newCopilotProvider(cfg *config.Config, modelRef string) (llm.Provider, error) {
	ref := strings.TrimSpace(modelRef)
	if ref == "" {
		ref = strings.TrimSpace(cfg.Agent.Model)
	}
	rm, err := cfg.ResolveLLM(ref)
	if err != nil {
		return nil, err
	}
	cap := cfg.Memory.CopilotMaxTokens
	clampProviderMax(rm, cap)
	return llm.NewProvider(rm.ProviderType, rm.Model, rm.APIKey, rm.BaseURL, rm.MaxTokens, rm.Temperature)
}

// RunRecall runs the recall sub-agent and returns text for the main prompt memory section.
// When opts is non-nil, OnStream receives streamed model text and reasoning deltas (recall phase only).
// ReadPaths lists scope:relative paths successfully read via coddy_memory_read (deduped, order preserved).
// Returns final recall text, wall-clock duration in milliseconds, read paths, and error.
func RunRecall(ctx context.Context, log *slog.Logger, cfg *config.Config, cwd, userQuery, modelRef string, opts *RunRecallOptions) (string, int64, []string, error) {
	var readPaths []string
	if !cfg.Memory.Enabled {
		return "", 0, nil, nil
	}
	store, err := NewStore(&cfg.Memory, cfg.Paths, cwd)
	if err != nil {
		return "", 0, nil, err
	}
	if !store.HasAnyFiles() {
		return "", 0, nil, nil
	}
	prov, err := newCopilotProvider(cfg, modelRef)
	if err != nil {
		return "", 0, nil, err
	}
	tools := RecallToolDefinitions()
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: recallSystem},
		{Role: llm.RoleUser, Content: "User message for this turn:\n" + userQuery},
	}
	max := cfg.Memory.RecallMaxTurns
	recallStarted := timeNowMs()
	if opts != nil && opts.OnPhaseStart != nil {
		opts.OnPhaseStart()
	}
	for step := 0; step < max; step++ {
		if ctx.Err() != nil {
			return "", timeNowMs() - recallStarted, readPaths, ctx.Err()
		}
		var resp *llm.Response
		if opts != nil && opts.OnStream != nil {
			resp, err = runRecallStreamRound(ctx, prov, msgs, tools, opts.OnStream)
		} else {
			resp, err = prov.Complete(ctx, msgs, tools)
		}
		if err != nil {
			return "", timeNowMs() - recallStarted, readPaths, err
		}
		if len(resp.ToolCalls) == 0 {
			out := strings.TrimSpace(resp.Content)
			dur := timeNowMs() - recallStarted
			if out == "" {
				return "", dur, readPaths, nil
			}
			return out, dur, readPaths, nil
		}
		msgs = append(msgs, llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, tc := range resp.ToolCalls {
			res, ex := execTool(store, &cfg.Memory, tc.Name, tc.InputJSON)
			if tc.Name == "coddy_memory_read" && ex == nil {
				var ra struct {
					Path string `json:"path"`
				}
				if uerr := json.Unmarshal([]byte(tc.InputJSON), &ra); uerr == nil {
					readPaths = appendRecallReadPath(readPaths, ra.Path)
				}
			}
			if ex != nil {
				res = "error: " + ex.Error()
			}
			msgs = append(msgs, llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: res})
		}
	}
	if log != nil {
		log.Warn("memory recall exceeded max turns")
	}
	dur := timeNowMs() - recallStarted
	return "", dur, readPaths, nil
}

// RunRecallOptions configures optional streaming hooks for recall.
type RunRecallOptions struct {
	// OnStream receives text or reasoning deltas from the model (not tool JSON).
	OnStream func(kind StreamKind, delta string)
	// OnPhaseStart is invoked once before the first LLM call (for wall-clock UI).
	OnPhaseStart func()
}

// StreamKind discriminates streamed memory copilot content.
type StreamKind string

const (
	StreamKindText      StreamKind = "text"
	StreamKindReasoning StreamKind = "reasoning"
)

func timeNowMs() int64 { return time.Now().UnixMilli() }

func appendRecallReadPath(slice []string, p string) []string {
	p = strings.TrimSpace(p)
	if p == "" {
		return slice
	}
	for _, x := range slice {
		if x == p {
			return slice
		}
	}
	return append(slice, p)
}

func runRecallStreamRound(ctx context.Context, prov llm.Provider, msgs []llm.Message, tools []llm.ToolDefinition, onStream func(kind StreamKind, delta string)) (*llm.Response, error) {
	resp, err := prov.Stream(ctx, msgs, tools, func(ch llm.StreamChunk) {
		if onStream == nil {
			return
		}
		if ch.TextDelta != "" {
			onStream(StreamKindText, ch.TextDelta)
		}
		if ch.ReasoningDelta != "" {
			onStream(StreamKindReasoning, ch.ReasoningDelta)
		}
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// PersistOutcome is the structured result after the memory curator runs.
type PersistOutcome struct {
	Saved        bool
	Scope        string
	RelativePath string // path under memory root written (scope:rel) when Saved
	Title        string
	Body         string // markdown body written when Saved (trimmed, for UI)
	Reason       string
	RawJudge     string // full final curator text (trace / UI)
}

// RunPersistOptions configures optional hooks for persist streaming.
type RunPersistOptions struct {
	OnPhaseStart func()
	OnStream     func(kind StreamKind, delta string)
}

// RunPersist optionally writes memory via tool-calling curator after a user turn.
func RunPersist(ctx context.Context, log *slog.Logger, cfg *config.Config, cwd, modelRef, userQuery, assistantReply string, opts *RunPersistOptions) (PersistOutcome, int64, error) {
	out := PersistOutcome{}
	if !cfg.Memory.Enabled {
		return out, 0, nil
	}
	assistantReply = strings.TrimSpace(assistantReply)
	if assistantReply == "" {
		return out, 0, nil
	}
	store, err := NewStore(&cfg.Memory, cfg.Paths, cwd)
	if err != nil {
		return out, 0, err
	}
	prov, err := newCopilotProvider(cfg, modelRef)
	if err != nil {
		return out, 0, err
	}
	userPayload := fmt.Sprintf("User:\n%s\n\nAssistant:\n%s\n", userQuery, assistantReply)
	tools := PersistToolDefinitions()
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: persistSystem},
		{Role: llm.RoleUser, Content: userPayload},
	}
	persistStarted := timeNowMs()
	if opts != nil && opts.OnPhaseStart != nil {
		opts.OnPhaseStart()
	}
	max := cfg.Memory.PersistMaxTurns
	type saveCapture struct {
		scopeLabel   string
		relativePath string
		title        string
		body         string
	}
	var lastSave *saveCapture
	for step := 0; step < max; step++ {
		if ctx.Err() != nil {
			return out, timeNowMs() - persistStarted, ctx.Err()
		}
		var resp *llm.Response
		if opts != nil && opts.OnStream != nil {
			resp, err = runRecallStreamRound(ctx, prov, msgs, tools, opts.OnStream)
		} else {
			resp, err = prov.Complete(ctx, msgs, tools)
		}
		if err != nil {
			return out, timeNowMs() - persistStarted, err
		}
		if len(resp.ToolCalls) == 0 {
			finalText := strings.TrimSpace(resp.Content)
			out.RawJudge = finalText
			out.Reason = finalText
			if lastSave != nil {
				out.Saved = true
				out.Scope = lastSave.scopeLabel
				out.RelativePath = lastSave.relativePath
				out.Title = lastSave.title
				out.Body = lastSave.body
				if log != nil {
					log.Info("memory saved", "path", lastSave.relativePath)
				}
			}
			return out, timeNowMs() - persistStarted, nil
		}
		msgs = append(msgs, llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, tc := range resp.ToolCalls {
			res, ex := execTool(store, &cfg.Memory, tc.Name, tc.InputJSON)
			if ex != nil {
				res = "error: " + ex.Error()
			} else if tc.Name == "coddy_memory_save" {
				var args struct {
					Title        string `json:"title"`
					Body         string `json:"body"`
					Scope        string `json:"scope"`
					RelativePath string `json:"relative_path"`
				}
				if uerr := json.Unmarshal([]byte(tc.InputJSON), &args); uerr == nil {
					written := strings.TrimPrefix(strings.TrimSpace(res), "saved as")
					written = strings.TrimSpace(written)
					body := strings.TrimSpace(args.Body)
					if len(body) > 900 {
						body = body[:900] + "\n..."
					}
					lastSave = &saveCapture{
						scopeLabel:   strings.ToLower(strings.TrimSpace(args.Scope)),
						relativePath: written,
						title:        strings.TrimSpace(args.Title),
						body:         body,
					}
				}
			}
			msgs = append(msgs, llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: res})
		}
	}
	if log != nil {
		log.Warn("memory persist exceeded max turns")
	}
	dur := timeNowMs() - persistStarted
	if lastSave != nil {
		out.Saved = true
		out.Scope = lastSave.scopeLabel
		out.RelativePath = lastSave.relativePath
		out.Title = lastSave.title
		out.Body = lastSave.body
		out.Reason = "persist stopped at max turns after a save"
		out.RawJudge = out.Reason
	}
	return out, dur, nil
}
