package update

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// OpenHTTPURL opens an http(s) URL through the native platform backend.
func OpenHTTPURL(w *gui.Window, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("invalid url %q", raw)
	}
	backend := w.NativePlatformBackend()
	if backend == nil {
		return fmt.Errorf("native platform unavailable")
	}
	return backend.OpenURI(u.String())
}
