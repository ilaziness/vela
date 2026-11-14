package app

import (
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/localfs"
	"github.com/ilaziness/vela/internal/sftp"
	"github.com/ilaziness/vela/internal/tools"
	"github.com/ilaziness/vela/internal/transfer"
)

// SFTPStatus is the panel's connection lifecycle.
type SFTPStatus int

const (
	// SFTPLoading means the SFTP client is still connecting / first
	// listings are in flight.
	SFTPLoading SFTPStatus = iota
	// SFTPReady means both panes can be used.
	SFTPReady
	// SFTPError means Connect or the first listing failed.
	SFTPError
)

// sftpDialog is the chown form's in-flight state: the remote path it
// acts on plus the uid/gid text being typed. Other dialogs are native
// prompts/confirms that capture their arguments in closures.
type sftpDialog struct {
	path   string // remote path the chown dialog acts on
	input  string // uid text
	input2 string // gid text
}

// SFTPState is the per-session SFTP panel state. It lives in the
// immediate-mode State so the UI reads and mutates it directly; all
// backend calls go through Controller methods.
type SFTPState struct {
	OwnerKey string
	Status   SFTPStatus
	Err      string

	// Local pane.
	LocalDir        string
	LocalParent     string
	LocalAtRoot     bool
	LocalShowHidden bool
	LocalEntries    []localfs.Entry
	LocalSelected   map[string]bool // keyed by entry Path
	LocalNav        string          // dir currently being listed (blocks transfers)
	LocalSeq        int             // bumps on each refresh to drop stale replies

	// Remote pane.
	RemoteDir        string
	RemoteShowHidden bool
	RemoteEntries    []sftp.FileInfo
	RemoteSelected   map[string]bool // keyed by entry Name
	RemoteNav        string
	RemoteSeq        int
	// RemoteReady flips once the first remote listing lands; cwd
	// follow is suppressed until then.
	RemoteReady bool

	// Path bar text mirrors RemoteDir/LocalDir; the input edits it.
	LocalInput  string
	RemoteInput string

	// Double-click tracking (go-gui has no click count).
	LastLocalClick    string
	LastLocalClickAt  time.Time
	LastRemoteClick   string
	LastRemoteClickAt time.Time

	// Transfers shown in the bottom list.
	Transfers []transfer.ProgressData

	// dlg is the open modal form/confirm, if any.
	dlg sftpDialog
}

// newSFTPState builds a fresh panel state. localDir seeds the local
// pane (already resolved by the caller).
func newSFTPState(ownerKey, localDir string) *SFTPState {
	return &SFTPState{
		OwnerKey:       ownerKey,
		Status:         SFTPLoading,
		LocalDir:       localDir,
		LocalSelected:  make(map[string]bool),
		RemoteSelected: make(map[string]bool),
		LocalInput:     localDir,
	}
}

// --- selection helpers (mutate state; UI-thread only) ---

// ToggleRemote selects only name (single-select); clicking the
// selected row again clears the selection.
func (s *SFTPState) ToggleRemote(name string) {
	if len(s.RemoteSelected) == 1 && s.RemoteSelected[name] {
		delete(s.RemoteSelected, name)
		return
	}
	s.RemoteSelected = map[string]bool{name: true}
}

// ToggleLocal selects only path (single-select); clicking the
// selected row again clears the selection.
func (s *SFTPState) ToggleLocal(path string) {
	if len(s.LocalSelected) == 1 && s.LocalSelected[path] {
		delete(s.LocalSelected, path)
		return
	}
	s.LocalSelected = map[string]bool{path: true}
}

// remotePaths returns full remote paths for selected entries.
func (s *SFTPState) RemotePaths() []string {
	out := make([]string, 0, len(s.RemoteSelected))
	for _, e := range s.RemoteEntries {
		if s.RemoteSelected[e.Name] {
			out = append(out, remoteJoin(s.RemoteDir, e.Name))
		}
	}
	return out
}

// localPaths returns absolute local paths for selected entries.
func (s *SFTPState) LocalPaths() []string {
	out := make([]string, 0, len(s.LocalSelected))
	for _, e := range s.LocalEntries {
		if s.LocalSelected[e.Path] {
			out = append(out, e.Path)
		}
	}
	return out
}

// pruneRemoteSelection drops selections whose names vanished.
func (s *SFTPState) PruneRemoteSelection() {
	keep := make(map[string]bool, len(s.RemoteEntries))
	for _, e := range s.RemoteEntries {
		keep[e.Name] = true
	}
	for name := range s.RemoteSelected {
		if !keep[name] {
			delete(s.RemoteSelected, name)
		}
	}
}

// pruneLocalSelection drops selections whose paths vanished.
func (s *SFTPState) PruneLocalSelection() {
	keep := make(map[string]bool, len(s.LocalEntries))
	for _, e := range s.LocalEntries {
		keep[e.Path] = true
	}
	for p := range s.LocalSelected {
		if !keep[p] {
			delete(s.LocalSelected, p)
		}
	}
}

func (s *SFTPState) ClearRemoteSelection() { s.RemoteSelected = make(map[string]bool) }

// remoteEntryByName finds an entry by name.
func (s *SFTPState) RemoteEntryByName(name string) (sftp.FileInfo, bool) {
	for _, e := range s.RemoteEntries {
		if e.Name == name {
			return e, true
		}
	}
	return sftp.FileInfo{}, false
}

// --- path helpers ---

// remoteJoin joins a remote dir and name POSIX-style.
func remoteJoin(dir, name string) string {
	if dir == "" || dir == "/" {
		return "/" + name
	}
	return strings.TrimRight(dir, "/") + "/" + name
}

// remoteParent returns the parent of a remote path.
func remoteParent(p string) string {
	p = path.Clean("/" + strings.TrimPrefix(p, "/"))
	parent := path.Dir(p)
	if parent == "." || parent == "/" {
		return "/"
	}
	return parent
}

// validRemoteName reports whether name is a safe single path element.
func validRemoteName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, "/\\")
}

// sortRemoteEntries orders dirs first, then natural name order.
func sortRemoteEntries(in []sftp.FileInfo) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].IsDir != in[j].IsDir {
			return in[i].IsDir
		}
		return naturalLess(in[i].Name, in[j].Name)
	})
}

// naturalLess compares names case-insensitively with numeric runs.
func naturalLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	for len(la) > 0 && len(lb) > 0 {
		ca, cb := la[0], lb[0]
		if ca >= '0' && ca <= '9' && cb >= '0' && cb <= '9' {
			i, j := 0, 0
			for i < len(la) && la[i] >= '0' && la[i] <= '9' {
				i++
			}
			for j < len(lb) && lb[j] >= '0' && lb[j] <= '9' {
				j++
			}
			na := strings.TrimLeft(la[:i], "0")
			nb := strings.TrimLeft(lb[:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			la, lb = la[i:], lb[j:]
			continue
		}
		if ca != cb {
			return ca < cb
		}
		la, lb = la[1:], lb[1:]
	}
	return len(la) < len(lb)
}

// --- formatting ---

// parseModeInput accepts "755", "0755", "0o755", or a 9-char rwx
// symbolic string and returns unix mode bits.
func parseModeInput(input string) (uint32, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, false
	}
	if strings.HasPrefix(input, "0o") || strings.HasPrefix(input, "0O") {
		input = input[2:]
	} else if len(input) >= 4 && input[0] == '0' {
		input = input[1:]
	}
	if len(input) >= 3 && len(input) <= 4 && allOctal(input) {
		n, err := strconv.ParseUint(input, 8, 32)
		if err == nil && n <= 0o7777 {
			return uint32(n), true
		}
		return 0, false
	}
	// Symbolic: use the tools converter (9 or 10 chars).
	res := tools.New().ConvertChmod(input, "toOctal")
	if !res.Success || res.Octal == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(res.Octal, 8, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func allOctal(s string) bool {
	for _, r := range s {
		if r < '0' || r > '7' {
			return false
		}
	}
	return true
}

// --- misc ---

// overwritePrompt builds the localized overwrite warning.
func overwritePrompt(names []string) string {
	if len(names) == 1 {
		return i18n.Tf("sftp.confirm_overwrite_one", names[0])
	}
	return i18n.Tf("sftp.confirm_overwrite_many", len(names))
}

// deletePrompt builds the localized delete warning.
func deletePrompt(paths []string) string {
	if len(paths) == 1 {
		return i18n.Tf("sftp.confirm_delete_one", path.Base(paths[0]))
	}
	return i18n.Tf("sftp.confirm_delete_many", len(paths))
}

// transferUpsert replaces or appends a progress row by ID.
func transferUpsert(list []transfer.ProgressData, p transfer.ProgressData) []transfer.ProgressData {
	for i := range list {
		if list[i].ID == p.ID {
			list[i] = p
			return list
		}
	}
	return append(list, p)
}

// transferRemove drops a row by ID.
func transferRemove(list []transfer.ProgressData, id string) []transfer.ProgressData {
	out := list[:0]
	for _, p := range list {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return out
}
