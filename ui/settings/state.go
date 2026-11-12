package settings

import (
	"github.com/ilaziness/vela/internal/ai"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/keybind"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/sshkey"
	"github.com/ilaziness/vela/internal/sync"
)

const (
	navGeneral     = "general"
	navTerminal    = "terminal"
	navSSH         = "ssh"
	navSync        = "sync"
	navAI          = "ai"
	navKeys        = "keys"
	navKnownHosts  = "known_hosts"
	navKeybindings = "keybindings"
	navAbout       = "about"
)

const (
	aiTabGeneral = "general"
	aiTabTools   = "tools"
	aiTabMCP     = "mcp"
	aiTabSkills  = "skills"
)

// State is per-settings-window draft + list cache (independent of app.State).
type State struct {
	ActiveNav string
	Busy      bool

	General  config.GeneralConfig
	Terminal config.TerminalConfig
	SSH      config.SSHConfig
	Sync     sync.SyncConfig
	AI       ai.Config
	AITab    string

	Keys            []sshkey.Info
	KeyGenName      string
	KeyGenAlgo      string
	KeyGenComment   string
	KnownHosts      []ssh.KnownHostEntry
	Keybindings     []keybind.Info
	RecordingAction string

	MCPServers []ai.MCPServerEntry
	Skills     []ai.SkillInfo

	loaded bool
}

func (s *State) ensureLoaded(d *Deps) {
	if s.loaded {
		return
	}
	cfg := d.Store.Config
	s.ActiveNav = navGeneral
	s.General = cfg.General
	s.Terminal = cfg.Terminal
	s.SSH = cfg.SSH
	s.Sync = cfg.Sync
	if s.Sync.Provider == "" {
		s.Sync.Provider = sync.ProviderSelfHosted
	}
	s.AI = cfg.AI
	s.AI.APIKey = ""
	s.AI.WebSearch.APIKey = ""
	s.AITab = aiTabGeneral
	s.KeyGenAlgo = "ed25519"
	s.loaded = true
	refreshLists(s, d)
}

func refreshLists(s *State, d *Deps) {
	if list, err := d.Keys.List(); err == nil {
		s.Keys = list
	}
	if list, err := d.SSH.ListKnownHosts(); err == nil {
		s.KnownHosts = list
	}
	s.Keybindings = d.Keybind.List()
	dir := d.Store.UserDataDir()
	if mcp, err := ai.LoadMCPConfig(dir); err == nil {
		s.MCPServers = mcp.Entries()
	}
	if skills, err := ai.ListSkills(dir); err == nil {
		s.Skills = skills
	}
}
