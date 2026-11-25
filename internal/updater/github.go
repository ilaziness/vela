package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/ilaziness/vela/internal/httpproxy"
)

const githubRepo = "ilaziness/vela"

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GitHubSource checks GitHub Releases for ilaziness/vela.
type GitHubSource struct {
	repo    string
	apiBase string
	client  *http.Client
	token   string
}

// NewGitHub returns the production GitHub source.
func NewGitHub() *GitHubSource {
	return NewGitHubSource(githubRepo, "https://api.github.com", httpproxy.Client())
}

// NewGitHubSource builds a source; apiBase lets tests point at httptest.
func NewGitHubSource(repo, apiBase string, client *http.Client) *GitHubSource {
	if client == nil {
		client = httpproxy.Client()
	}
	return &GitHubSource{repo: repo, apiBase: strings.TrimRight(apiBase, "/"), client: client, token: os.Getenv("GITHUB_TOKEN")}
}

func (s *GitHubSource) Name() string { return "github" }

func (s *GitHubSource) Check(ctx context.Context, currentVersion string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.apiBase+"/repos/"+s.repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "vela-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}
	var data struct {
		TagName string    `json:"tag_name"`
		Body    string    `json:"body"`
		HTMLURL string    `json:"html_url"`
		Assets  []ghAsset `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.TagName == "" || compareVersions(data.TagName, currentVersion) <= 0 {
		return nil, nil
	}

	rel := &Release{
		Source:  s.Name(),
		Tag:     data.TagName,
		Version: normalizeVersion(data.TagName),
		Notes:   data.Body,
		URL:     data.HTMLURL,
	}
	if asset := selectAsset(runtime.GOOS, runtime.GOARCH, data.TagName, data.Assets); asset != nil {
		rel.Asset = Asset{Name: asset.Name, URL: asset.BrowserDownloadURL, Size: asset.Size}
		if sumAsset := findAsset(data.Assets, "SHA256SUMS"); sumAsset != nil {
			rel.Asset.SHA256 = s.fetchChecksum(ctx, sumAsset.BrowserDownloadURL, asset.Name)
		}
	}
	return rel, nil
}

func findAsset(assets []ghAsset, name string) *ghAsset {
	for i := range assets {
		if strings.EqualFold(assets[i].Name, name) {
			return &assets[i]
		}
	}
	return nil
}

func (s *GitHubSource) fetchChecksum(ctx context.Context, url, assetName string) string {
	if url == "" {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "vela-updater")
	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return ChecksumFor(string(body), assetName)
}

func selectAsset(goos, goarch, tag string, assets []ghAsset) *ghAsset {
	for _, name := range assetCandidates(goos, goarch, tag) {
		if a := findAsset(assets, name); a != nil {
			return a
		}
	}
	return nil
}

func assetCandidates(goos, goarch, tag string) []string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	var names []string
	for _, prefix := range []string{"Vela_", "Vexo_"} {
		names = append(names, prefix+tag+"_"+goos+"_"+goarch+ext)
	}
	if goos == "darwin" {
		for _, prefix := range []string{"Vela_", "Vexo_"} {
			names = append(names, prefix+tag+"_"+goos+"_"+goarch+".zip")
		}
	}
	return names
}

// ChecksumFor returns the SHA256 for name from a SHA256SUMS document.
func ChecksumFor(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.TrimPrefix(fields[0], "*")
		file := strings.TrimPrefix(fields[len(fields)-1], "*")
		if strings.EqualFold(file, name) || strings.EqualFold(file, "./"+name) {
			return sum
		}
	}
	return ""
}
