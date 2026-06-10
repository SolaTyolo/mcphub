#!/usr/bin/env sh
# Create demo MCP server + text/vision agents via HTTP API.
# Requires mcphub running (make dev or make up).
set -e

GATEWAY_URL="${MCPHUB_URL:-http://localhost:8090}"
MCP_FS_ROOT="${MCP_FS_ROOT:-/tmp}"
GATEWAY_API_KEY="${GATEWAY_API_KEY:-}"
TEXT_MODEL="${LLM_MODEL:-qwen2.5:7b-instruct-q4_K_M}"
VISION_MODEL="${LLM_VISION_MODEL:-llama3.2-vision}"

curl_auth() {
  if [ -n "$GATEWAY_API_KEY" ]; then
    curl -sf -H "X-API-Key: $GATEWAY_API_KEY" "$@"
  else
    curl -sf "$@"
  fi
}

json_field() {
  python3 -c "import sys,json; print(json.load(sys.stdin)['$1'])"
}

echo "mcphub: $GATEWAY_URL"
echo "Text model: $TEXT_MODEL"
echo "Vision model: $VISION_MODEL"
echo "MCP filesystem root: $MCP_FS_ROOT"

server_resp=$(curl_auth -X POST "$GATEWAY_URL/api/mcp-servers" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"filesystem\",\"transport\":\"stdio\",\"command\":\"npx\",\"args\":[\"-y\",\"@modelcontextprotocol/server-filesystem\",\"$MCP_FS_ROOT\"],\"enabled\":true}")

server_id=$(printf '%s' "$server_resp" | json_field id)

text_agent=$(curl_auth -X POST "$GATEWAY_URL/api/agents" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"text-agent\",\"systemPrompt\":\"You are a helpful assistant with filesystem tools.\",\"llmModel\":\"$TEXT_MODEL\",\"mcpServerIds\":[\"$server_id\"],\"enabled\":true}")

text_id=$(printf '%s' "$text_agent" | json_field id)

vision_agent=$(curl_auth -X POST "$GATEWAY_URL/api/agents" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"vision-agent\",\"systemPrompt\":\"Describe images clearly in the user's language.\",\"llmModel\":\"$TEXT_MODEL\",\"visionModel\":\"$VISION_MODEL\",\"enabled\":true}")

vision_id=$(printf '%s' "$vision_agent" | json_field id)

echo ""
echo "Demo ready:"
echo "  text_agent_id=$text_id   (text + MCP tools, model=$TEXT_MODEL)"
echo "  vision_agent_id=$vision_id (image Q&A, vision model=$VISION_MODEL)"
echo "  mcp_server_id=$server_id"
echo ""
echo "Open $GATEWAY_URL — use Whisper audio, image upload, or text chat."
