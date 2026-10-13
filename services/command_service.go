package services

import (
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

type CommandService struct {
	windows *Windows
	core    *command.Service
}

func NewCommandService(windows *Windows, core *command.Service) *CommandService {
	return &CommandService{windows: windows, core: core}
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
func (cs *CommandService) GetBuiltinCommands() map[string][]BuiltinCommand {
	return cs.core.Builtin()
}
func (cs *CommandService) GetAllCommands() map[string][]CommandInfo { return cs.core.All() }
func (cs *CommandService) SendCommand(req SendCommandRequest) error {
	return cs.core.Send(req.Command, req.SessionIDs)
}
func (cs *CommandService) ShowWindow()  { cs.windows.ShowCommand() }
func (cs *CommandService) CloseWindow() { cs.windows.CloseCommand() }
