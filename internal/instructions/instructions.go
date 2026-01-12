package instructions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	baseInstructionsURL = "https://raw.githubusercontent.com/openai/codex/refs/heads/main/codex-rs/core/prompt.md"
	gpt5InstructionsURL = "https://raw.githubusercontent.com/openai/codex/refs/heads/main/codex-rs/core/gpt_5_codex_prompt.md"
)

type cacheEntry struct {
	value   string
	expires time.Time
}

type InstructionsCache struct {
	ttl    time.Duration
	client *http.Client
	mu     sync.Mutex
	base   cacheEntry
	gpt5   cacheEntry
}

func NewInstructionsCache(ttl time.Duration, client *http.Client) *InstructionsCache {
	return &InstructionsCache{
		ttl:    ttl,
		client: client,
	}
}

func (c *InstructionsCache) Get(ctx context.Context, model string) (string, error) {
	useGpt5 := shouldUseGpt5CodexInstructions(model)
	c.mu.Lock()
	entry := c.base
	if useGpt5 {
		entry = c.gpt5
	}
	if entry.value != "" && time.Now().Before(entry.expires) {
		c.mu.Unlock()
		return entry.value, nil
	}
	c.mu.Unlock()

	url := baseInstructionsURL
	if useGpt5 {
		url = gpt5InstructionsURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("failed to fetch instructions")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text := string(body)

	c.mu.Lock()
	if useGpt5 {
		c.gpt5 = cacheEntry{value: text, expires: time.Now().Add(c.ttl)}
	} else {
		c.base = cacheEntry{value: text, expires: time.Now().Add(c.ttl)}
	}
	c.mu.Unlock()

	return text, nil
}

func shouldUseGpt5CodexInstructions(model string) bool {
	model = strings.TrimSpace(model)
	return strings.HasPrefix(model, "gpt-5-codex") || strings.HasPrefix(model, "codex-")
}
