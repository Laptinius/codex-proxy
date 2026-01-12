package types

type InputItem struct {
	Type    string         `json:"type"`
	Role    string         `json:"role,omitempty"`
	Content []ContentItem  `json:"content,omitempty"`
	CallID  string         `json:"call_id,omitempty"`
	Output  string         `json:"output,omitempty"`
	Name    string         `json:"name,omitempty"`
	Args    string         `json:"arguments,omitempty"`
	Extra   map[string]any `json:"-"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ResponsesRequest struct {
	Model         string      `json:"model"`
	Instructions  string      `json:"instructions,omitempty"`
	Input         []InputItem `json:"input"`
	Stream        bool        `json:"stream"`
	Store         bool        `json:"store"`
	ParallelTools bool        `json:"parallel_tool_calls,omitempty"`
}

type Usage map[string]any

type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int            `json:"index"`
	Message      ChatMessageOut `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

type ChatMessageOut struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionStreamChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []ChatStreamChoice `json:"choices"`
}

type ChatStreamChoice struct {
	Index        int             `json:"index"`
	Delta        ChatStreamDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type ChatStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type TextCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []TextChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

type TextChoice struct {
	Index        int         `json:"index"`
	Text         string      `json:"text"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     interface{} `json:"logprobs"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string `json:"message"`
}
