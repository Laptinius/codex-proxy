package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	APIKey       string
	ResponsesURL string
	ClientID     string
	AuthFile     string
	InstrTTL     time.Duration
	LogUpstream  bool
	LogTokens    bool
}

func LoadConfig() (Config, error) {
	addr := envOr("ADDR", "0.0.0.0:8080")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return Config{}, fmt.Errorf("API_KEY is required")
	}
	responsesURL := envOr("CHATGPT_RESPONSES_URL", "https://chatgpt.com/backend-api/codex/responses")
	clientID := envOr("CHATGPT_LOCAL_CLIENT_ID", "app_EMoamEEZ73f0CkXaXp7hrann")
	authFile := os.Getenv("AUTH_FILE")
	if authFile == "" {
		authFile = filepath.Join("configs", "auth.json")
	}

	ttlHours := envOr("INSTR_TTL_HOURS", "12")
	ttlInt, err := strconv.Atoi(ttlHours)
	if err != nil || ttlInt <= 0 {
		ttlInt = 12
	}

	logUpstream := envOr("LOG_UPSTREAM", "false") == "true"
	logTokens := envOr("LOG_TOKENS", "true") == "true"

	return Config{
		Addr:         addr,
		APIKey:       apiKey,
		ResponsesURL: responsesURL,
		ClientID:     clientID,
		AuthFile:     authFile,
		InstrTTL:     time.Duration(ttlInt) * time.Hour,
		LogUpstream:  logUpstream,
		LogTokens:    logTokens,
	}, nil
}

func envOr(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
