package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	genkitAI "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
)

const (
	defaultExecTimeout   = 30 * time.Second
	maxExecTimeout       = 300 * time.Second
	maxCommandLen        = 32000
	defaultMaxOutputByte = 64 * 1024
	maxMaxOutputByte     = 256 * 1024
	maxSearchResults     = 200
)

// execParams normalizes the shared timeout / output-cap arguments.
func execParams(timeoutSec, maxOutputBytes int) (time.Duration, int) {
	timeout := defaultExecTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
		if timeout > maxExecTimeout {
			timeout = maxExecTimeout
		}
	}
	maxOut := defaultMaxOutputByte
	if maxOutputBytes > 0 {
		maxOut = maxOutputBytes
		if maxOut > maxMaxOutputByte {
			maxOut = maxMaxOutputByte
		}
	}
	return timeout, maxOut
}

func validCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return errors.New("command is required")
	}
	if len(cmd) > maxCommandLen {
		return fmt.Errorf("command too long (max %d bytes)", maxCommandLen)
	}
	return nil
}

// --- input schemas ---

type searchBookmarksInput struct {
	Query    string   `json:"query,omitempty" jsonschema_description:"Free-text filter matched against title, host, user, group and tags (case-insensitive). Empty returns all."`
	Group    string   `json:"group,omitempty" jsonschema_description:"Exact bookmark group name filter (case-insensitive)."`
	Protocol string   `json:"protocol,omitempty" jsonschema_description:"Protocol filter: ssh or serial."`
	Tags     []string `json:"tags,omitempty" jsonschema_description:"Bookmark must contain all listed tags."`
}

type bookmarkIDInput struct {
	ID string `json:"id" jsonschema_description:"Bookmark id from search_bookmarks."`
}

type sessionCommandInput struct {
	SessionID      string `json:"session_id" jsonschema_description:"Session id from list_sessions."`
	Command        string `json:"command" jsonschema_description:"Shell command to run on the remote host."`
	TimeoutSec     int    `json:"timeout_sec,omitempty" jsonschema_description:"Command timeout in seconds (default 30, max 300)."`
	MaxOutputBytes int    `json:"max_output_bytes,omitempty" jsonschema_description:"Max captured output bytes (default 65536, max 262144)."`
}

type readTerminalInput struct {
	SessionID       string `json:"session_id" jsonschema_description:"Session id from list_sessions."`
	ScrollbackLines int    `json:"scrollback_lines,omitempty" jsonschema_description:"Scrollback lines to include (0 = current screen only)."`
}

type bookmarkCommandInput struct {
	BookmarkID     string `json:"bookmark_id" jsonschema_description:"SSH bookmark id from search_bookmarks."`
	Command        string `json:"command" jsonschema_description:"Shell command to run on the remote host."`
	TimeoutSec     int    `json:"timeout_sec,omitempty" jsonschema_description:"Command timeout in seconds (default 30, max 300)."`
	MaxOutputBytes int    `json:"max_output_bytes,omitempty" jsonschema_description:"Max captured output bytes (default 65536, max 262144)."`
}

type adhocInput struct {
	Host           string `json:"host" jsonschema_description:"Target hostname or IP."`
	Port           int    `json:"port,omitempty" jsonschema_description:"SSH port (default 22)."`
	User           string `json:"user" jsonschema_description:"Login user."`
	Password       string `json:"password,omitempty" jsonschema_description:"Password auth (optional if key auth is used)."`
	PrivateKeyPEM  string `json:"private_key_pem,omitempty" jsonschema_description:"Inline private key PEM (optional)."`
	KeyID          string `json:"key_id,omitempty" jsonschema_description:"Stored application SSH key id (optional)."`
	Command        string `json:"command,omitempty" jsonschema_description:"Shell command to run; omit for test_connection."`
	TimeoutSec     int    `json:"timeout_sec,omitempty" jsonschema_description:"Timeout in seconds (default 30, max 300)."`
	MaxOutputBytes int    `json:"max_output_bytes,omitempty" jsonschema_description:"Max captured output bytes (default 65536, max 262144)."`
}

// --- pure helpers (unit-testable without genkit) ---

type bookmarkMatch struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Group    string   `json:"group"`
	Protocol string   `json:"protocol"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	User     string   `json:"user"`
	Tags     []string `json:"tags,omitempty"`
}

func flattenBookmarks(groups []*bookmark.Group) []bookmark.Bookmark {
	var out []bookmark.Bookmark
	for _, g := range groups {
		if g == nil {
			continue
		}
		out = append(out, g.Bookmarks...)
	}
	return out
}

func matchQuery(b bookmark.Bookmark, q string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, field := range []string{b.Title, b.Host, b.User, b.GroupName} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	for _, t := range b.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func filterBookmarks(all []bookmark.Bookmark, in searchBookmarksInput) []bookmarkMatch {
	var out []bookmarkMatch
	for _, b := range all {
		if in.Group != "" && !strings.EqualFold(strings.TrimSpace(b.GroupName), strings.TrimSpace(in.Group)) {
			continue
		}
		if in.Protocol != "" {
			proto, ok := termio.ParseProtocol(b.Protocol)
			if !ok || string(proto) != strings.ToLower(strings.TrimSpace(in.Protocol)) {
				continue
			}
		}
		if len(in.Tags) > 0 {
			have := make(map[string]bool, len(b.Tags))
			for _, t := range b.Tags {
				have[strings.ToLower(t)] = true
			}
			ok := true
			for _, want := range in.Tags {
				if !have[strings.ToLower(strings.TrimSpace(want))] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		if !matchQuery(b, in.Query) {
			continue
		}
		out = append(out, bookmarkMatch{
			ID: b.ID, Title: b.Title, Group: b.GroupName, Protocol: b.Protocol,
			Host: b.Host, Port: b.Port, User: b.User, Tags: b.Tags,
		})
		if len(out) >= maxSearchResults {
			break
		}
	}
	return out
}

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// execError formats a failed remote command: captured output first (when any),
// then the exit/error line.
func execError(out string, err error) error {
	if msg := strings.TrimSpace(out); msg != "" {
		return fmt.Errorf("%s\nexit/error: %w", msg, err)
	}
	return fmt.Errorf("exit/error: %w", err)
}

// --- tool implementations ---

func searchBookmarks(ctx context.Context, d *Deps, in searchBookmarksInput) (string, error) {
	if d.Bookmarks == nil {
		return "", usererr.Coded("mcp.bookmarks_unavailable", "Bookmark service is not available")
	}
	groups, err := d.Bookmarks.ListGrouped()
	if err != nil {
		return "", err
	}
	matches := filterBookmarks(flattenBookmarks(groups), in)
	if matches == nil {
		matches = []bookmarkMatch{}
	}
	return marshalJSON(matches)
}

func getBookmark(ctx context.Context, d *Deps, in bookmarkIDInput) (string, error) {
	if d.Bookmarks == nil {
		return "", usererr.Coded("mcp.bookmarks_unavailable", "Bookmark service is not available")
	}
	b, err := d.Bookmarks.GetMasked(strings.TrimSpace(in.ID))
	if err != nil {
		return "", err
	}
	return marshalJSON(bookmarkMatch{
		ID: b.ID, Title: b.Title, Group: b.GroupName, Protocol: b.Protocol,
		Host: b.Host, Port: b.Port, User: b.User, Tags: b.Tags,
	})
}

func listSessions(ctx context.Context, d *Deps) (string, error) {
	if d.Hub == nil {
		return "[]", nil
	}
	infos := d.Hub.Active()
	if infos == nil {
		infos = []termio.Info{}
	}
	return marshalJSON(infos)
}

func runCommandOnSession(ctx context.Context, d *Deps, in sessionCommandInput) (string, error) {
	if d.SSH == nil {
		return "", usererr.Coded("mcp.ssh_unavailable", "SSH manager is not available")
	}
	if err := validCommand(in.Command); err != nil {
		return "", err
	}
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		return "", errors.New("session_id is required")
	}
	if d.Hub != nil {
		if sess, ok := d.Hub.Get(sessionID); ok && sess != nil && sess.Protocol() != termio.ProtocolSSH {
			return "", usererr.Coded("mcp.session_not_ssh", "Session %s is not an SSH session", sessionID)
		}
	}
	timeout, maxOut := execParams(in.TimeoutSec, in.MaxOutputBytes)
	out, err := d.SSH.Exec(ctx, sessionID, strings.TrimSpace(in.Command), timeout, maxOut)
	if err != nil {
		return "", execError(out, err)
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}

func readTerminal(ctx context.Context, d *Deps, in readTerminalInput) (string, error) {
	if d.Screen == nil {
		return "", usererr.Coded("mcp.screen_unavailable", "Terminal screen reading is not available")
	}
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		return "", errors.New("session_id is required")
	}
	lines := in.ScrollbackLines
	if lines < 0 {
		lines = 0
	}
	if lines > 10000 {
		lines = 10000
	}
	return d.Screen(sessionID, lines)
}

func runCommandOnBookmark(ctx context.Context, d *Deps, in bookmarkCommandInput) (string, error) {
	if d.SSH == nil {
		return "", usererr.Coded("mcp.ssh_unavailable", "SSH manager is not available")
	}
	if err := validCommand(in.Command); err != nil {
		return "", err
	}
	hops, proxy, err := d.resolveBookmarkDial(in.BookmarkID)
	if err != nil {
		return "", err
	}
	timeout, maxOut := execParams(in.TimeoutSec, in.MaxOutputBytes)
	out, err := d.SSH.ExecHops(ctx, hops, proxy, strings.TrimSpace(in.Command), timeout, maxOut)
	if err != nil {
		return "", execError(out, err)
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}

func testBookmark(ctx context.Context, d *Deps, in bookmarkIDInput) (string, error) {
	if d.SSH == nil {
		return "", usererr.Coded("mcp.ssh_unavailable", "SSH manager is not available")
	}
	hops, proxy, err := d.resolveBookmarkDial(in.ID)
	if err != nil {
		return "", err
	}
	if err := d.SSH.TestConnect(hops, proxy, ""); err != nil {
		return "", err
	}
	return "connection ok", nil
}

func runCommand(ctx context.Context, d *Deps, in adhocInput) (string, error) {
	if d.SSH == nil {
		return "", usererr.Coded("mcp.ssh_unavailable", "SSH manager is not available")
	}
	if err := validCommand(in.Command); err != nil {
		return "", err
	}
	target := adhocTarget{
		Host: in.Host, Port: in.Port, User: in.User,
		Password: in.Password, PrivateKeyPEM: in.PrivateKeyPEM, KeyID: in.KeyID,
	}
	hops, proxy, err := d.resolveAdhocDial(target)
	if err != nil {
		return "", err
	}
	timeout, maxOut := execParams(in.TimeoutSec, in.MaxOutputBytes)
	out, err := d.SSH.ExecHops(ctx, hops, proxy, strings.TrimSpace(in.Command), timeout, maxOut)
	if err != nil {
		return "", execError(out, err)
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}

func testConnection(ctx context.Context, d *Deps, in adhocInput) (string, error) {
	if d.SSH == nil {
		return "", usererr.Coded("mcp.ssh_unavailable", "SSH manager is not available")
	}
	target := adhocTarget{
		Host: in.Host, Port: in.Port, User: in.User,
		Password: in.Password, PrivateKeyPEM: in.PrivateKeyPEM, KeyID: in.KeyID,
	}
	hops, proxy, err := d.resolveAdhocDial(target)
	if err != nil {
		return "", err
	}
	if err := d.SSH.TestConnect(hops, proxy, ""); err != nil {
		return "", err
	}
	return "connection ok", nil
}

// --- genkit tool registration ---

func defineTools(g *genkit.Genkit, d *Deps) {
	// Dialing a host the app has not trusted yet may need a GUI confirmation.
	const dialNote = " Dials through the app's SSH stack; unknown host keys or keyboard-interactive auth may open a confirmation dialog in the Vela window and block until answered."

	genkit.DefineTool(g, "search_bookmarks",
		"Search saved bookmarks by free text, group, protocol and tags. Returns bookmark metadata only — never credentials.",
		func(tc *genkitAI.ToolContext, in searchBookmarksInput) (string, error) {
			return searchBookmarks(tc, d, in)
		})

	genkit.DefineTool(g, "get_bookmark",
		"Get one bookmark's metadata by id (credentials are masked).",
		func(tc *genkitAI.ToolContext, in bookmarkIDInput) (string, error) {
			return getBookmark(tc, d, in)
		})

	genkit.DefineTool(g, "list_sessions",
		"List currently open terminal sessions (id, protocol, label) in the Vela app.",
		func(tc *genkitAI.ToolContext, in struct{}) (string, error) {
			return listSessions(tc, d)
		})

	genkit.DefineTool(g, "run_command_on_session",
		"Run a shell command on an already-connected SSH session from list_sessions. Reuses the live connection.",
		func(tc *genkitAI.ToolContext, in sessionCommandInput) (string, error) {
			return runCommandOnSession(tc, d, in)
		})

	genkit.DefineTool(g, "read_terminal",
		"Read the current screen (plus optional scrollback) of an open terminal session.",
		func(tc *genkitAI.ToolContext, in readTerminalInput) (string, error) {
			return readTerminal(tc, d, in)
		})

	genkit.DefineTool(g, "run_command_on_bookmark",
		"Connect to a saved SSH bookmark (honoring its proxy jump chain and proxy settings) and run a shell command. Requires the Vela vault to be unlocked for bookmarks with stored secrets."+dialNote,
		func(tc *genkitAI.ToolContext, in bookmarkCommandInput) (string, error) {
			return runCommandOnBookmark(tc, d, in)
		})

	genkit.DefineTool(g, "test_bookmark",
		"Test connectivity to a saved SSH bookmark without opening a session."+dialNote,
		func(tc *genkitAI.ToolContext, in bookmarkIDInput) (string, error) {
			return testBookmark(tc, d, in)
		})

	genkit.DefineTool(g, "run_command",
		"Connect with ad-hoc SSH credentials and run a shell command. Password, inline private_key_pem and stored key_id may be combined."+dialNote,
		func(tc *genkitAI.ToolContext, in adhocInput) (string, error) {
			return runCommand(tc, d, in)
		})

	genkit.DefineTool(g, "test_connection",
		"Test connectivity with ad-hoc SSH credentials without running a command."+dialNote,
		func(tc *genkitAI.ToolContext, in adhocInput) (string, error) {
			return testConnection(tc, d, in)
		})
}
