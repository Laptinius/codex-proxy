package utils

import (
	"encoding/json"
	"strings"

	"codex-openai-wrapper/internal/types"
)

type ChatMessage struct {
	Role    string
	Content any
}

func NormalizeModelName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "gpt-5"
	}
	base := strings.SplitN(name, ":", 2)[0]
	switch base {
	case "gpt5", "gpt-5-latest":
		return "gpt-5"
	case "codex", "codex-mini", "codex-mini-latest":
		return "codex-mini-latest"
	default:
		return base
	}
}

func ExtractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ptype, _ := obj["type"].(string)
			if ptype != "text" {
				continue
			}
			if txt, ok := obj["text"].(string); ok && txt != "" {
				parts = append(parts, txt)
			} else if txt, ok := obj["content"].(string); ok && txt != "" {
				parts = append(parts, txt)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func CoerceMessages(payload map[string]any) []ChatMessage {
	raw, ok := payload["messages"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]ChatMessage, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := obj["role"].(string)
		content := obj["content"]
		out = append(out, ChatMessage{Role: role, Content: content})
	}
	return out
}

func NormalizeMessages(messages []ChatMessage) []ChatMessage {
	if len(messages) == 0 {
		return messages
	}
	sysIdx := -1
	for i, msg := range messages {
		if msg.Role == "system" {
			sysIdx = i
			break
		}
	}
	if sysIdx == -1 {
		return messages
	}
	sys := messages[sysIdx]
	var rest []ChatMessage
	rest = append(rest, messages[:sysIdx]...)
	rest = append(rest, messages[sysIdx+1:]...)
	return append([]ChatMessage{{Role: "user", Content: sys.Content}}, rest...)
}

func BuildInputItems(messages []ChatMessage) []types.InputItem {
	out := make([]types.InputItem, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role != "assistant" {
			role = "user"
		}
		text := ExtractText(msg.Content)
		if strings.TrimSpace(text) == "" {
			continue
		}
		contentType := "input_text"
		if role == "assistant" {
			contentType = "output_text"
		}
		out = append(out, types.InputItem{
			Type: "message",
			Role: role,
			Content: []types.ContentItem{
				{Type: contentType, Text: text},
			},
		})
	}
	return out
}

func ParseJSONBody(data []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}
