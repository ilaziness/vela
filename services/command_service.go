package services

import (
	"strings"

	"github.com/ilaziness/vela/internal/command"
)

type BuiltinCommand = command.BuiltinCommand
type UserCommand = command.UserCommand
type CommandHistory = command.History
type CommandInfo = command.Info

type SendCommandRequest struct {
	Command    string   `json:"command"`
	SessionIDs []string `json:"session_ids"`
}

// historyGate decides whether a terminal submit may be stored (password / TUI).
type historyGate interface {
	AcceptHistory(sessionID, cmd string) bool
}

type CommandService struct {
	windows *Windows
	core    *command.Service
	gate    historyGate
}

func NewCommandService(windows *Windows, core *command.Service, gate historyGate) *CommandService {
	return &CommandService{windows: windows, core: core, gate: gate}
}

func (cs *CommandService) GetUserCommands() []UserCommand { return cs.core.UserCommands() }
func (cs *CommandService) SaveUserCommand(cmd UserCommand) error {
	return cs.core.SaveUser(cmd)
}
func (cs *CommandService) DeleteUserCommand(category, name string) error {
	return cs.core.DeleteUser(category, name)
}
func (cs *CommandService) GetCommandHistory() []CommandHistory { return cs.core.History() }
func (cs *CommandService) AddCommandHistory(c string) error    { return cs.core.AddHistory(c) }
func (cs *CommandService) ClearCommandHistory() error          { return cs.core.ClearHistory() }

// RecordShellHistory stores a line submitted from the terminal UI after gate checks.
func (cs *CommandService) RecordShellHistory(sessionID, cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || !cs.gate.AcceptHistory(sessionID, cmd) {
		return nil
	}
	return cs.core.AddHistory(cmd)
}

func (cs *CommandService) GetBuiltinCommands() map[string][]BuiltinCommand {
	return cs.core.Builtin()
}
func (cs *CommandService) GetAllCommands() map[string][]CommandInfo { return cs.core.All() }
func (cs *CommandService) SendCommand(req SendCommandRequest) error {
	return cs.core.Send(req.Command, req.SessionIDs)
}
func (cs *CommandService) ShowWindow()  { cs.windows.ShowCommand() }
func (cs *CommandService) CloseWindow() { cs.windows.CloseCommand() }
