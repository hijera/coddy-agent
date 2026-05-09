# Long-term memory (Memory Copilot)

Implementation for Coddy lives in this directory (`external/memory`) and is **always linked** into the main `coddy` binary. Use **`memory.enabled`** in `config.yaml` to turn behavior on or off at runtime. There is no separate memory-only build.

## Build

`make build` produces `build/coddy` (see the Makefile header for optional **`TAGS`** and other build notes).

## Behaviour

In the LLM sense, "memory" is whatever is injected into the context. Short-term memory is the chat history. **Long-term** memory here means markdown files on disk that are turned into a short block **before** the main model answers, merged into the same template slot as session notes (`{{.Memory}}` in `agent.md` / `plan.md`).

When `memory.enabled` is true, Coddy also emits **ACP `session/update`** notifications (`memory_phase`, `memory_message_chunk`) and persists a per-session **`memory_trace.json`** alongside `messages.json`. That trace and the HTTP **`memoryTurns`** field on **`GET /coddy/sessions/{id}/messages`** are for UI observability only - they are **not** part of the Chat Completions transcript sent to the primary model.

A separate **memory copilot** (extra `llm.Stream` / completion passes with native tool calling) uses only **`coddy_memory_*`** tools:

- **Recall** (before the main reply) - **`coddy_memory_search`**, **`coddy_memory_list`**, **`coddy_memory_read`** only. It is expected to **find entry notes, then read linked paths in further rounds** until enough context is loaded. The final plain-text answer should separate what is **already on disk** (from files actually read) from what is **not covered** by those notes, so the main model knows the gap. Output is merged into `{{.Memory}}` after the copilot stops calling tools.
- **Persist** (after the final assistant message in a user turn, when there are no pending tool calls) - curator uses **`PersistToolDefinitions`**: search/list/read (including link-following) plus **`coddy_memory_mkdir`**, **`coddy_memory_save`**, **`coddy_memory_delete`**. It should **read existing relevant notes first**, then save only **net-new** durable facts not already present. The curator ends with plain text (no tools) summarizing verification and save/skip; **`coddy_memory_save`** body length is capped.

Cross-links inside stored bodies should use **`scope:relative/path.md`** (or Markdown targets with that form) so paths stay unambiguous across global vs project roots.

The main ReAct loop **does not** receive these tool definitions and cannot call memory as a normal tool.

## Storage layout

- **Global** (shared across sessions): `memory.dir` in config. When `dir` is empty or unset, the root is **`$CODDY_HOME/memory`** (typically `~/.coddy/memory`). Values support `${CODDY_HOME}` and `~` expansion like other paths in config.
- **Project** (per workspace): always **`<session cwd>/memory`**. This path is not configurable.

Supported file extensions: `.md` and `.txt`. `coddy_memory_search` ranks nested files under each root by word overlap with the query. Subdirectories are encouraged for thematic grouping; use **`coddy_memory_mkdir`** before saving into a new folder branch.

REST endpoints under **`/coddy/sessions/{id}/memory/*`** expose the same tree for the SPA and mirror filesystem layout produced by copilot tools.

## Configuration (`memory`)

See `config.example.yaml` and `docs/config.md`. Fields:

- `enabled` - master switch at runtime.
- `model` - optional exact `models[].model` id for **recall and persist only**; does not change the main agent. Pin it (for example `rpa/gpt-oss:120b`) when memory should stay on a fixed model regardless of `agent.model`. Empty uses the active session / `agent.model`.
- `dir`, `recall_max_turns`, `persist_max_turns`, `copilot_max_tokens`, `max_search_hits` - see the example config comments.

## Cost and latency

Each user turn with memory enabled adds at least recall LLM rounds when memory files exist, plus persist rounds when the turn ends cleanly. Both steps use English system prompts in `copilot.go`. Persist tool rounds are capped by **`persist_max_turns`**.

## Code layout

- `store.go` - roots, search, read/write (flat slug or **`relative_path`**, nested dirs), mkdir, listing, delete.
- `tools.go` - tool schemas (`RecallToolDefinitions`, `PersistToolDefinitions`) and **`execTool`**.
- `copilot.go` - recall persist loops with **`llm.Complete`/`Stream`** and tool callbacks.

Runtime wiring: `internal/agent/memory_hooks.go` imports this package.
