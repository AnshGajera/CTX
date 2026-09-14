package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/AnshGajera/CTX/internal/versioning"
)

// SyncClient talks to the ctx cloud API.
type SyncClient struct {
	apiURL string
	token  string
	http   *http.Client
}

// NewSyncClient creates a client.
func NewSyncClient(apiURL, token string) *SyncClient {
	return &SyncClient{
		apiURL: apiURL, token: token,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SyncClient) auth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")
}

// Push uploads a snapshot.
func (c *SyncClient) Push(projectID string, snapshot *versioning.ContextSnapshot) error {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	url := fmt.Sprintf("%s/api/v1/projects/%s/push", c.apiURL, projectID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("push request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push failed (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// Pull downloads the latest snapshot.
func (c *SyncClient) Pull(projectID string) (*versioning.ContextSnapshot, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%s/pull", c.apiURL, projectID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pull request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pull failed (%d): %s", resp.StatusCode, string(b))
	}
	var snap versioning.ContextSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return &snap, nil
}

// Login exchanges email/password for a token.
func (c *SyncClient) Login(email, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	url := c.apiURL + "/auth/login"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(b))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode login: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("empty token in login response")
	}
	return out.Token, nil
}

// CreateToken creates a share token.
func (c *SyncClient) CreateToken(name string, readOnly bool) (string, error) {
	body, _ := json.Marshal(map[string]any{"name": name, "read_only": readOnly})
	url := c.apiURL + "/api/v1/tokens"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create token failed (%d): %s", resp.StatusCode, string(b))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	return out.Token, nil
}

// CredentialsPath returns ~/.config/ctx/credentials.json.
func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ctx", "credentials.json"), nil
}

// SaveToken persists token.
func SaveToken(token string) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"token": token})
	return os.WriteFile(path, data, 0o600)
}

// LoadToken loads token.
func LoadToken() string {
	path, err := CredentialsPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m["token"]
}
