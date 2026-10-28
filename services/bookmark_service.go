package services

import (
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ilaziness/vela/internal/bookmark"
	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/usererr"
)

const (
	EventBookmarkUpdate  = "eventBookmarkUpdate"
	EventConnectBookmark = "eventConnectBookmark"
	BookmarkUpdateMsg    = "bookmark update"
)

func init() {
	application.RegisterEvent[string](EventBookmarkUpdate)
	application.RegisterEvent[string](EventConnectBookmark)
}

type SSHBookmark = bookmark.Bookmark
type BookmarkGroup = bookmark.Group
type BookmarkListItem = bookmark.ListItem
type SSHConfigImportResult = bookmark.ImportResult
type SSHConfigExportResult = bookmark.ExportResult

type BookmarkService struct {
	app  *application.App
	core *bookmark.Service
	ssh  *SSHService
}

func NewBookmarkService(app *application.App, core *bookmark.Service, ssh *SSHService) *BookmarkService {
	return &BookmarkService{app: app, core: core, ssh: ssh}
}

func (bs *BookmarkService) ConnectBookmark(bookmarkID string) {
	bs.app.Event.Emit(EventConnectBookmark, bookmarkID)
}

func (bs *BookmarkService) ConnectBookmarkByID(bookmarkID string) (string, error) {
	b, err := bs.core.GetDecrypted(bookmarkID)
	if err != nil {
		return "", err
	}
	hops, err := bs.core.ResolveHopsBookmark(*b)
	if err != nil {
		return "", err
	}
	proxy, err := resolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
		Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
		User: b.ProxyUser, Password: b.ProxyPassword,
	}, bs.sshConfig())
	if err != nil {
		return "", err
	}
	return bs.ssh.connectHops(hops, proxy)
}

func (bs *BookmarkService) sshConfig() config.SSHConfig {
	if bs.ssh == nil || bs.ssh.getSSHConfig == nil {
		return config.SSHConfig{}
	}
	return bs.ssh.getSSHConfig()
}

func (bs *BookmarkService) ListBookmarks() ([]*BookmarkGroup, error) {
	return bs.core.ListGrouped()
}
func (bs *BookmarkService) GetAllBookmarks() ([]*BookmarkListItem, error) {
	return bs.core.ListItems()
}
func (bs *BookmarkService) GetBookmarkByID(id string) (*SSHBookmark, error) {
	return bs.core.GetMasked(id)
}
func (bs *BookmarkService) SaveBookmark(b SSHBookmark) (string, error) {
	if err := validateBookmarkProxy(b); err != nil {
		return "", err
	}
	return bs.core.Save(b)
}
func (bs *BookmarkService) DeleteBookmark(id string) error {
	return bs.core.Delete(id)
}
func (bs *BookmarkService) CopyBookmark(id string) (*SSHBookmark, error) {
	return bs.core.Copy(id)
}
func (bs *BookmarkService) AddGroup(name string) error {
	return bs.core.AddGroup(name)
}
func (bs *BookmarkService) UpdateGroup(oldName, newName, icon string) error {
	return bs.core.UpdateGroup(oldName, newName, icon)
}
func (bs *BookmarkService) DeleteGroup(name string) error {
	return bs.core.DeleteGroup(name)
}
func (bs *BookmarkService) TestConnection(b SSHBookmark) error {
	prep, err := bs.core.PrepareTest(b)
	if err != nil {
		return err
	}
	hops, err := bs.core.ResolveHops(prep.Endpoint, prep.JumpID)
	if err != nil {
		return err
	}
	proxy, err := resolveDialProxy(b.ProxyMode, ssh.ProxyConfig{
		Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort,
		User: b.ProxyUser, Password: prep.ProxyPassword,
	}, bs.sshConfig())
	if err != nil {
		return err
	}
	return bs.ssh.testHops(hops, proxy)
}

func validateBookmarkProxy(b SSHBookmark) error {
	mode := strings.ToLower(strings.TrimSpace(b.ProxyMode))
	if mode == "" {
		mode = "inherit"
	}
	switch mode {
	case "inherit", "none":
		return nil
	case "custom":
		p := ssh.ProxyConfig{Type: b.ProxyType, Host: b.ProxyHost, Port: b.ProxyPort}
		return p.Validate(i18n.T("bookmark.custom_proxy"))
	default:
		return usererr.Coded("bookmark.proxy_mode_invalid", "Invalid proxy mode: %s", b.ProxyMode)
	}
}

func (bs *BookmarkService) SaveAndConnect(b SSHBookmark) (string, error) {
	if err := bs.TestConnection(b); err != nil {
		return "", usererr.Coded("bookmark.test_failed", "Connection test failed: %s", err.Error())
	}
	id, err := bs.SaveBookmark(b)
	if err != nil {
		return "", usererr.Coded("bookmark.save_failed", "Failed to save bookmark: %s", err.Error())
	}
	bs.ConnectBookmark(id)
	return id, nil
}

// ImportSSHConfig opens a file dialog and imports OpenSSH config into groupName.
// Cancelled dialog returns Cancelled=true without error.
func (bs *BookmarkService) ImportSSHConfig(groupName string) (SSHConfigImportResult, error) {
	path, err := bs.app.Dialog.OpenFile().
		SetTitle(i18n.T("dialog.select_openssh_config")).
		CanChooseDirectories(false).
		CanChooseFiles(true).
		PromptForSingleSelection()
	if dialogCancelled(err) || path == "" {
		return SSHConfigImportResult{Cancelled: true}, nil
	}
	if err != nil {
		return SSHConfigImportResult{}, err
	}
	return bs.core.ImportOpenSSH(path, groupName)
}

// ExportSSHConfig writes bookmarks as OpenSSH config via save dialog.
// groupName empty exports all. Cancelled dialog returns Cancelled=true without error.
func (bs *BookmarkService) ExportSSHConfig(groupName string) (SSHConfigExportResult, error) {
	content, warnings, err := bs.core.ExportOpenSSH(groupName)
	if err != nil {
		return SSHConfigExportResult{Warnings: warnings}, err
	}
	if strings.TrimSpace(content) == "" {
		if len(warnings) == 0 {
			warnings = []string{i18n.T("bookmark.export_empty")}
		}
		return SSHConfigExportResult{Warnings: warnings}, nil
	}
	if len(warnings) > 0 {
		var b strings.Builder
		for _, w := range warnings {
			b.WriteString("# warning: ")
			b.WriteString(w)
			b.WriteByte('\n')
		}
		b.WriteString(content)
		content = b.String()
	}
	path, err := promptSaveFile(bs.app, i18n.T("dialog.export_openssh_config"), "config")
	if dialogCancelled(err) || path == "" {
		return SSHConfigExportResult{Cancelled: true, Warnings: warnings}, nil
	}
	if err != nil {
		return SSHConfigExportResult{Warnings: warnings}, err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return SSHConfigExportResult{Warnings: warnings}, err
	}
	return SSHConfigExportResult{Written: true, Warnings: warnings}, nil
}
