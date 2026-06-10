# mcphub

Go 实现的私有化 MCP Gateway。定义 **Agent**（system prompt、返回格式、关联 MCP Server），通过 Chat Completions 兼容 API 运行 LLM Agent 循环。可选 Whisper 语音转文字。

模块：[`github.com/SolaTyolo/mcphub`](https://github.com/SolaTyolo/mcphub)

[English README](README.md)

## 功能

- Agent 中心化配置（system prompt、JSON Schema / 返回描述、MCP Server 绑定）
- Postgres 存储 Agent 与 MCP Server
- 官方 [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)（stdio / http）
- HTTP MCP 使用 [SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient)
- Chat Completions 兼容 LLM 客户端（Ollama、DeepSeek、OpenAI 等）
- 可选 Whisper 侧车 STT（`/v1/audio/transcriptions`）
- REST API + 内嵌 Chat 测试页
- Docker Compose（Postgres + Ollama + Whisper + mcphub）

## 快速开始

```bash
cp .env.example .env
make infra-up
make migrate
make ollama-pull
make dev         # http://localhost:8090
make seed        # 创建 filesystem MCP + fs-agent
```

## 存储后端

只需配置一个 `LLM_STORE_DSN`，格式为 `scheme://path`，根据 scheme 与扩展名自动选择后端：

| DSN 示例 | 后端 | 场景 |
|----------|------|------|
| `postgres://user:pass@host/db?sslmode=disable` | Postgres | 生产 / Docker Compose |
| `sqlite://./data/llm.db` 或 `file:./data/llm.db` | SQLite | 本地单文件，无需 Postgres |
| `file://./data/store.yml` | YAML 文件 | 零依赖，可手改或 Git 管理 |

未设置时默认 `file://./data/store.yml`。旧变量 `LLM_DB_DSN` 仍可作为回退；若只设置了 `LLM_DATA_FILE`，会自动转为 `file://…`。

```bash
# YAML 模式
make dev-file
cp data/store.example.yml data/store.yml

# SQLite
LLM_STORE_DSN=sqlite://./data/llm.db make migrate
make dev-sqlite
```

`file` 模式不需要跑 SQL 迁移。Postgres 用 `migrations/postgres/`，SQLite 用 `migrations/sqlite/`。

## LLM 配置

统一 Chat Completions 兼容 API。全局 env 为默认；每个 Agent 可覆盖 `llmBaseUrl`、`llmApiKey`、`llmModel`、`visionModel`。

### 本地开发（Ollama + Whisper）

```bash
LLM_BASE_URL=http://localhost:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen2.5:7b-instruct-q4_K_M      # 文本 / tool calling
LLM_VISION_MODEL=llama3.2-vision         # 消息含图片时自动选用
WHISPER_BASE_URL=http://localhost:8000/v1
```

```bash
make infra-up
make ollama-pull-local
make migrate && make dev && make seed
```

seed 会创建 `text-agent`（文本+MCP）和 `vision-agent`（图文）。

### 生产环境（兼容 API）

```bash
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=sk-...
LLM_MODEL=gpt-4o-mini
LLM_VISION_MODEL=gpt-4o
```

请求级覆盖：`{ "model": "gpt-4o", "messages": [...] }`。

## 多模态消息

`content` 支持 **字符串** 或 **多模态内容数组**（text + image_url）。含图片时自动路由到 vision 模型。

multipart：`audio`（Whisper）、`image`（图片）、`text`（可选提示词）。

可选网关鉴权：`GATEWAY_API_KEY` + `X-API-Key`。

## 语音转文字（Whisper）

配置 `WHISPER_BASE_URL`（如 `http://localhost:8000/v1`）。Docker Compose 已包含 `fedirz/faster-whisper-server`。

- `POST /api/transcribe` — multipart `audio` → `{ "text": "..." }`
- `POST /api/agents/{id}/chat` — multipart 上传 `audio`，可选 `messages` JSON 历史

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| POST | `/api/agents` | 创建 Agent |
| GET | `/api/agents` | Agent 列表 |
| GET | `/api/agents/{id}` | 获取 Agent |
| PUT | `/api/agents/{id}` | 更新 Agent |
| DELETE | `/api/agents/{id}` | 删除 Agent |
| POST | `/api/agents/{id}/chat` | 对话（JSON 或 multipart 语音） |
| POST | `/api/mcp-servers` | 添加 MCP Server |
| GET | `/api/mcp-servers` | MCP Server 列表 |
| PUT | `/api/mcp-servers/{serverId}` | 更新 MCP Server |
| DELETE | `/api/mcp-servers/{serverId}` | 删除 MCP Server |
| POST | `/api/mcp-servers/{serverId}/test` | 测试连接 |
| POST | `/api/transcribe` | 语音转文字 |

设置 `GATEWAY_API_KEY` 后，受保护路由需 `X-API-Key` 请求头。

### Agent 示例

```bash
curl -X POST http://localhost:8090/api/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fs-agent",
    "systemPrompt": "你是文件系统助手。",
    "responseDescription": "用简洁中文回答。",
    "mcpServerIds": ["<server-uuid>"]
  }'

curl -X POST http://localhost:8090/api/agents/<agent-id>/chat \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"列出 /tmp 目录"}]}'
```

### Agent 返回定义

| 字段 | 行为 |
|------|------|
| `responseSchema` | 最后一轮使用 `response_format: json_schema` 强制 JSON |
| `responseDescription` | 写入 system prompt，描述期望输出格式 |
| 两者同时存在 | schema 优先；description 作为补充说明 |

## 环境变量

| 变量 | 说明 |
|------|------|
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` / `LLM_VISION_MODEL` | LLM |
| `LLM_STORE_DSN` | 存储（`postgres://` / `sqlite://` / `file://`） |
| `GATEWAY_API_KEY` | 网关 API Key（可选） |
| `WHISPER_BASE_URL` / `WHISPER_API_KEY` / `WHISPER_MODEL` | STT |
| `LLM_SERVER_ADDR` | 监听地址 |
| `MCP_IDLE_TTL` / `AGENT_MAX_ROUNDS` | Agent 行为 |
| `MCP_FS_ROOT` | seed 脚本 filesystem 根目录 |

## 数据库迁移

```bash
make migrate
```

`001_init.sql` → `002_agent_refactor.sql`（Agent 表、移除 tenant）。

## 项目结构

```
mcphub/
├── cmd/mcphub/
├── cmd/migrate/
├── internal/
│   ├── agent/       # Agent 循环
│   ├── stt/         # Whisper STT
│   ├── mcp/ llm/ http/ storage/ models/
├── migrations/
│   ├── postgres/
│   └── sqlite/
├── web/static/
└── deploy/
```

## 注意事项

- Ollama tool calling 推荐 `llama3.1+` 或 `qwen2.5`。
- 支持 `stdio` 与 `http`（Streamable HTTP）MCP 传输。
- HTTP MCP 使用 [github.com/SolaTyolo/httpclient](https://github.com/SolaTyolo/httpclient)，内置重试。
- MCP 配置变更会失效缓存会话；空闲 `MCP_IDLE_TTL` 后自动断开。
