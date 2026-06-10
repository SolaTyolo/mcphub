package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerAddr         string
	Store              StoreConfig
	AttachmentStore    AttachmentStoreConfig
	AttachmentMaxBytes int64
	GatewayAPIKey      string
	LLMAPIKey         string
	LLMBaseURL        string
	LLMModel          string
	LLMVisionModel    string
	WhisperBaseURL    string
	WhisperAPIKey     string
	WhisperModel      string
	MarkItDownMCPURL  string
	MCPIdleTTL        time.Duration
	AgentMaxRounds    int
}

func Load() Config {
	storeDSN := firstNonEmpty(os.Getenv("LLM_STORE_DSN"), os.Getenv("LLM_DB_DSN"))
	if storeDSN == "" {
		if dataFile := strings.TrimSpace(os.Getenv("LLM_DATA_FILE")); dataFile != "" {
			if strings.Contains(dataFile, "://") {
				storeDSN = dataFile
			} else {
				storeDSN = "file://" + dataFile
			}
		}
	}
	store := ParseStoreDSN(storeDSN)
	if legacy := os.Getenv("LLM_STORE"); legacy != "" {
		store = applyLegacyStoreKind(store, legacy)
	}
	attachmentDSN := firstNonEmpty(os.Getenv("ATTACHMENT_STORE_DSN"), os.Getenv("ATTACHMENT_DSN"))
	return Config{
		ServerAddr:         getenv("LLM_SERVER_ADDR", ":8090"),
		Store:              store,
		AttachmentStore:    ParseAttachmentDSN(attachmentDSN),
		AttachmentMaxBytes: int64Env("ATTACHMENT_MAX_BYTES", 32<<20),
		GatewayAPIKey:  os.Getenv("GATEWAY_API_KEY"),
		LLMAPIKey:      getenv("LLM_API_KEY", "ollama"),
		LLMBaseURL:     getenv("LLM_BASE_URL", "http://localhost:11434/v1"),
		LLMModel:       getenv("LLM_MODEL", "qwen2.5:7b-instruct-q4_K_M"),
		LLMVisionModel: getenv("LLM_VISION_MODEL", "llama3.2-vision"),
		WhisperBaseURL:    strings.TrimSpace(os.Getenv("WHISPER_BASE_URL")),
		WhisperAPIKey:     os.Getenv("WHISPER_API_KEY"),
		WhisperModel:      getenv("WHISPER_MODEL", "whisper-1"),
		MarkItDownMCPURL:  strings.TrimSpace(os.Getenv("MARKITDOWN_MCP_URL")),
		MCPIdleTTL:        durationEnv("MCP_IDLE_TTL", 5*time.Minute),
		AgentMaxRounds:    intEnv("AGENT_MAX_ROUNDS", 10),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func intEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func int64Env(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func (c Config) WhisperEnabled() bool {
	return c.WhisperBaseURL != ""
}

func (c Config) MarkItDownEnabled() bool {
	return c.MarkItDownMCPURL != ""
}
