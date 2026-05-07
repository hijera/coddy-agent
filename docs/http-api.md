# OpenAI-compatible HTTP API

The `coddy http` subcommand is available only when the binary is built with **`go build -tags=http`** (or `make build TAGS=http`). It exposes a subset of the [OpenAI REST shape](https://github.com/openai/openai-openapi/blob/manual_spec/openapi.yaml) backed by the same session manager and ReAct agent as **`coddy acp`**.

## OpenAPI and Swagger UI

When the HTTP server runs (build with **`http`** tag), the API description is generated on each request from the same source as the handlers, so it stays aligned with the live routes.

- **`GET /openapi.yaml`** - OpenAPI 3.0 document (YAML). **`info.version`** matches the embedded build version (`make build` / **`-ldflags -X ...version.Version=...`**). Served as **`text/yaml`** with **`Content-Disposition: inline`** so the browser shows it in the tab instead of downloading. Use **`GET /openapi.json`** for JSON (same **inline** behavior).
- **`GET /docs`** - Swagger UI from the **unpkg** CDN loads **`/openapi.yaml`** from your server.

No authentication is enforced on these URLs (same as **`/v1/*`** ignore dummy API keys). Run offline or behind strict networks only if pulling scripts from CDN is blocked, in which case serve the Swagger UI bundle yourself.

## Endpoints

| Method | Path | Notes |
|--------|------|--------|
| GET | `/openapi.yaml` | Generated OpenAPI 3 (YAML); version matches **`coddy -v`** when built via **`make`**. |
| GET | `/openapi.json` | Same document as JSON. |
| GET | `/docs`, `/docs/` | Swagger UI (CDN); points at **`/openapi.yaml`**. |
| GET | `/v1/models` | Lists Coddy modes **`agent`** and **`plan`** (OpenAI-shaped objects; **`owned_by`** is **`coddy-mode`**). Configure LLMs in YAML **`models`** and pass a **`models[].model`** string in **`model`** when calling chat or responses if you pick a backend explicitly. |
| POST | `/v1/chat/completions` | Chat; **`model`** is **`agent`**, **`plan`**, or a **`models[].model`** selector from config (mode sets session profile without changing which LLM id is resolved; effective LLM stays **`agent.model`** and optional session overrides). Supports **`stream: true`** (SSE) or non-streaming JSON. |
| POST | `/v1/responses` | MVP: **`model`** rules match chat completions; **`input`** is plain user text (simplified vs full OpenAI Responses API). |
| GET | `/v1/responses/{id}` | MVP: returns metadata if **`id`** is an active session id. |

## Session behavior

- Without header: each `chat.completions` request that needs a new session calls ACP-style **`session/new`** (default cwd from **`--cwd`** / `CODDY_CWD`).
- **`X-Coddy-Session-ID`**: use an existing in-memory session (returns **404** if unknown).
- On the first response for a newly created session, the server may add **`X-Coddy-Session-ID`** (non-streaming and streaming) so clients can continue server-side history.

**Stateless mode (full `messages` every time)**: send the full OpenAI `messages` array; the last message must be **`user`**. Earlier messages become session prefix; the last user line is the new turn (same as the HTTP integration path in the agent).

**`/v1/models` vs completions `model`**: **`GET /v1/models`** only exposes session profiles. Passing **`agent`** or **`plan`** as **`model`** switches that profile for this session turn (same tooling rules as **`coddy acp`** modes). Passing a **`models[].model`** value keeps the usual LLM picker behavior.

There is no interactive permission UI on HTTP. **`tools.permission_master_key`** bypasses prompts for both ACP and HTTP. Without it, gated tools that require confirmation will fail the turn unless session **`permission_grants.json`** already contains matching **`allow_always`** grants from a prior ACP session on disk.

## CLI

Flags match **`coddy acp`** where applicable (`--config`, `--home`, `--cwd`, `--sessions-dir`, `--disable-session`, `--session-id`, `--log-*`), plus:

- **`-H` / `--host`**: bind address (built-in default **`0.0.0.0`** unless **`httpserver.host`** overrides when flags stay at **`0.0.0.0`** and **`12345`**)
- **`-P` / `--port`**: port (built-in default **`12345`** unless **`httpserver.port`** overrides in the same case)

YAML **`httpserver.host`** and **`httpserver.port`** apply only when **`--host`** and **`--port`** are still exactly **`0.0.0.0`** and **`12345`**. Passing `-H`/`-P` always wins.

## Official client (Python)

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:12345/v1", api_key="dummy")
# Returns session modes agent and plan, not YAML models[]. To target an LLM, pass agent.model-compatible selector:
client.chat.completions.create(model="agent", messages=[{"role": "user", "content": "hi"}])
# Or pass models[].model, e.g. model="openai/gpt-4o", unchanged from config.
```

Clients may still send **`api_key`**; Coddy ignores it for HTTP.

## Build

```bash
make build TAGS=http
# binary: build/coddy
```

Default **`make build`** does not include HTTP; `go test ./...` also skips the HTTP package unless you run **`go test -tags=http ./...`**.
