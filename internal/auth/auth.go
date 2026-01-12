package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type AuthTokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
}

type AuthFile struct {
	Tokens      AuthTokens `json:"tokens"`
	LastRefresh string     `json:"last_refresh"`
}

type AuthStore struct {
	path string
	mu   sync.Mutex
}

func NewAuthStore(path string) *AuthStore {
	return &AuthStore{path: path}
}

func (s *AuthStore) Load() (AuthFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var auth AuthFile
	data, err := os.ReadFile(s.path)
	if err != nil {
		return auth, err
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return auth, err
	}
	return auth, nil
}

func (s *AuthStore) Save(auth AuthFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func (s *AuthStore) GetAccessToken() (AuthTokens, error) {
	auth, err := s.Load()
	if err != nil {
		return AuthTokens{}, err
	}
	tokens := auth.Tokens
	if tokens.AccessToken == "" {
		return AuthTokens{}, errors.New("missing access_token in auth.json")
	}
	if tokens.AccountID == "" && tokens.IDToken != "" {
		if acc, err := accountIDFromIDToken(tokens.IDToken); err == nil {
			tokens.AccountID = acc
		}
	}
	return tokens, nil
}

func (s *AuthStore) Refresh(ctx context.Context, client *http.Client, clientID string) (AuthTokens, error) {
	auth, err := s.Load()
	if err != nil {
		return AuthTokens{}, err
	}
	if auth.Tokens.RefreshToken == "" {
		return AuthTokens{}, errors.New("missing refresh_token in auth.json")
	}

	reqBody := map[string]string{
		"client_id":     clientID,
		"grant_type":    "refresh_token",
		"refresh_token": auth.Tokens.RefreshToken,
		"scope":         "openid profile email",
	}
	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/oauth/token", bytes.NewReader(payload))
	if err != nil {
		return AuthTokens{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return AuthTokens{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AuthTokens{}, fmt.Errorf("refresh failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var refreshResp struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &refreshResp); err != nil {
		return AuthTokens{}, err
	}

	updated := auth
	if refreshResp.IDToken != "" {
		updated.Tokens.IDToken = refreshResp.IDToken
	}
	if refreshResp.AccessToken != "" {
		updated.Tokens.AccessToken = refreshResp.AccessToken
	}
	if refreshResp.RefreshToken != "" {
		updated.Tokens.RefreshToken = refreshResp.RefreshToken
	}
	if updated.Tokens.AccountID == "" && updated.Tokens.IDToken != "" {
		if acc, err := accountIDFromIDToken(updated.Tokens.IDToken); err == nil {
			updated.Tokens.AccountID = acc
		}
	}
	updated.LastRefresh = time.Now().UTC().Format(time.RFC3339)

	if err := s.Save(updated); err != nil {
		return AuthTokens{}, err
	}
	return updated.Tokens, nil
}

func accountIDFromIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid id_token format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", err
	}
	authClaim, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return "", errors.New("missing auth claim")
	}
	if val, ok := authClaim["chatgpt_account_id"].(string); ok {
		return val, nil
	}
	return "", errors.New("missing chatgpt_account_id")
}
