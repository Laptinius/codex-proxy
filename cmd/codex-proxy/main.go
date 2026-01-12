package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"codex-openai-wrapper/internal/auth"
	"codex-openai-wrapper/internal/config"
	"codex-openai-wrapper/internal/httpapi"
	"codex-openai-wrapper/internal/instructions"
	"codex-openai-wrapper/internal/upstream"
)

func main() {
	if err := config.EnsureConfigs("configs"); err != nil {
		log.Fatalf("config files error: %v", err)
	}
	config.LoadDotenv("configs/.env")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	authStore := auth.NewAuthStore(cfg.AuthFile)
	instructionsCache := instructions.NewInstructionsCache(cfg.InstrTTL, client)
	upstreamClient := upstream.NewUpstreamClient(cfg, authStore, client)

	app := httpapi.NewApp(cfg, upstreamClient, instructionsCache)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
