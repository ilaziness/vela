package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/agent"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/termio"
	agentmodel "github.com/ilaziness/vela/ui/agent/model"
	"github.com/ilaziness/vela/ui/errtext"
)

// SetAgent injects the agent service during bootstrap.
func (c *Controller) SetAgent(s *agent.Service) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agent = s
}

// AgentReloadConfig hot-swaps the engine after settings save AI config.
func (c *Controller) AgentReloadConfig() {
	svc := c.agentService()
	if svc == nil {
		return
	}
	go func() {
		if err := svc.Reload(); err != nil {
			c.logger.Warn("agent reload config", zap.Error(err))
		}
	}()
}

// AgentServiceReady reports whether the agent backend is wired and AI is
// enabled in settings.
func (c *Controller) AgentServiceReady() bool {
	svc := c.agentService()
	if svc == nil {
		return false
	}
	cfg, err := svc.GetConfig()
	return err == nil && cfg != nil && cfg.Enabled
}

func (c *Controller) agentService() *agent.Service {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.agent
}

// AgentEmit receives every service stream chunk on a Run goroutine and folds
// it into the live transcript on the UI thread.
func (c *Controller) AgentEmit(chunk agent.AIStreamChunkData) {
	w := c.Window()
	if w == nil {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		if st.AgentChat == nil || st.AgentChat.SessionID != chunk.SessionID {
			return
		}
		st.AgentChat.Apply(chunk)
	})
}

// AgentToggle opens or closes the assistant side panel.
func (c *Controller) AgentToggle(w *gui.Window) {
	st := gui.State[State](w)
	st.AgentOpen = !st.AgentOpen
	if st.AgentOpen {
		c.AgentEnsureLoaded(w)
	}
}

// AgentOpenPanel opens the panel (used by deep links / first enable).
func (c *Controller) AgentOpenPanel(w *gui.Window) {
	st := gui.State[State](w)
	if !st.AgentOpen {
		st.AgentOpen = true
	}
	c.AgentEnsureLoaded(w)
}

// AgentEnsureLoaded lazily creates/loads the chat transcript for the selected
// session. Safe to call repeatedly; it no-ops while a load is in flight.
func (c *Controller) AgentEnsureLoaded(w *gui.Window) {
	st := gui.State[State](w)
	svc := c.agentService()
	if svc == nil {
		return
	}
	if st.AgentChat != nil && st.AgentChat.SessionID == st.AgentSessionID && st.AgentSessionID != "" {
		return
	}
	want := st.AgentSessionID
	go func() {
		sessions, err := svc.ListSessions(50)
		if err != nil {
			c.logger.Warn("agent list sessions", zap.Error(err))
		}
		id := want
		found := false
		for _, s := range sessions {
			if s.ID == id {
				found = true
				break
			}
		}
		var sess *agent.AISession
		if !found {
			if len(sessions) > 0 {
				id = sessions[0].ID
			} else {
				s, cerr := svc.CreateSession()
				if cerr != nil {
					c.logger.Error("agent create session", zap.Error(cerr))
					w.QueueCommand(func(w *gui.Window) {
						gui.State[State](w).AgentChat = agentmodel.NewChat("")
					})
					return
				}
				sessions = append([]*agent.AISession{s}, sessions...)
				id = s.ID
				sess = s
			}
		}
		msgs, merr := svc.ListMessages(id)
		if merr != nil {
			c.logger.Warn("agent list messages", zap.Error(merr))
		}
		if sess == nil {
			sess, _ = svc.GetSession(id)
		}
		title := ""
		if sess != nil {
			title = sess.Title
		}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			// A newer selection may have superseded this load.
			if st.AgentChat != nil && st.AgentChat.SessionID != "" && st.AgentChat.SessionID != id && st.AgentSessionID != id {
				return
			}
			st.AgentSessionID = id
			st.AgentSessions = sessions
			chat := agentmodel.NewChat(id)
			chat.SessionTitle = title
			if len(msgs) > 0 {
				chat.Load(msgs)
			}
			st.AgentChat = chat
		})
	}()
}

// AgentSelectSession switches the visible chat to another session.
func (c *Controller) AgentSelectSession(w *gui.Window, id string) {
	st := gui.State[State](w)
	if st.AgentSessionID == id {
		return
	}
	st.AgentSessionID = id
	st.AgentChat = nil
	c.AgentEnsureLoaded(w)
}

// AgentNewSession creates a fresh chat session and selects it.
func (c *Controller) AgentNewSession(w *gui.Window) {
	svc := c.agentService()
	if svc == nil {
		return
	}
	go func() {
		sess, err := svc.CreateSession()
		if err != nil {
			c.logger.Error("agent create session", zap.Error(err))
			return
		}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			st.AgentSessions = append([]*agent.AISession{sess}, st.AgentSessions...)
			st.AgentSessionID = sess.ID
			chat := agentmodel.NewChat(sess.ID)
			chat.SessionTitle = sess.Title
			st.AgentChat = chat
		})
	}()
}

// AgentDeleteSession removes a chat session; the current one is replaced by a
// fresh session so the panel never points at a deleted id.
func (c *Controller) AgentDeleteSession(w *gui.Window, id string) {
	svc := c.agentService()
	if svc == nil || id == "" {
		return
	}
	go func() {
		if err := svc.DeleteSession(id); err != nil {
			c.logger.Warn("agent delete session", zap.Error(err))
		}
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			kept := st.AgentSessions[:0]
			for _, s := range st.AgentSessions {
				if s.ID != id {
					kept = append(kept, s)
				}
			}
			st.AgentSessions = kept
			if st.AgentSessionID == id {
				st.AgentSessionID = ""
				st.AgentChat = nil
				c.AgentEnsureLoaded(w)
			}
		})
	}()
}

// AgentSend submits the composer text. The run streams back through AgentEmit.
func (c *Controller) AgentSend(text string) {
	w := c.Window()
	if w == nil {
		return
	}
	st := gui.State[State](w)
	chat := st.AgentChat
	svc := c.agentService()
	if chat == nil || chat.SessionID == "" || chat.Running || svc == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	sshCtx := c.agentBind(st)
	chat.AppendUser(text)
	chat.BeginRun("")
	go func() {
		_, err := svc.Chat(&agent.ChatRequest{
			SessionID:  chat.SessionID,
			NewMessage: text,
			SSHContext: sshCtx,
		})
		cancelled := errors.Is(err, agent.ErrGenerationStopped)
		w.QueueCommand(func(w *gui.Window) {
			st := gui.State[State](w)
			if st.AgentChat != chat {
				return
			}
			chat.EndRun(cancelled)
			if err != nil && !cancelled {
				c.toast(gui.ToastError, i18n.T("ai.chat.load_failed"), errtext.Text(err))
			}
			c.agentRefreshSessions(w)
		})
	}()
}

// AgentStop cancels the in-flight generation for the current session.
func (c *Controller) AgentStop() {
	w := c.Window()
	if w == nil {
		return
	}
	st := gui.State[State](w)
	chat := st.AgentChat
	svc := c.agentService()
	if chat == nil || !chat.Running || svc == nil {
		return
	}
	if err := svc.StopGeneration(chat.SessionID); err != nil {
		c.logger.Debug("agent stop", zap.Error(err))
	}
}

// AgentRespondApproval answers a pending tool approval (live or cold resume).
func (c *Controller) AgentRespondApproval(approvalID string, approved bool, reason string) {
	w := c.Window()
	if w == nil {
		return
	}
	st := gui.State[State](w)
	chat := st.AgentChat
	svc := c.agentService()
	if chat == nil || approvalID == "" || svc == nil {
		return
	}
	sshCtx := c.agentBind(st)
	// Live approvals resolve a channel; cold resume runs the whole
	// generation synchronously, so never call this on the UI thread.
	go func() {
		if err := svc.RespondToolApproval(&agent.ToolApprovalRequest{
			SessionID:  chat.SessionID,
			ApprovalID: approvalID,
			Approved:   approved,
			Reason:     reason,
			SSHContext: sshCtx,
		}); err != nil {
			c.logger.Warn("agent approval respond", zap.Error(err))
		}
	}()
	var toolCallID string
	if ap := chat.Approval(approvalID); ap != nil {
		toolCallID = ap.ToolCallID
	}
	w.QueueCommand(func(w *gui.Window) {
		st := gui.State[State](w)
		if st.AgentChat != nil {
			st.AgentChat.RemoveApproval(approvalID)
			if !approved && toolCallID != "" {
				st.AgentChat.SetToolStatus(toolCallID, agentmodel.ToolDenied)
			}
			if approved {
				if toolCallID != "" {
					st.AgentChat.SetToolStatus(toolCallID, agentmodel.ToolRunning)
				}
				// Live runs already stream into a running bubble; cold
				// resume (approval answered after reload) starts one now.
				st.AgentChat.BeginRun("")
			}
		}
	})
}

// agentBind resolves the active terminal tab into an SSHContext for the
// service. Returns nil when no live session tab is active or the protocol is
// not agent-capable.
func (c *Controller) agentBind(st *State) *agent.SSHContext {
	tab := st.Active()
	if tab == nil || tab.Kind != TabSession || tab.SessID == "" {
		return nil
	}
	switch tab.Protocol {
	case termio.ProtocolSSH:
		sess, err := c.sshMgr.GetSession(tab.SessID)
		if err != nil || sess == nil {
			return nil
		}
		ep := sess.Endpoint()
		return &agent.SSHContext{LinkID: tab.SessID, Host: ep.Host, Port: ep.Port, User: ep.User}
	case termio.ProtocolSerial:
		return &agent.SSHContext{LinkID: tab.SessID}
	default:
		return nil
	}
}

// AgentBindInfo describes the terminal the next run will bind to, for the
// panel header. status is "connected" | "connecting" | "" (unbound).
func (c *Controller) AgentBindInfo(st *State) (label, status string, sessID string) {
	tab := st.Active()
	if tab == nil || tab.Kind != TabSession || tab.SessID == "" {
		return "", "", ""
	}
	switch tab.Status {
	case TabStatusConnected:
		status = "connected"
	case TabStatusConnecting:
		status = "connecting"
	default:
		status = ""
	}
	sessID = tab.SessID
	switch tab.Protocol {
	case termio.ProtocolSSH:
		if sess, err := c.sshMgr.GetSession(tab.SessID); err == nil && sess != nil {
			ep := sess.Endpoint()
			user := ep.User
			if user == "" {
				user = "?"
			}
			if ep.Port > 0 {
				label = fmt.Sprintf("%s@%s:%d", user, ep.Host, ep.Port)
			} else {
				label = fmt.Sprintf("%s@%s", user, ep.Host)
			}
		}
	case termio.ProtocolSerial:
		if sess, err := c.serialMgr.GetSession(tab.SessID); err == nil && sess != nil {
			label = fmt.Sprintf("%s @ %d", sess.Device(), sess.BaudRate())
		}
	}
	return label, status, sessID
}

// agentRefreshSessions reloads the picker list after a run (titles update).
func (c *Controller) agentRefreshSessions(w *gui.Window) {
	svc := c.agentService()
	if svc == nil {
		return
	}
	go func() {
		sessions, err := svc.ListSessions(50)
		if err != nil {
			return
		}
		w.QueueCommand(func(w *gui.Window) {
			gui.State[State](w).AgentSessions = sessions
		})
	}()
}
