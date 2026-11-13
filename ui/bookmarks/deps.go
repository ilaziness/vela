package bookmarks

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/ui/app"
)

// Deps are process-wide services the bookmark manager window needs.
type Deps struct {
	App       *gui.App
	Bookmarks *bookmark.Service
	Keys      *sshkey.Service
	Ctrl      *app.Controller
	Store     *config.Store
	Vault     *secret.Vault
	DB        *database.Database
	IconPNG   []byte
}
