package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestGitHubCheckNewRelease(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	assetName := fmt.Sprintf("Vela_v9.9.9_%s_%s%s", goos, goarch, ext)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"tag_name":"v9.9.9","body":"notes","html_url":"https://example.com/r","assets":[
				{"name":%q,"browser_download_url":%q,"size":10},
				{"name":"SHA256SUMS","browser_download_url":%q,"size":80}
			]}`, assetName, srv.URL+"/asset", srv.URL+"/sums")
		case "/sums":
			fmt.Fprintf(w, "deadbeef  %s\n", assetName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	src := NewGitHubSource("o/r", srv.URL, srv.Client())
	rel, err := src.Check(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.Tag != "v9.9.9" || rel.Version != "9.9.9" {
		t.Fatalf("rel=%+v", rel)
	}
	if rel.Asset.Name != assetName || rel.Asset.SHA256 != "deadbeef" {
		t.Fatalf("asset=%+v", rel.Asset)
	}
}

func TestGitHubCheckUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.0.0","body":"","html_url":"","assets":[]}`)
	}))
	defer srv.Close()
	src := NewGitHubSource("o/r", srv.URL, srv.Client())
	rel, err := src.Check(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if rel != nil {
		t.Fatalf("expected nil, got %+v", rel)
	}
}
