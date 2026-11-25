package app

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/tunnel"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/utils"
	"github.com/ilaziness/vela/ui/errtext"
)

// TunnelDraft is the raw create-form input (strings straight from the
// dialog); RemotePort stays text so the form can show inline errors.
type TunnelDraft struct {
	Type       string
	LocalAddr  string
	RemoteAddr string
	RemotePort string
}

// TunnelArgs is a validated start request.
type TunnelArgs struct {
	Type       string
	LocalAddr  string
	RemoteAddr string
	RemotePort int
}

// TunnelRule is a persisted bookmark-scoped tunnel rule. It is only a
// saved configuration: starting it creates a fresh runtime tunnel.
type TunnelRule struct {
	ID         string
	BookmarkID string
	Type       string
	LocalAddr  string
	RemoteAddr string
	RemotePort int
}

// Summary is the one-line address description shown in lists/dialogs.
func (r TunnelRule) Summary() string {
	switch r.Type {
	case tunnel.TypeRemote:
		return fmt.Sprintf("%s → 127.0.0.1:%d", r.LocalAddr, r.RemotePort)
	case tunnel.TypeLocal:
		return fmt.Sprintf("%s → %s", r.LocalAddr, r.RemoteAddr)
	default:
		return r.LocalAddr
	}
}

// tunnelRuleCache is the Controller-side rules store, keyed by
// bookmark id. Items are replaced wholesale on refresh and never
// mutated in place, so readers may share the slice.
type tunnelRuleCache struct {
	loaded bool
	items  []TunnelRule
	err    string
}

// SetTunnels injects the tunnel engine (bootstrap).
func (c *Controller) SetTunnels(m *tunnel.Manager) {
	c.mu.Lock()
	c.tunnels = m
	c.mu.Unlock()
}

// TunnelManager exposes the engine for lifecycle teardown.
func (c *Controller) TunnelManager() *tunnel.Manager {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tunnels
}

// TunnelControls reports whether the tab may show the tunnel panel.
func TunnelControls(tab *Tab) bool {
	return tab != nil && tab.Kind == TabSession && tab.SessID != "" && tab.Protocol == termio.ProtocolSSH
}

// TunnelTypeLabel is the localized tunnel type name.
func TunnelTypeLabel(t string) string {
	switch t {
	case tunnel.TypeLocal:
		return i18n.T("validation.tunnel_local")
	case tunnel.TypeRemote:
		return i18n.T("validation.tunnel_remote")
	case tunnel.TypeDynamic:
		return i18n.T("validation.tunnel_dynamic")
	default:
		return t
	}
}

// TunnelSnapshot returns the session's running tunnels, sorted by type
// then address. Call from view build only.
func (c *Controller) TunnelSnapshot(sessID string) []tunnel.TunnelInfo {
	mgr := c.TunnelManager()
	if mgr == nil || sessID == "" {
		return nil
	}
	var out []tunnel.TunnelInfo
	for _, group := range mgr.List() {
		for _, info := range group.Tunnels {
			if info.SessionID == sessID {
				out = append(out, info)
			}
		}
	}
	sortTunnelInfos(out)
	return out
}

// TunnelRules returns the cached saved rules for a bookmark.
func (c *Controller) TunnelRules(bookmarkID string) (items []TunnelRule, loaded bool, errMsg string) {
	if bookmarkID == "" {
		return nil, true, ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	rc := c.tunnelRules[bookmarkID]
	if rc == nil {
		return nil, false, ""
	}
	return rc.items, rc.loaded, rc.err
}

// TunnelRuleActive reports whether a rule already has a matching
// running tunnel in this session.
func (c *Controller) TunnelRuleActive(sessID string, rule TunnelRule) bool {
	for _, info := range c.TunnelSnapshot(sessID) {
		if ruleMatchesInfo(rule, info) {
			return true
		}
	}
	return false
}

// refreshTunnelRules reloads the bookmark's saved rules off the UI
// thread and writes the cache back under c.mu.
func (c *Controller) refreshTunnelRules(w *gui.Window, bookmarkID string) {
	if bookmarkID == "" {
		return
	}
	c.mu.Lock()
	if c.tunnelRules == nil {
		c.tunnelRules = make(map[string]*tunnelRuleCache)
	}
	rc := c.tunnelRules[bookmarkID]
	if rc == nil {
		rc = &tunnelRuleCache{}
		c.tunnelRules[bookmarkID] = rc
	}
	rc.loaded = false
	rc.err = ""
	db := c.db
	c.mu.Unlock()

	if db == nil || db.SSHTunnelRepo == nil {
		c.mu.Lock()
		rc.loaded = true
		c.mu.Unlock()
		return
	}
	go func() {
		rows, err := db.SSHTunnelRepo.ListByBookmark(bookmarkID)
		items := make([]TunnelRule, 0, len(rows))
		for _, r := range rows {
			items = append(items, TunnelRule{
				ID: r.ID, BookmarkID: r.BookmarkID, Type: r.TunnelType,
				LocalAddr: r.LocalAddr, RemoteAddr: r.RemoteAddr, RemotePort: r.RemotePort,
			})
		}
		errMsg := ""
		if err != nil {
			c.logger.Warn("list tunnel rules failed", zap.String("bookmark", bookmarkID), zap.Error(err))
			errMsg = errtext.Text(err)
		}
		c.mu.Lock()
		rc.items = items
		rc.loaded = true
		rc.err = errMsg
		c.mu.Unlock()
		w.InvalidateLayout()
	}()
}

// ValidateTunnelDraft parses and validates form input, returning a
// localized message ("" when valid) for inline dialog display.
func ValidateTunnelDraft(d TunnelDraft) string {
	if _, err := tunnelArgsFromDraft(d); err != nil {
		return errtext.Text(err)
	}
	return ""
}

// StartTunnel validates and starts a tunnel on the tab's session. For
// bookmark sessions the rule is persisted (deduplicated) after a
// successful start; quick-connect sessions stay temporary.
func (c *Controller) StartTunnel(w *gui.Window, tabID string, draft TunnelDraft) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || !TunnelControls(tab) {
		return
	}
	sessID := tab.SessID
	bookmarkID := tab.BookmarkID
	args, err := tunnelArgsFromDraft(draft)
	if err != nil {
		w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	mgr := c.TunnelManager()
	if mgr == nil {
		w.Toast(gui.ToastCfg{Body: i18n.T("tunnel.start_tunnel_failed"), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	go func() {
		_, err := startTunnelByType(mgr, sessID, args)
		if err != nil {
			w.QueueCommand(func(w *gui.Window) {
				w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
			})
			return
		}
		var saveErr error
		if bookmarkID != "" {
			saveErr = c.saveTunnelRule(bookmarkID, args)
		}
		w.QueueCommand(func(w *gui.Window) {
			if saveErr != nil {
				c.logger.Warn("save tunnel rule failed", zap.String("bookmark", bookmarkID), zap.Error(saveErr))
				w.Toast(gui.ToastCfg{Body: i18n.T("tunnel.save_rule_failed"), Severity: gui.ToastError, Duration: 4 * time.Second})
			} else {
				w.Toast(gui.ToastCfg{
					Body:     i18n.Tf("tunnel.start_success", TunnelTypeLabel(args.Type)),
					Severity: gui.ToastSuccess,
					Duration: 3 * time.Second,
				})
			}
			if bookmarkID != "" {
				c.refreshTunnelRules(w, bookmarkID)
			}
			w.InvalidateLayout()
		})
	}()
}

// StartTunnelRule starts a saved rule on the tab's session. The rule
// is not re-saved; it belongs to the tab's bookmark.
func (c *Controller) StartTunnelRule(w *gui.Window, tabID string, rule TunnelRule) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || !TunnelControls(tab) || rule.BookmarkID == "" || rule.BookmarkID != tab.BookmarkID {
		return
	}
	sessID := tab.SessID
	args := TunnelArgs{Type: rule.Type, LocalAddr: rule.LocalAddr, RemoteAddr: rule.RemoteAddr, RemotePort: rule.RemotePort}
	if err := validateTunnelArgs(args); err != nil {
		w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	if c.TunnelRuleActive(sessID, rule) {
		w.Toast(gui.ToastCfg{Body: i18n.T("tunnel.already_running"), Severity: gui.ToastInfo, Duration: 3 * time.Second})
		return
	}
	mgr := c.TunnelManager()
	if mgr == nil {
		return
	}
	go func() {
		_, err := startTunnelByType(mgr, sessID, args)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				return
			}
			w.Toast(gui.ToastCfg{
				Body:     i18n.Tf("tunnel.start_success", TunnelTypeLabel(args.Type)),
				Severity: gui.ToastSuccess,
				Duration: 3 * time.Second,
			})
			w.InvalidateLayout()
		})
	}()
}

// StopTunnel stops one running tunnel by runtime id. Saved rules are
// untouched — stopping never deletes a rule.
func (c *Controller) StopTunnel(w *gui.Window, tunnelID string) {
	mgr := c.TunnelManager()
	if mgr == nil || tunnelID == "" {
		return
	}
	go func() {
		err := mgr.StopByID(tunnelID)
		w.QueueCommand(func(w *gui.Window) {
			if err != nil {
				w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
				return
			}
			w.Toast(gui.ToastCfg{Body: i18n.T("tunnel.stop_success"), Severity: gui.ToastSuccess, Duration: 3 * time.Second})
			w.InvalidateLayout()
		})
	}()
}

// DeleteTunnelRule removes a saved rule after confirmation. A running
// tunnel with the same config keeps running; only the rule is deleted.
func (c *Controller) DeleteTunnelRule(w *gui.Window, tabID string, rule TunnelRule) {
	st := gui.State[State](w)
	tab := st.TabByID(tabID)
	if tab == nil || tab.BookmarkID == "" || rule.ID == "" || rule.BookmarkID != tab.BookmarkID {
		return
	}
	bookmarkID := tab.BookmarkID
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogConfirm,
		Title:      i18n.T("tunnel.rule_delete_title"),
		Body:       i18n.Tf("tunnel.rule_delete_body", rule.Summary()),
		OnOkYes: func(win *gui.Window) {
			go func() {
				c.mu.Lock()
				db := c.db
				c.mu.Unlock()
				var err error
				if db != nil && db.SSHTunnelRepo != nil {
					err = db.SSHTunnelRepo.Delete(rule.ID, bookmarkID)
				}
				win.QueueCommand(func(win *gui.Window) {
					if err != nil {
						win.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
						return
					}
					win.Toast(gui.ToastCfg{Body: i18n.T("tunnel.rule_deleted"), Severity: gui.ToastSuccess, Duration: 3 * time.Second})
					c.refreshTunnelRules(win, bookmarkID)
				})
			}()
		},
	})
}

// saveTunnelRule inserts the rule unless an identical one exists.
func (c *Controller) saveTunnelRule(bookmarkID string, args TunnelArgs) error {
	c.mu.Lock()
	db := c.db
	c.mu.Unlock()
	if db == nil || db.SSHTunnelRepo == nil {
		return nil
	}
	rows, err := db.SSHTunnelRepo.ListByBookmark(bookmarkID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if r.TunnelType == args.Type && r.LocalAddr == args.LocalAddr &&
			r.RemoteAddr == args.RemoteAddr && r.RemotePort == args.RemotePort {
			return nil
		}
	}
	return db.SSHTunnelRepo.Insert(&database.SSHTunnelDB{
		ID:         utils.GenerateRandomID(),
		BookmarkID: bookmarkID,
		TunnelType: args.Type,
		LocalAddr:  args.LocalAddr,
		RemoteAddr: args.RemoteAddr,
		RemotePort: args.RemotePort,
	})
}

func startTunnelByType(mgr *tunnel.Manager, sessID string, a TunnelArgs) (string, error) {
	switch a.Type {
	case tunnel.TypeLocal:
		return mgr.StartLocal(sessID, a.LocalAddr, a.RemoteAddr)
	case tunnel.TypeRemote:
		return mgr.StartRemote(sessID, a.RemotePort, a.LocalAddr)
	case tunnel.TypeDynamic:
		return mgr.StartDynamic(sessID, a.LocalAddr)
	default:
		return "", usererr.Coded("validation.tunnel", "Unknown tunnel type: %s", a.Type)
	}
}

// tunnelArgsFromDraft converts raw form strings into validated args.
func tunnelArgsFromDraft(d TunnelDraft) (TunnelArgs, error) {
	a := TunnelArgs{
		Type:       strings.TrimSpace(d.Type),
		LocalAddr:  strings.TrimSpace(d.LocalAddr),
		RemoteAddr: strings.TrimSpace(d.RemoteAddr),
	}
	if a.Type == tunnel.TypeRemote {
		portText := strings.TrimSpace(d.RemotePort)
		if portText == "" {
			return a, usererr.Coded("validation.port_required", "Port is required")
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			return a, usererr.Coded("validation.port_range", "Port must be 1-65535")
		}
		a.RemotePort = port
	}
	return a, validateTunnelArgs(a)
}

func validateTunnelArgs(a TunnelArgs) error {
	switch a.Type {
	case tunnel.TypeLocal:
		if err := tunnel.ValidateListenAddr(a.LocalAddr); err != nil {
			return err
		}
		return tunnel.ValidateTargetAddr(a.RemoteAddr)
	case tunnel.TypeRemote:
		if err := tunnel.ValidateTargetAddr(a.LocalAddr); err != nil {
			return err
		}
		return tunnel.ValidatePort(a.RemotePort)
	case tunnel.TypeDynamic:
		return tunnel.ValidateListenAddr(a.LocalAddr)
	default:
		return usererr.Coded("validation.tunnel", "Unknown tunnel type: %s", a.Type)
	}
}

// ruleMatchesInfo reports whether a running tunnel corresponds to a
// saved rule. Remote tunnels record the listener as 127.0.0.1:port in
// RemoteAddr, so the port is compared numerically.
func ruleMatchesInfo(rule TunnelRule, info tunnel.TunnelInfo) bool {
	if info.TunnelType != rule.Type {
		return false
	}
	switch rule.Type {
	case tunnel.TypeLocal:
		return info.LocalAddr == rule.LocalAddr && info.RemoteAddr == rule.RemoteAddr
	case tunnel.TypeDynamic:
		return info.LocalAddr == rule.LocalAddr
	case tunnel.TypeRemote:
		if info.LocalAddr != rule.LocalAddr {
			return false
		}
		_, port, err := net.SplitHostPort(info.RemoteAddr)
		if err != nil {
			return false
		}
		n, err := strconv.Atoi(port)
		return err == nil && n == rule.RemotePort
	default:
		return false
	}
}

func tunnelTypeRank(t string) int {
	switch t {
	case tunnel.TypeLocal:
		return 0
	case tunnel.TypeRemote:
		return 1
	default:
		return 2
	}
}

func sortTunnelInfos(list []tunnel.TunnelInfo) {
	sort.SliceStable(list, func(i, j int) bool {
		ri, rj := tunnelTypeRank(list[i].TunnelType), tunnelTypeRank(list[j].TunnelType)
		if ri != rj {
			return ri < rj
		}
		if list[i].LocalAddr != list[j].LocalAddr {
			return list[i].LocalAddr < list[j].LocalAddr
		}
		return list[i].RemoteAddr < list[j].RemoteAddr
	})
}
