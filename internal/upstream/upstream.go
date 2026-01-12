package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (u *UpstreamClient) DoResponsesRequest(ctx context.Context, reqBody types.ResponsesRequest) (types.ResponsesResponse, int, []byte, error) {
	var respPayload types.ResponsesResponse
	bodyBytes, _ := json.Marshal(reqBody)

	tokens, err := u.auth.GetAccessToken()
	if err != nil {
		return respPayload, 0, nil, err
	}

	resp, status, raw, err := u.doOnce(ctx, bodyBytes, tokens)
	if err == nil && status != http.StatusUnauthorized {
		return resp, status, raw, nil
	}
	if status != http.StatusUnauthorized {
		return resp, status, raw, err
	}

	refreshed, refreshErr := u.auth.Refresh(ctx, u.client, u.cfg.ClientID)
	if refreshErr != nil {
		if err != nil {
			return respPayload, status, raw, err
		}
		return respPayload, status, raw, refreshErr
	}

	return u.doOnce(ctx, bodyBytes, refreshed)
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

func (u *UpstreamClient) doOnce(ctx context.Context, body []byte, tokens auth.AuthTokens) (types.ResponsesResponse, int, []byte, error) {
	var out types.ResponsesResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.ResponsesURL, bytes.NewReader(body))
	if err != nil {
		return out, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	if tokens.AccountID != "" {
		req.Header.Set("chatgpt-account-id", tokens.AccountID)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return out, 0, nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, resp.StatusCode, raw, fmt.Errorf("invalid upstream JSON: %w", err)
	}
	return out, resp.StatusCode, raw, nil
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
