package homely

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type tokenSource struct {
	baseURL  string
	username string
	password string
}

type tokenPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken      string    `json:"access_token"`
	ExpiresIn        int       `json:"expires_in"`
	RefreshExpiresIn int       `json:"refresh_expires_in"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type:"`
	NotBeforePolicy  int       `json:"not-before-policy"`
	SessionState     uuid.UUID `json:"session_state"`
	Scope            string    `json:"scope"`
}

func (s *tokenSource) Token() (*oauth2.Token, error) {
	payload := &bytes.Buffer{}

	err := json.NewEncoder(payload).Encode(tokenPayload{
		Username: s.username,
		Password: s.password,
	})
	if err != nil {
		return nil, fmt.Errorf("encode token request body: %w", err)
	}

	path, err := url.JoinPath(s.baseURL, "homely/oauth/token")
	if err != nil {
		return nil, fmt.Errorf("create token URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, path, payload)
	if err != nil {
		return nil, fmt.Errorf("created token request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("get token: %s", res.Status)
	}

	var r tokenResponse

	if err := json.Unmarshal(bodyBytes, &r); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	t := &oauth2.Token{
		AccessToken:  r.AccessToken,
		TokenType:    r.TokenType,
		RefreshToken: r.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(r.ExpiresIn)),
	}

	return t.WithExtra(map[string]any{
		"refresh_expires_in": r.RefreshExpiresIn,
		"not_before_policy":  r.NotBeforePolicy,
		"session_state":      r.SessionState,
		"scope":              r.Scope,
	}), nil
}
