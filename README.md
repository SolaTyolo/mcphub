# mcphub

Private-deployment MCP Gateway written in Go. Define **Agents** with system prompts, response schemas, and linked MCP servers; run an LLM agent loop via any chat-completions compatible API. Optional Whisper STT for voice input.

Module: [`github.com/SolaTyolo/mcphub`](https://github.com/SolaTyolo/mcphub)

[中文文档](README.zh-CN.md)

## Features

- Agent-centric configuration (system prompt, JSON Schema / response description, MCP server bindings)
- Postgres-backed Agent and MCP server configuration
- Official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) client (`stdio`, `http` transports)
- HTTP MCP uses [SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient) (retryable HTTP)
- Chat-completions compatible LLM client — works with Ollama, DeepSeek, OpenAI, etc.
- Optional Whisper sidecar for speech-to-text (`/v1/audio/transcriptions`)
- REST API + embedded static chat test page
- Docker Compose (Postgres + Ollama + Whisper + mcphub)

## Quick start

```bash
cp .env.example .env
make infra-up
make migrate
make ollama-pull
make dev         # http://localhost:8090
make seed        # creates filesystem MCP + fs-agent
```

Or run everything in Docker:

```bash
cp .env.example .env
make up
make migrate
make seed
```

## Storage backends

Configure a single `LLM_STORE_DSN` using `scheme://path`. Backend type is inferred from the scheme and file extension:

| DSN example | Backend | Use case |
|-------------|---------|----------|
| `postgres://user:pass@host/db?sslmode=disable` | Postgres | Production / Docker Compose |
| `sqlite://./data/llm.db` or `file:./data/llm.db` | SQLite | Local single-file DB, no Postgres |
| `file://./data/store.yml` | YAML file | Zero deps, edit by hand / GitOps |

If unset, defaults to `file://./data/store.yml`. Legacy env `LLM_DB_DSN` is accepted as a fallback; `LLM_DATA_FILE` is mapped to `file://…` when `LLM_STORE_DSN` is empty.

```bash
# YAML — zero DB setup
make dev-file
cp data/store.example.yml data/store.yml

# SQLite
LLM_STORE_DSN=sqlite://./data/llm.db make migrate
make dev-sqlite
```

File store skips SQL migrations. Postgres uses `migrations/postgres/`; SQLite uses `migrations/sqlite/`.

## LLM configuration

All backends use the same chat-completions compatible API. Global env defaults apply; each Agent can override `llmBaseUrl`, `llmApiKey`, `llmModel`, `visionModel`.

### Local dev (Ollama + Whisper)

```bash
LLM_BASE_URL=http://localhost:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen2.5:7b-instruct-q4_K_M      # text + tool calling
LLM_VISION_MODEL=llama3.2-vision         # auto when messages contain images
WHISPER_BASE_URL=http://localhost:8000/v1
```

```bash
make infra-up
make ollama-pull-local   # pulls text + vision models
make migrate && make dev && make seed
```

### Production (compatible API)

```bash
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=sk-...
LLM_MODEL=gpt-4o-mini
LLM_VISION_MODEL=gpt-4o    # same model often handles both
```

Per-request override: `{ "model": "gpt-4o", "messages": [...] }`.

## Multimodal messages

Chat accepts `content` as **string** or **multimodal array**:

```json
{
  "messages": [{
    "role": "user",
    "content": [
      { "type": "text", "text": "What's in this image?" },
      { "type": "image_url", "image_url": { "url": "data:image/jpeg;base64,..." } }
    ]
  }]
}
```

When messages contain `image_url`, the gateway selects `LLM_VISION_MODEL` (or agent `visionModel`). Text-only messages use `LLM_MODEL`.

Multipart chat (`POST /api/agents/{id}/chat`):

| Field | Description |
|-------|-------------|
| `audio` | Voice file → Whisper STT |
| `image` | Image file → base64 data URL |
| `text` | Optional prompt (with image) |
| `messages` | Optional JSON history |
| `model` | Optional per-request model override |

## Speech-to-text (Whisper)

Set `WHISPER_BASE_URL` (e.g. `http://localhost:8000/v1`). Docker Compose includes `fedirz/faster-whisper-server`.

- `POST /api/transcribe` — multipart `audio` file → `{ "text": "..." }`
- `POST /api/agents/{id}/chat` — multipart with `audio` + optional `messages` JSON

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/agents` | Create agent |
| GET | `/api/agents` | List agents |
| GET | `/api/agents/{id}` | Get agent |
| PUT | `/api/agents/{id}` | Update agent |
| DELETE | `/api/agents/{id}` | Delete agent |
| POST | `/api/agents/{id}/chat` | Chat (JSON or multipart audio) |
| POST | `/api/mcp-servers` | Add MCP server |
| GET | `/api/mcp-servers` | List MCP servers |
| PUT | `/api/mcp-servers/{serverId}` | Update MCP server |
| DELETE | `/api/mcp-servers/{serverId}` | Delete MCP server |
| POST | `/api/mcp-servers/{serverId}/test` | Test connection + list tools |
| POST | `/api/transcribe` | Transcribe audio file |

Protected routes require `X-API-Key` when `GATEWAY_API_KEY` is set.

### Agent example

```bash
# Create MCP server
curl -X POST http://localhost:8090/api/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{"name":"fs","transport":"stdio","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp"]}'

# Create agent
curl -X POST http://localhost:8090/api/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fs-agent",
    "systemPrompt": "You are a filesystem assistant.",
    "responseSchema": {"type":"object","properties":{"answer":{"type":"string"}}},
    "mcpServerIds": ["<server-uuid>"]
  }'

# Chat
curl -X POST http://localhost:8090/api/agents/<agent-id>/chat \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"List files in /tmp"}]}'
```

## Environment

See [`.env.example`](.env.example):

- `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`, `LLM_VISION_MODEL`
- `LLM_STORE_DSN` (storage: `postgres://`, `sqlite://`, `file://`)
- `GATEWAY_API_KEY` (optional)
- `WHISPER_BASE_URL`, `WHISPER_API_KEY`, `WHISPER_MODEL`
- `LLM_SERVER_ADDR`
- `MCP_IDLE_TTL`, `AGENT_MAX_ROUNDS`, `MCP_FS_ROOT`

## Database migrations

```bash
make migrate
```

Migrations: `001_init.sql` (legacy tenant schema) → `002_agent_refactor.sql` (agents, drop tenants).

## MCP tool naming

Tools from multiple servers are prefixed as `{server_name}__{tool_name}`.

## Notes

- Ollama tool calling works best with `llama3.1+` or `qwen2.5`.
- Supports `stdio` and `http` (Streamable HTTP) MCP transports.
- HTTP MCP uses [github.com/SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient) with retries.
- MCP sessions idle out after `MCP_IDLE_TTL`; config changes invalidate cached sessions.
