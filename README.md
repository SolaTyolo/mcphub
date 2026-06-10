# mcphub

Private-deployment MCP Gateway written in Go. Define **Agents** with system prompts, response schemas, and linked MCP servers; run an LLM agent loop via any chat-completions compatible API. Optional Whisper STT for voice input.

Module: [`github.com/SolaTyolo/mcphub`](https://github.com/SolaTyolo/mcphub)

[中文文档](README.zh-CN.md)

## Features

- Agent-centric configuration (system prompt, JSON Schema / response description, MCP server bindings)
- Pluggable storage (YAML file / SQLite / Postgres)
- Official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) client (`stdio`, `http` transports)
- HTTP MCP uses [SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient) (retryable HTTP)
- Chat-completions compatible LLM client — works with Ollama, DeepSeek, OpenAI, etc.
- Optional Whisper sidecar for speech-to-text (`/v1/audio/transcriptions`)
- REST API + embedded static chat test page
- Docker Compose (Postgres + Ollama + Whisper + mcphub)

## Quick start (file store, no database)

```bash
cp .env.example .env
# .env defaults to LLM_STORE_DSN=file://./data/store.yml
cp data/store.example.yml data/store.yml

make infra-up          # Ollama + Whisper (optional, for local LLM / voice)
make ollama-pull-local
make dev-file          # http://localhost:8090
make seed              # filesystem MCP + text/vision agents
```

agents and MCP servers live in `./data/store.yml` (editable by hand).

### Docker Compose (Postgres production)

```bash
cp .env.example .env
# set LLM_STORE_DSN=postgres://llm:llm@localhost:5433/llm?sslmode=disable
make up
make migrate
make seed
```

## Storage backends

Configure a single `LLM_STORE_DSN` using `scheme://path`. Backend type is inferred from the scheme and file extension:

| DSN example | Backend | Use case |
|-------------|---------|----------|
| `file://./data/store.yml` | YAML file | **Local quick start**, zero deps, edit by hand / GitOps |
| `sqlite://./data/llm.db` or `file:./data/llm.db` | SQLite | Local single-file DB, no Postgres |
| `postgres://user:pass@host/db?sslmode=disable` | Postgres | Production / Docker Compose |

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
make ollama-pull-local
make dev-file && make seed
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
| `audio` | Voice file → Whisper STT (**eager**) |
| `image` | Image file → base64 data URL (**eager**, vision model) |
| `document` / `file` | Office file → stored as attachment (**lazy**, AI parses via builtin tool) |
| `text` | Optional prompt |
| `attachmentIds` | JSON array of pre-uploaded attachment IDs |
| `messages` | Optional JSON history |
| `model` | Optional per-request model override |

Upload attachments separately via `POST /api/attachments` (multipart `file` field).

**Hybrid ingress:** audio and images are preprocessed immediately; documents are stored in the attachment backend and parsed on demand by builtin tools `mcphub__list_attachments` and `mcphub__parse_document` when `MARKITDOWN_MCP_URL` is set ([MarkItDown MCP](https://github.com/microsoft/markitdown/tree/main/packages/markitdown-mcp): Office, PDF, CSV, JSON, images, audio, and more).

### Document parsing (MarkItDown)

Set `MARKITDOWN_MCP_URL` (e.g. `http://localhost:3001/mcp`). Docker Compose includes the official [`mcp/markitdown`](https://hub.docker.com/r/mcp/markitdown) sidecar (Streamable HTTP).

- mcphub calls MarkItDown internally via MCP; agents still use `mcphub__parse_document(attachment_id)` — no URI handling in prompts
- Leave `MARKITDOWN_MCP_URL` empty to disable document parsing tools

### Attachment store

Configure `ATTACHMENT_STORE_DSN`:

| DSN | Backend |
|-----|---------|
| `file://./data/attachments` | Local directory (default) |
| `s3://key:secret@host:9000/bucket/prefix?path_style=true` | S3-compatible (RustFS / MinIO) |

Dev RustFS: `docker compose -f deploy/docker-compose.yml up -d rustfs`

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
| POST | `/api/agents/{id}/chat` | Chat (JSON or multipart: audio / image / document) |
| POST | `/api/mcp-servers` | Add MCP server |
| GET | `/api/mcp-servers` | List MCP servers |
| PUT | `/api/mcp-servers/{serverId}` | Update MCP server |
| DELETE | `/api/mcp-servers/{serverId}` | Delete MCP server |
| POST | `/api/mcp-servers/{serverId}/test` | Test connection + list tools |
| POST | `/api/transcribe` | Transcribe audio file |
| POST | `/api/attachments` | Upload attachment (returns id) |

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
- `ATTACHMENT_STORE_DSN` (attachments: `file://`, `s3://`)
- `GATEWAY_API_KEY` (optional)
- `WHISPER_BASE_URL`, `WHISPER_API_KEY`, `WHISPER_MODEL` (STT enabled when `WHISPER_BASE_URL` is set)
- `MARKITDOWN_MCP_URL` (document parsing enabled when set; e.g. `http://localhost:3001/mcp`)
- `LLM_SERVER_ADDR`
- `MCP_IDLE_TTL`, `AGENT_MAX_ROUNDS`, `MCP_FS_ROOT`

## Database migrations

Only **Postgres / SQLite** need migrations; file store skips them.

```bash
# Postgres (Docker Compose)
LLM_STORE_DSN=postgres://llm:llm@localhost:5433/llm?sslmode=disable make migrate

# SQLite
LLM_STORE_DSN=sqlite://./data/llm.db make migrate
```

Migration files: `migrations/postgres/001_init.sql`, `migrations/sqlite/001_init.sql`.

## MCP tool naming

Tools from multiple servers are prefixed as `{server_name}__{tool_name}`.

## Notes

- Ollama tool calling works best with `llama3.1+` or `qwen2.5`.
- Supports `stdio` and `http` (Streamable HTTP) MCP transports.
- HTTP MCP uses [github.com/SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient) with retries.
- MCP sessions idle out after `MCP_IDLE_TTL`; config changes invalidate cached sessions.
- **Mac / Windows local:** If you only need a local OpenAI-compatible API and don't require MCP Agent orchestration, consider [LocalAI](https://github.com/mudler/LocalAI) as a lighter alternative to this project. To keep using mcphub without Docker Ollama, point `LLM_BASE_URL` at LocalAI (e.g. `http://localhost:8080/v1`).
