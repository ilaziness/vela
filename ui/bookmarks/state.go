package bookmarks

import (
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/sshkey"
)

const (
	tabBasic      = "basic"
	tabConnection = "connection"
	tabAuth       = "auth"
	tabSession    = "session"

	selNone     = ""
	selGroup    = "group"
	selBookmark = "bookmark"
)

// State is per-bookmark-manager-window draft + tree selection.
type State struct {
	Groups []*bookmark.Group
	Keys   []sshkey.Info

	Search string

	SelKind string // group | bookmark | ""
	SelID   string // group name or bookmark id

	Draft    bookmark.Bookmark
	EnvText  string
	TagDraft string
	Editing  bool // right pane shows editor

	HasPassword    bool
	HasKeyPassword bool
	HasProxyPass   bool

	ActiveTab string
	Busy      bool
	Dirty     bool
	Lang      string
	loaded    bool

	// Dialog drafts for import/export/rename/add-group/color.
	ImportGroup string
	ImportBusy  bool
	ImportError string
	ImportGen   int    // bumped to ignore late import callbacks after cancel
	ExportGroup string // "" = all
	DialogName  string
	ColorDraft  gui.Color
}

func (s *State) ensureLoaded() {
	if s.loaded {
		return
	}
	s.ActiveTab = tabBasic
	s.ImportGroup = bookmark.ImportedGroupName
	s.ExportGroup = ""
	s.loaded = true
}

func (s *State) clearSelection() {
	s.SelKind = selNone
	s.SelID = ""
	s.Editing = false
	s.Dirty = false
	s.Draft = bookmark.Bookmark{}
	s.EnvText = ""
	s.TagDraft = ""
	s.HasPassword = false
	s.HasKeyPassword = false
	s.HasProxyPass = false
	s.ActiveTab = tabBasic
}
