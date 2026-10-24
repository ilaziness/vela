package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultSearchCount = 8
	searchHTTPTimeout  = 30 * time.Second
	maxSearchQueryLen  = 512
)

// SearchResult is one web hit returned to the model.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchConfig is runtime search credentials.
type SearchConfig struct {
	Provider SearchProvider
	APIKey   string
	Count    int
}

// Search runs a query against the configured provider.
func Search(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}
	if len(query) > maxSearchQueryLen {
		return nil, fmt.Errorf("query too long (max %d characters)", maxSearchQueryLen)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("web search API key not configured")
	}
	if cfg.Count <= 0 {
		cfg.Count = defaultSearchCount
	}
	if cfg.Count > 20 {
		cfg.Count = 20
	}
	switch cfg.Provider {
	case SearchTavily:
		return searchTavily(ctx, cfg, query)
	case SearchBrave:
		return searchBrave(ctx, cfg, query)
	case SearchSerper:
		return searchSerper(ctx, cfg, query)
	case SearchDeepseek:
		return searchDeepseek(ctx, cfg, query)
	case SearchYuanbao:
		return searchYuanbao(ctx, cfg, query)
	case SearchBocha:
		return searchBocha(ctx, cfg, query)
	case SearchMetaso:
		return searchMetaso(ctx, cfg, query)
	default:
		return nil, fmt.Errorf("unsupported web search provider: %s", cfg.Provider)
	}
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: searchHTTPTimeout,
		// Do not follow redirects: keys in headers/body must not be replayed to another host.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func doJSON(ctx context.Context, method, endpoint string, headers map[string]string, body any, dest any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rdr)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("search API HTTP %d: redirects are not allowed", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("search API HTTP %d", resp.StatusCode)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(data, dest)
}

func searchTavily(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	var raw struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	err := doJSON(ctx, http.MethodPost, "https://api.tavily.com/search", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	}, map[string]any{
		"query":       query,
		"max_results": cfg.Count,
	}, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return out, nil
}

func searchBrave(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	u := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) +
		"&count=" + fmt.Sprintf("%d", cfg.Count)
	var raw struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	err := doJSON(ctx, http.MethodGet, u, map[string]string{
		"Accept":               "application/json",
		"X-Subscription-Token": cfg.APIKey,
	}, nil, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Web.Results))
	for _, r := range raw.Web.Results {
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return out, nil
}

func searchSerper(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	var raw struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	err := doJSON(ctx, http.MethodPost, "https://google.serper.dev/search", map[string]string{
		"X-API-KEY": cfg.APIKey,
	}, map[string]any{"q": query, "num": cfg.Count}, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Organic))
	for _, r := range raw.Organic {
		out = append(out, SearchResult{Title: r.Title, URL: r.Link, Snippet: r.Snippet})
	}
	return out, nil
}

func searchDeepseek(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	// Official limit: query 1–128 Unicode characters.
	if r := []rune(query); len(r) > 128 {
		query = string(r[:128])
	}
	var raw struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Summary string `json:"summary"`
		} `json:"results"`
	}
	err := doJSON(ctx, http.MethodPost, "https://api.deepseek.com/v1/tools/search", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	}, map[string]any{
		"query": query,
		"count": cfg.Count,
	}, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		snip := r.Summary
		if snip == "" {
			snip = r.Snippet
		}
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: snip})
	}
	return out, nil
}

func searchBocha(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	var raw struct {
		Data struct {
			WebPages struct {
				Value []struct {
					Name    string `json:"name"`
					URL     string `json:"url"`
					Snippet string `json:"snippet"`
					Summary string `json:"summary"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	err := doJSON(ctx, http.MethodPost, "https://api.bochaai.com/v1/web-search", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	}, map[string]any{
		"query":   query,
		"summary": true,
		"count":   cfg.Count,
	}, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Data.WebPages.Value))
	for _, r := range raw.Data.WebPages.Value {
		snip := r.Summary
		if snip == "" {
			snip = r.Snippet
		}
		out = append(out, SearchResult{Title: r.Name, URL: r.URL, Snippet: snip})
	}
	return out, nil
}

func searchMetaso(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	var raw struct {
		Webpages []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"webpages"`
	}
	err := doJSON(ctx, http.MethodPost, "https://metaso.cn/api/v1/search", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
		"Accept":        "application/json",
	}, map[string]any{
		"q":     query,
		"scope": "webpage",
		"size":  cfg.Count,
	}, &raw)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(raw.Webpages))
	for _, r := range raw.Webpages {
		out = append(out, SearchResult{Title: r.Title, URL: r.Link, Snippet: r.Snippet})
	}
	return out, nil
}

func searchYuanbao(ctx context.Context, cfg SearchConfig, query string) ([]SearchResult, error) {
	var raw struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
			Pages []string `json:"Pages"`
		} `json:"Response"`
	}
	err := doJSON(ctx, http.MethodPost, "https://api.wsa.cloud.tencent.com/SearchPro", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
		"Content-Type":  "application/json; charset=UTF-8",
	}, map[string]any{
		"Query": query,
		"Cnt":   yuanbaoCnt(cfg.Count),
	}, &raw)
	if err != nil {
		return nil, err
	}
	if e := raw.Response.Error; e != nil && e.Code != "" {
		return nil, fmt.Errorf("yuanbao search: %s %s", e.Code, e.Message)
	}
	out := make([]SearchResult, 0, len(raw.Response.Pages))
	for _, page := range raw.Response.Pages {
		var item struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Passage string `json:"passage"`
			Content string `json:"content"`
		}
		if json.Unmarshal([]byte(page), &item) != nil {
			continue
		}
		snip := item.Content
		if snip == "" {
			snip = item.Passage
		}
		out = append(out, SearchResult{Title: item.Title, URL: item.URL, Snippet: snip})
	}
	return out, nil
}

func yuanbaoCnt(n int) int {
	if n <= 10 {
		return 10
	}
	return 20
}
