package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"codex-openai-wrapper/internal/auth"
	"codex-openai-wrapper/internal/config"
	"codex-openai-wrapper/internal/types"
)

type UpstreamClient struct {
	cfg    config.Config
	auth   *auth.AuthStore
	client *http.Client
}

func NewUpstreamClient(cfg config.Config, auth *auth.AuthStore, client *http.Client) *UpstreamClient {
	return &UpstreamClient{cfg: cfg, auth: auth, client: client}
}

func (u *UpstreamClient) DoResponsesStream(ctx context.Context, reqBody types.ResponsesRequest) (*http.Response, int, error) {
	bodyBytes, _ := json.Marshal(reqBody)

	tokens, err := u.auth.GetAccessToken()
	if err != nil {
		return nil, 0, err
	}

	resp, status, err := u.doStreamOnce(ctx, bodyBytes, tokens)
	if err == nil && status != http.StatusUnauthorized {
		return resp, status, nil
	}
	if status != http.StatusUnauthorized {
		return resp, status, err
	}

	refreshed, refreshErr := u.auth.Refresh(ctx, u.client, u.cfg.ClientID)
	if refreshErr != nil {
		if err != nil {
			return resp, status, err
		}
		return resp, status, refreshErr
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return u.doStreamOnce(ctx, bodyBytes, refreshed)
}

func (u *UpstreamClient) DoRawResponsesStream(ctx context.Context, bodyBytes []byte) (*http.Response, int, error) {
	tokens, err := u.auth.GetAccessToken()
	if err != nil {
		return nil, 0, err
	}

	resp, status, err := u.doStreamOnce(ctx, bodyBytes, tokens)
	if err == nil && status != http.StatusUnauthorized {
		return resp, status, nil
	}
	if status != http.StatusUnauthorized {
		return resp, status, err
	}

	refreshed, refreshErr := u.auth.Refresh(ctx, u.client, u.cfg.ClientID)
	if refreshErr != nil {
		if err != nil {
			return resp, status, err
		}
		return resp, status, refreshErr
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return u.doStreamOnce(ctx, bodyBytes, refreshed)
}

func (u *UpstreamClient) doStreamOnce(ctx context.Context, body []byte, tokens auth.AuthTokens) (*http.Response, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.ResponsesURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	if tokens.AccountID != "" {
		req.Header.Set("chatgpt-account-id", tokens.AccountID)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	return resp, resp.StatusCode, nil
}
