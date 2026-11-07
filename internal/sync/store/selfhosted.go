package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SelfHostedOptions configures the Vela sync-backend client.
type SelfHostedOptions struct {
	ServerURL string
	SyncID    string
	UserKey   string
}

type selfHostedStore struct {
	syncID     string
	userKey    string
	httpClient *http.Client
	baseURL    string
}

// NewSelfHosted creates a Store for the Vela sync-backend HTTP API.
func NewSelfHosted(opts SelfHostedOptions) (Store, error) {
	if strings.TrimSpace(opts.ServerURL) == "" {
		return nil, fmt.Errorf("server url required")
	}
	if strings.TrimSpace(opts.SyncID) == "" {
		return nil, fmt.Errorf("sync id required")
	}
	if strings.TrimSpace(opts.UserKey) == "" {
		return nil, fmt.Errorf("user key required")
	}
	return &selfHostedStore{
		syncID:     opts.SyncID,
		userKey:    opts.UserKey,
		httpClient: &http.Client{Timeout: storeHTTPTimeout},
		baseURL:    normalizeURL(opts.ServerURL),
	}, nil
}

func normalizeURL(u string) string {
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return "http://" + u
}

func (s *selfHostedStore) setAuthHeaders(req *http.Request) {
	req.Header.Set("X-Sync-ID", s.syncID)
	req.Header.Set("X-User-Key", s.userKey)
}

func (s *selfHostedStore) Upload(ctx context.Context, name string, r io.Reader, size int64) (*BackupInfo, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	u := s.baseURL + "/upload?name=" + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/octet-stream")
	if size >= 0 {
		req.ContentLength = size
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	var result BackupInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *selfHostedStore) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	name, err := SanitizeName(name)
	if err != nil {
		return nil, err
	}
	u := s.baseURL + "/download?name=" + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.setAuthHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("download failed: %s - %s", resp.Status, string(body))
	}
	return resp.Body, nil
}

func (s *selfHostedStore) ListBackups(ctx context.Context, limit, offset int) ([]BackupInfo, error) {
	u := s.baseURL + "/backups"
	if limit > 0 {
		u += fmt.Sprintf("?limit=%d&offset=%d", limit, offset)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.setAuthHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list backups failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Backups []BackupInfo `json:"backups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Backups, nil
}

func (s *selfHostedStore) DeleteBackup(ctx context.Context, name string) error {
	name, err := SanitizeName(name)
	if err != nil {
		return err
	}
	u := s.baseURL + "/backups?name=" + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	s.setAuthHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete backup failed: %s - %s", resp.Status, string(body))
	}
	return nil
}

func (s *selfHostedStore) HealthCheck(ctx context.Context) error {
	_, err := s.ListBackups(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}
