package httpapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"codex-openai-wrapper/internal/types"
)

type sseEvent struct {
	Type     string `json:"type"`
	Delta    string `json:"delta"`
	Response struct {
		ID    string       `json:"id"`
		Usage *types.Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

func redactSSEData(data string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return data
	}
	resp, ok := payload["response"].(map[string]any)
	if ok {
		if _, has := resp["instructions"]; has {
			resp["instructions"] = "є"
		}
		if _, has := resp["prompt_cache_key"]; has {
			resp["prompt_cache_key"] = "є"
		}
		payload["response"] = resp
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return data
	}
	return string(out)
}

func collectFromSSE(body io.Reader, logFn func(string)) (string, string, *types.Usage, error) {
	reader := bufio.NewReader(body)
	var textBuilder strings.Builder
	var responseID string
	var usage *types.Usage

	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", "", nil, err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if logFn != nil {
				logFn(redactSSEData(data))
			}
			if data == "[DONE]" {
				break
			}
			var evt sseEvent
			if jsonErr := json.Unmarshal([]byte(data), &evt); jsonErr == nil {
				if evt.Response.ID != "" {
					responseID = evt.Response.ID
				}
				switch evt.Type {
				case "response.output_text.delta":
					if evt.Delta != "" {
						textBuilder.WriteString(evt.Delta)
					}
				case "response.completed":
					if evt.Response.Usage != nil {
						usage = evt.Response.Usage
					}
				case "response.failed":
					if evt.Response.Error != nil {
						return "", responseID, usage, errors.New(evt.Response.Error.Message)
					}
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}

	return textBuilder.String(), responseID, usage, nil
}

func streamChatCompletions(
	w http.ResponseWriter,
	upstreamBody io.Reader,
	model string,
	logFn func(string),
) (*types.Usage, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	reader := bufio.NewReader(upstreamBody)
	created := time.Now().Unix()
	responseID := "chatcmpl-stream"
	var usage *types.Usage
	sentRole := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return usage, err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if logFn != nil {
				logFn(redactSSEData(data))
			}
			if data == "[DONE]" {
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}
			var evt sseEvent
			if jsonErr := json.Unmarshal([]byte(data), &evt); jsonErr == nil {
				if evt.Response.ID != "" {
					responseID = evt.Response.ID
				}
				if evt.Type == "response.output_text.delta" && evt.Delta != "" {
					delta := types.ChatStreamDelta{Content: evt.Delta}
					if !sentRole {
						delta.Role = "assistant"
						sentRole = true
					}
					chunk := types.ChatCompletionStreamChunk{
						ID:      responseID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []types.ChatStreamChoice{
							{
								Index:        0,
								Delta:        delta,
								FinishReason: "",
							},
						},
					}
					payload, _ := json.Marshal(chunk)
					_, _ = io.WriteString(w, "data: ")
					_, _ = w.Write(payload)
					_, _ = io.WriteString(w, "\n\n")
					flusher.Flush()
				}
				if evt.Type == "response.completed" {
					if evt.Response.Usage != nil {
						usage = evt.Response.Usage
					}
					finish := types.ChatCompletionStreamChunk{
						ID:      responseID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []types.ChatStreamChoice{
							{
								Index:        0,
								Delta:        types.ChatStreamDelta{},
								FinishReason: "stop",
							},
						},
					}
					payload, _ := json.Marshal(finish)
					_, _ = io.WriteString(w, "data: ")
					_, _ = w.Write(payload)
					_, _ = io.WriteString(w, "\n\n")
					_, _ = io.WriteString(w, "data: [DONE]\n\n")
					flusher.Flush()
					break
				}
				if evt.Type == "response.failed" && evt.Response.Error != nil {
					errChunk := types.ChatCompletionStreamChunk{
						ID:      responseID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   model,
						Choices: []types.ChatStreamChoice{
							{
								Index:        0,
								Delta:        types.ChatStreamDelta{},
								FinishReason: "error",
							},
						},
					}
					errPayload, _ := json.Marshal(errChunk)
					_, _ = io.WriteString(w, "data: ")
					_, _ = w.Write(errPayload)
					_, _ = io.WriteString(w, "\n\n")
					_, _ = io.WriteString(w, "data: [DONE]\n\n")
					flusher.Flush()
					return usage, errors.New(evt.Response.Error.Message)
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return usage, nil
}
