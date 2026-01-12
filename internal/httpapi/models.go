package httpapi

type ModelsResponse struct {
	Object string        `json:"object"`
	Data   []ModelRecord `json:"data"`
}

type ModelRecord struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func modelsResponse() ModelsResponse {
	return ModelsResponse{
		Object: "list",
		Data: []ModelRecord{
			{ID: "gpt-5.1-codex-max", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5.1-codex", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5.1-codex-mini", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5.2", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5.1", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5-codex", Object: "model", OwnedBy: "owner"},
			{ID: "gpt-5-codex-mini", Object: "model", OwnedBy: "owner"},
		},
	}
}
