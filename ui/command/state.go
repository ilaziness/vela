package commandui

import (
	"github.com/ilaziness/vela/internal/command"
	"github.com/ilaziness/vela/ui/chrome"
)

// State is per-command-window draft state (independent of app.State).
type State struct {
	// Frameless is the decoration mode captured at window creation;
	// BarClicks feeds the title strip's double-click maximize.
	Frameless bool
	BarClicks chrome.ClickTracker
	Lang      string

	// all/hist cache the DB-backed command data; views rebuild often, so
	// they must not query SQLite per frame. reload refreshes both.
	all        map[string][]command.Info
	hist       []command.History
	dataLoaded bool

	// selectedSess holds the session IDs picked for command delivery.
	selectedSess map[string]bool

	// selCat/selName identify the command leaf last clicked in the tree;
	// selCustom gates the delete button (only user commands are removable).
	selCat    string
	selName   string
	selCustom bool

	input string

	// addDraft backs the add-command dialog form.
	addCat  string
	addName string
	addDesc string
	addCmd  string
}

func (s *State) ensureLoaded() {
	if s.selectedSess == nil {
		s.selectedSess = make(map[string]bool)
	}
}

// reload snapshots commands and history. Called on open, on language
// change, and after every mutation (add/delete/clear/send).
func (s *State) reload(d *Deps) {
	s.all = d.Commands.All()
	s.hist = d.Commands.History()
	s.dataLoaded = true
}

// selectedIDs returns the picked session IDs in stable order.
func (s *State) selectedIDs() []string {
	out := make([]string, 0, len(s.selectedSess))
	for id := range s.selectedSess {
		out = append(out, id)
	}
	return out
}
