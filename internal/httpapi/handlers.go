package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"codex-openai-wrapper/internal/config"
	"codex-openai-wrapper/internal/instructions"
	"codex-openai-wrapper/internal/types"
	"codex-openai-wrapper/internal/upstream"
	"codex-openai-wrapper/internal/utils"
)

type App struct {
	cfg          config.Config
	upstream     *upstream.UpstreamClient
	instructions *instructions.InstructionsCache
}

func NewApp(cfg config.Config, upstreamClient *upstream.UpstreamClient, instructionsCache *instructions.InstructionsCache) *App {
	return &App{
		cfg:          cfg,
		upstream:     upstreamClient,
		instructions: instructionsCache,
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", a.handleChatCompletions)
	mux.HandleFunc("/v1/completions", a.handleCompletions)
	mux.HandleFunc("/v1/models", a.handleModels)
	mux.HandleFunc("/", a.handleNotFound)

	return withCORS(withLogging(a.cfg, mux))
}

func (a *App) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "Not found")
}

func (a *App) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if !a.authorize(r) {
		writeError(w, http.StatusUnauthorized, "Invalid API key")
		return
	}
	start := time.Now()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	payload, err := utils.ParseJSONBody(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	model := utils.NormalizeModelName(stringOr(payload["model"], ""))
	messages := utils.NormalizeMessages(utils.CoerceMessages(payload))
	if len(messages) == 0 {
		if prompt, ok := payload["prompt"].(string); ok && prompt != "" {
			messages = []utils.ChatMessage{{Role: "user", Content: prompt}}
		}
		if input, ok := payload["input"].(string); ok && input != "" {
			messages = []utils.ChatMessage{{Role: "user", Content: input}}
		}
	}
	if len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "Request must include messages")
		return
	}
	if !hasNonEmptyMessages(messages) {
		writeError(w, http.StatusBadRequest, "messages must contain non-empty content")
		return
	}

	streamRequested := false
	if v, ok := payload["stream"].(bool); ok && v {
		streamRequested = true
	}

	instructionsValue, err := a.resolveInstructions(r.Context(), payload, model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to fetch instructions")
		return
	}

	inputItems := utils.BuildInputItems(messages)
	reqBody := types.ResponsesRequest{
		Model:        model,
		Instructions: instructionsValue,
		Input:        inputItems,
		Stream:       true,
		Store:        false,
	}

	upstreamResp, status, err := a.upstream.DoResponsesStream(r.Context(), reqBody)
	if err != nil {
		log.Printf("upstream error: %v", err)
		writeError(w, http.StatusBadGateway, "Upstream request failed")
		return
	}
	if status < 200 || status >= 300 {
		raw, _ := io.ReadAll(upstreamResp.Body)
		_ = upstreamResp.Body.Close()
		log.Printf("upstream status=%d body=%s", status, string(redactUpstreamError(raw)))
		writeError(w, status, upstreamErrorMessage(raw))
		return
	}

	var logFn func(string)
	if a.cfg.LogUpstream {
		logFn = func(data string) {
			log.Printf("upstream sse data=%s", data)
		}
	}

	if streamRequested {
		usage, err := streamChatCompletions(w, upstreamResp.Body, model, logFn)
		_ = upstreamResp.Body.Close()
		logResponseTokens(a.cfg, model, time.Since(start), http.StatusOK, usage)
		if err != nil {
			log.Printf("stream error: %v", err)
		}
		return
	}

	text, responseID, usage, err := collectFromSSE(upstreamResp.Body, logFn)
	_ = upstreamResp.Body.Close()
	if err != nil {
		log.Printf("stream collect error: %v", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	created := time.Now().Unix()
	out := types.ChatCompletionResponse{
		ID:      nonEmpty(responseID, "chatcmpl"),
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Message: types.ChatMessageOut{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: "stop",
			},
		},
		Usage: usage,
	}
	logResponseTokens(a.cfg, model, time.Since(start), http.StatusOK, usage)
	writeJSON(w, out)
}

func (a *App) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if !a.authorize(r) {
		writeError(w, http.StatusUnauthorized, "Invalid API key")
		return
	}
	start := time.Now()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	payload, err := utils.ParseJSONBody(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	model := utils.NormalizeModelName(stringOr(payload["model"], ""))

	var prompt string
	switch v := payload["prompt"].(type) {
	case string:
		prompt = v
	case []any:
		var parts []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		prompt = strings.Join(parts, "")
	}
	if prompt == "" {
		if input, ok := payload["input"].(string); ok {
			prompt = input
		}
	}

	if prompt == "" {
		writeError(w, http.StatusBadRequest, "Request must include prompt")
		return
	}

	streamRequested := false
	if v, ok := payload["stream"].(bool); ok && v {
		streamRequested = true
	}

	instructionsValue, err := a.resolveInstructions(r.Context(), payload, model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to fetch instructions")
		return
	}

	messages := []utils.ChatMessage{{Role: "user", Content: prompt}}
	inputItems := utils.BuildInputItems(messages)
	reqBody := types.ResponsesRequest{
		Model:        model,
		Instructions: instructionsValue,
		Input:        inputItems,
		Stream:       true,
		Store:        false,
	}

	upstreamResp, status, err := a.upstream.DoResponsesStream(r.Context(), reqBody)
	if err != nil {
		log.Printf("upstream error: %v", err)
		writeError(w, http.StatusBadGateway, "Upstream request failed")
		return
	}
	if status < 200 || status >= 300 {
		raw, _ := io.ReadAll(upstreamResp.Body)
		_ = upstreamResp.Body.Close()
		log.Printf("upstream status=%d body=%s", status, string(redactUpstreamError(raw)))
		writeError(w, status, upstreamErrorMessage(raw))
		return
	}

	var logFn func(string)
	if a.cfg.LogUpstream {
		logFn = func(data string) {
			log.Printf("upstream sse data=%s", data)
		}
	}

	if streamRequested {
		usage, err := streamChatCompletions(w, upstreamResp.Body, model, logFn)
		_ = upstreamResp.Body.Close()
		logResponseTokens(a.cfg, model, time.Since(start), http.StatusOK, usage)
		if err != nil {
			log.Printf("stream error: %v", err)
		}
		return
	}

	text, responseID, usage, err := collectFromSSE(upstreamResp.Body, logFn)
	_ = upstreamResp.Body.Close()
	if err != nil {
		log.Printf("stream collect error: %v", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	created := time.Now().Unix()
	out := types.TextCompletionResponse{
		ID:      nonEmpty(responseID, "cmpl"),
		Object:  "text_completion",
		Created: created,
		Model:   model,
		Choices: []types.TextChoice{
			{
				Index:        0,
				Text:         text,
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
		Usage: usage,
	}
	logResponseTokens(a.cfg, model, time.Since(start), http.StatusOK, usage)
	writeJSON(w, out)
}

func (a *App) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	out := modelsResponse()
	writeJSON(w, out)
}

func (a *App) authorize(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token != "" && token == a.cfg.APIKey
}

func (a *App) resolveInstructions(ctx context.Context, payload map[string]any, model string) (string, error) {
	if inst, ok := payload["instructions"].(string); ok && strings.TrimSpace(inst) != "" {
		return inst, nil
	}
	return a.instructions.Get(ctx, model)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(types.ErrorResponse{Error: types.ErrorBody{Message: normalizeErrorMessage(message)}})
}

func nonEmpty(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

func stringOr(v any, fallback string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

func logResponseTokens(cfg config.Config, model string, duration time.Duration, status int, usage *types.Usage) {
	if !cfg.LogTokens {
		return
	}
	if usage == nil {
		log.Printf("response status=%d model=%s duration=%s tokens=n/a", status, model, duration)
		return
	}
	inputTokens := usageInt(*usage, "input_tokens")
	outputTokens := usageInt(*usage, "output_tokens")
	totalTokens := usageInt(*usage, "total_tokens")
	cachedTokens := usageNestedInt(*usage, "input_tokens_details", "cached_tokens")
	log.Printf(
		"response status=%d model=%s duration=%s input=%d cached=%d output=%d total=%d",
		status,
		model,
		duration,
		inputTokens,
		cachedTokens,
		outputTokens,
		totalTokens,
	)
}

func usageInt(usage types.Usage, key string) int {
	raw, ok := usage[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func usageNestedInt(usage types.Usage, key, subkey string) int {
	raw, ok := usage[key]
	if !ok {
		return 0
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return 0
	}
	sub, ok := obj[subkey]
	if !ok {
		return 0
	}
	switch v := sub.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func hasNonEmptyMessages(messages []utils.ChatMessage) bool {
	for _, msg := range messages {
		if strings.TrimSpace(utils.ExtractText(msg.Content)) != "" {
			return true
		}
	}
	return false
}

func redactUpstreamError(raw []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	if detail, ok := payload["detail"].(map[string]any); ok {
		if _, has := detail["id"]; has {
			detail["id"] = "present"
		}
		if _, has := detail["instructions"]; has {
			detail["instructions"] = "present"
		}
		if _, has := detail["prompt_cache_key"]; has {
			detail["prompt_cache_key"] = "present"
		}
		if _, has := detail["safety_identifier"]; has {
			detail["safety_identifier"] = "present"
		}
		payload["detail"] = detail
	}
	if resp, ok := payload["response"].(map[string]any); ok {
		if _, has := resp["id"]; has {
			resp["id"] = "present"
		}
		if _, has := resp["instructions"]; has {
			resp["instructions"] = "present"
		}
		if _, has := resp["prompt_cache_key"]; has {
			resp["prompt_cache_key"] = "present"
		}
		if _, has := resp["safety_identifier"]; has {
			resp["safety_identifier"] = "present"
		}
		payload["response"] = resp
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return out
}

func upstreamErrorMessage(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "Upstream error"
	}
	if errObj, ok := payload["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if detail, ok := payload["detail"].(string); ok && detail != "" {
		return detail
	}
	if detailObj, ok := payload["detail"].(map[string]any); ok {
		if msg, ok := detailObj["message"].(string); ok && msg != "" {
			return msg
		}
		if errObj, ok := detailObj["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				return msg
			}
		}
	}
	return "Upstream error"
}

func normalizeErrorMessage(message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "Upstream error"
	}
	return msg
}
