package app

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-term/term"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/utils"
	"github.com/ilaziness/vela/ui/errtext"
	"github.com/ilaziness/vela/ui/fonts"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/protocol"
)

// defaultReplayIdleLimit caps a single inter-frame gap while replaying so a
// recording with a long think-pause stays watchable. It is a product default,
// not part of the .gtr format; a settings knob can replace it later.
const defaultReplayIdleLimit = 2 * time.Second

// maxRecordingsListed bounds the picker list; older files remain reachable
// through the native file dialog.
const maxRecordingsListed = 100

// IsReplayProtocol reports whether p marks a local .gtr playback pane.
func IsReplayProtocol(p termio.Protocol) bool { return p == termio.ProtocolReplay }

type recordingItem struct {
	name    string
	path    string
	size    int64
	modTime time.Time
}

// listRecordings returns up to maxRecordingsListed .gtr files in dir, newest
// first. A missing directory is an empty list, not an error.
func (c *Controller) listRecordings(dir string) ([]recordingItem, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]recordingItem, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".gtr") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, recordingItem{
			name:    name,
			path:    filepath.Join(dir, name),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].modTime.After(items[j].modTime) })
	if len(items) > maxRecordingsListed {
		items = items[:maxRecordingsListed]
	}
	return items, nil
}

type recordingPickerState struct {
	tabID    string
	dir      string
	items    []recordingItem
	selected string
}

// ShowRecordingPicker opens the .gtr picker used by the connect form. It lists
// recent recordings from recordingDir and offers a native file fallback.
func (c *Controller) ShowRecordingPicker(w *gui.Window, tabID string) {
	dir, err := c.recordingDir()
	if err != nil {
		c.logger.Error("resolve recording dir", zap.Error(err))
		w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	items, err := c.listRecordings(dir)
	if err != nil {
		c.logger.Warn("list recordings failed", zap.String("dir", dir), zap.Error(err))
		items = nil
	}
	picker := &recordingPickerState{tabID: tabID, dir: dir, items: items}
	w.Dialog(gui.DialogCfg{
		DialogType: gui.DialogCustom,
		Title:      i18n.T("terminal.replay_pick_title"),
		Width:      560,
		Height:     480,
		CustomView: func(win *gui.Window) gui.View {
			return c.recordingPickerView(picker)
		},
	})
}

func (c *Controller) recordingPickerView(p *recordingPickerState) gui.View {
	th := gui.CurrentTheme()
	body := []gui.View{
		gui.Text(gui.TextCfg{
			Text:      i18n.T("terminal.replay_pick_hint"),
			TextStyle: th.TextStyleBody,
			Mode:      gui.TextModeWrap,
			Sizing:    gui.FillFit,
		}),
	}

	if len(p.items) == 0 {
		body = append(body, gui.Text(gui.TextCfg{
			Text:      i18n.T("terminal.no_recordings"),
			TextStyle: th.TextStyleSecondary,
		}))
	} else {
		rows := make([]gui.View, 0, len(p.items))
		for i, it := range p.items {
			bg := gui.Color{}
			if p.selected == it.path {
				bg = th.ColorActive
			}
			secondary := utils.FormatBytes(it.size) + " · " + it.modTime.Format("2006-01-02 15:04:05")
			rows = append(rows, gui.Button(gui.ButtonCfg{
				ID:      "recording-item-" + strconv.Itoa(i),
				Variant: gui.ButtonGhost,
				Sizing:  gui.FillFit,
				Color:   bg,
				HAlign:  gui.Some(gui.HAlignStart),
				Content: []gui.View{
					gui.Column(layout.Flush(gui.ContainerCfg{
						Sizing:  gui.FillFit,
						Spacing: gui.SpacingTight,
						HAlign:  gui.HAlignStart,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: it.name, TextStyle: th.TextStyleBody}),
							gui.Text(gui.TextCfg{Text: secondary, TextStyle: th.TextStyleSecondary}),
						},
					})),
				},
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					p.selected = it.path
				},
			}))
		}
		body = append(body, gui.Column(layout.Flush(gui.ContainerCfg{
			ID:         "recording-list",
			Sizing:     gui.FillFill,
			Scrollable: true,
			MaxHeight:  300,
			Spacing:    gui.SpacingTight,
			Content:    rows,
		})))
	}

	body = append(body, gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		HAlign:  gui.HAlignEnd,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "recording-pick-cancel",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("common.cancel"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "recording-pick-file",
				Variant: gui.ButtonSecondary,
				Label:   i18n.T("common.select_file"),
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					startDir := p.dir
					if startDir == "" {
						startDir = c.lastDialogDir()
					}
					ctx.Window.NativeOpenDialog(gui.NativeOpenDialogCfg{
						Title:    i18n.T("terminal.replay_pick_title"),
						StartDir: startDir,
						Filters:  []gui.NativeFileFilter{{Extensions: []string{"gtr"}}},
						OnDone: func(res gui.NativeDialogResult, win *gui.Window) {
							if res.Status != gui.DialogOK {
								return
							}
							paths := res.PathStrings()
							if len(paths) == 0 {
								return
							}
							path := paths[0]
							c.rememberDialogPath(path)
							if !strings.EqualFold(filepath.Ext(path), ".gtr") {
								win.Toast(gui.ToastCfg{Body: i18n.T("terminal.replay_invalid_file"), Severity: gui.ToastError, Duration: 4 * time.Second})
								return
							}
							win.DialogDismiss()
							c.PlayRecording(win, path, p.tabID)
						},
					})
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:       "recording-pick-play",
				Variant:  gui.ButtonPrimary,
				Label:    i18n.T("terminal.replay_play"),
				Disabled: p.selected == "",
				OnClick: func(ctx gui.EventCtx) {
					ctx.Consume()
					ctx.Window.DialogDismiss()
					c.PlayRecording(ctx.Window, p.selected, p.tabID)
				},
			}),
		},
	})))

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingMedium,
		Content: body,
	}))
}

// PlayRecording opens path as a local .gtr replay in targetTabID, converting
// a connect tab into a session tab. It mirrors attachSession's pane wiring but
// skips the hub: there is no live termio.Session, PTY, or SSH/serial state.
func (c *Controller) PlayRecording(w *gui.Window, path, targetTabID string) {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".gtr") {
		w.Toast(gui.ToastCfg{Body: i18n.T("terminal.replay_invalid_file"), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	st := gui.State[State](w)
	sessID := utils.GenerateRandomID()
	target := paneRef{tabID: targetTabID, pane: -1}

	termCfg, termTheme := c.termRuntime()

	var tm *term.Term
	var err error
	tm, err = term.NewReplay(w, term.Cfg{
		NoWindowHandler: true,
		ScrollbackRows:  termCfg.Scrollback,
		TextStyle:       fonts.TermStyle(termCfg),
		Themes:          []term.NamedTheme{termTheme},
		CursorStyle:     cursorStyleOf(termCfg.CursorStyle),
		CursorBlink:     termCfg.CursorBlink,
		CursorLocked:    termCfg.CursorLock,
		// A replay is playback, not a session: never start a recording on it.
		RecordInput: false,
		OnClickFocus: func() {
			if tm == nil {
				return
			}
			c.activatePaneForSess(w, sessID)
			if ids := w.ResolveID(tm.FocusID()); len(ids) > 0 {
				w.SetFocus(ids[0])
			}
		},
	}, term.ReplayCfg{
		Path:      path,
		Speed:     1,
		IdleLimit: defaultReplayIdleLimit,
		Loop:      false,
		Controls:  true,
	})
	if err != nil {
		c.logger.Error("open recording replay", zap.String("path", path), zap.Error(err))
		w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	c.mu.Lock()
	c.terms[sessID] = tm
	c.mu.Unlock()
	tm.SetKeyBindings(c.termKeyMap())

	// Open the replay before converting the tab: a bad .gtr must leave the
	// connect form untouched.
	tab, idx, err := c.resolvePaneTarget(st, target)
	if err != nil {
		c.mu.Lock()
		delete(c.terms, sessID)
		c.mu.Unlock()
		_ = tm.Close()
		w.Toast(gui.ToastCfg{Body: errtext.Text(err), Severity: gui.ToastError, Duration: 4 * time.Second})
		return
	}
	tabID := tab.ID

	tab.Panes[idx] = Pane{
		SessID:   sessID,
		Status:   TabStatusConnected,
		Label:    filepath.Base(path),
		Protocol: termio.ProtocolReplay,
	}
	tab.Kind = TabSession
	tab.Draft = protocol.Draft{}
	tab.ActivePane = idx
	tab.SFTPOpen = false
	st.ActiveTab = tab.ID
	tab.SyncActive()

	c.mu.Lock()
	c.paneLoc[sessID] = paneRef{tabID: tabID, pane: idx}
	c.mu.Unlock()
	c.focusActiveTerm(st)

	w.Toast(gui.ToastCfg{
		Body:     i18n.T("terminal.replay_controls"),
		Severity: gui.ToastSuccess,
		Duration: 4 * time.Second,
	})
}
