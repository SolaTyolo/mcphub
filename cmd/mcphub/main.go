package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/SolaTyolo/mcphub/internal/agent"
	"github.com/SolaTyolo/mcphub/internal/attachment"
	"github.com/SolaTyolo/mcphub/internal/builtin"
	"github.com/SolaTyolo/mcphub/internal/config"
	httpapi "github.com/SolaTyolo/mcphub/internal/http"
	"github.com/SolaTyolo/mcphub/internal/llm"
	"github.com/SolaTyolo/mcphub/internal/mcp"
	"github.com/SolaTyolo/mcphub/internal/logx"
	"github.com/SolaTyolo/mcphub/internal/markitdown"
	"github.com/SolaTyolo/mcphub/internal/storage"
	"github.com/SolaTyolo/mcphub/internal/stt"
	"github.com/SolaTyolo/mcphub/web"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	ctx := context.Background()
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	if c, ok := store.(interface{ Close() error }); ok {
		defer c.Close()
	} else if c, ok := store.(interface{ Close() }); ok {
		defer c.Close()
	}
	logx.Info("mcphub", "store connected type=%s dsn=%s", cfg.Store.Kind, cfg.Store.RawDSN)

	attachments, err := attachment.Open(ctx, cfg.AttachmentStore)
	if err != nil {
		log.Fatalf("attachment store: %v", err)
	}
	logx.Info("mcphub", "attachment store connected type=%s dsn=%s", cfg.AttachmentStore.Kind, cfg.AttachmentStore.RawDSN)

	pool := mcp.NewPool(cfg.MCPIdleTTL)
	router := llm.NewRouter(cfg)
	markdownClient := markitdown.NewClient(cfg)
	builtinRunner := builtin.NewRunner(attachments, markdownClient)
	agentSvc := agent.NewService(cfg, router, pool, builtinRunner)
	sttClient := stt.NewClient(cfg)

	staticRoot, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	srv := httpapi.NewServer(cfg, store, attachments, agentSvc, pool, sttClient, http.FileServer(http.FS(staticRoot)))

	server := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logx.Info("mcphub", "listening addr=%s llm=%s vision=%s whisper=%t markitdown=%t auth=%t",
			cfg.ServerAddr, cfg.LLMModel, cfg.LLMVisionModel,
			cfg.WhisperEnabled(),
			cfg.MarkItDownEnabled(),
			cfg.GatewayAPIKey != "")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	logx.Info("mcphub", "shutdown signal received: %v", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logx.Error("mcphub", "shutdown: %v", err)
	} else {
		logx.Info("mcphub", "stopped")
	}
}
