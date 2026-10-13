package command

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
)

type BuiltinCommand struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

type CategoryCommands struct {
	Category string           `json:"category"`
	Commands []BuiltinCommand `json:"commands"`
}

type UserCommand struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}

type History struct {
	Timestamp int64  `json:"timestamp"`
	Command   string `json:"command"`
}

type Info struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
	IsCustom    bool   `json:"is_custom"`
}

type Service struct {
	logger  *zap.Logger
	db      *database.Database
	builtin map[string][]BuiltinCommand
	sendFn  func(sessionID, cmd string) error
}

func New(logger *zap.Logger, db *database.Database, builtinJSON []byte, sendFn func(sessionID, cmd string) error) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	s := &Service{logger: logger, db: db, sendFn: sendFn, builtin: map[string][]BuiltinCommand{}}
	if err := s.load(builtinJSON); err != nil {
		logger.Error("load builtin commands failed", zap.Error(err))
	}
	return s
}

func (s *Service) load(data []byte) error {
	var categories []CategoryCommands
	if err := json.Unmarshal(data, &categories); err != nil {
		return fmt.Errorf("unmarshal builtin commands: %w", err)
	}
	for _, cat := range categories {
		s.builtin[cat.Category] = cat.Commands
	}
	return nil
}

func (s *Service) UserCommands() []UserCommand {
	dbCommands, err := s.db.UserCommandRepo.GetAllCommands()
	if err != nil {
		s.logger.Error("get user commands failed", zap.Error(err))
		return []UserCommand{}
	}
	out := make([]UserCommand, 0, len(dbCommands))
	for _, cmd := range dbCommands {
		out = append(out, UserCommand{
			Category: cmd.Category, Name: cmd.Name, Command: cmd.Command,
			Description: cmd.Description, CreatedAt: cmd.CreatedAt.UnixNano() / 1000000,
		})
	}
	return out
}

func (s *Service) SaveUser(cmd UserCommand) error {
	existing, err := s.db.UserCommandRepo.GetCommandByCategoryAndName(cmd.Category, cmd.Name)
	cmd.CreatedAt = time.Now().UnixNano() / 1000000
	dbCmd := &database.UserCommandDB{
		Category: cmd.Category, Name: cmd.Name, Command: cmd.Command,
		Description: cmd.Description, CreatedAt: time.Unix(cmd.CreatedAt/1000, 0),
	}
	if err == nil && existing != nil {
		dbCmd.ID = existing.ID
		return s.db.UserCommandRepo.UpdateCommand(dbCmd)
	}
	_, err = s.db.UserCommandRepo.InsertCommand(dbCmd)
	return err
}

func (s *Service) DeleteUser(category, name string) error {
	cmd, err := s.db.UserCommandRepo.GetCommandByCategoryAndName(category, name)
	if err != nil {
		return fmt.Errorf("user command not found: category=%s, name=%s", category, name)
	}
	return s.db.UserCommandRepo.DeleteCommand(cmd.ID)
}

func (s *Service) History() []History {
	rows, err := s.db.CommandHistoryRepo.GetHistory()
	if err != nil {
		s.logger.Error("get command history failed", zap.Error(err))
		return []History{}
	}
	out := make([]History, 0, len(rows))
	for _, h := range rows {
		out = append(out, History{Command: h.Command, Timestamp: h.Timestamp.UnixNano() / 1000000})
	}
	return out
}

func (s *Service) AddHistory(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	latest, err := s.db.CommandHistoryRepo.GetLatestHistory()
	if err != nil {
		s.logger.Error("get latest command history failed", zap.Error(err))
	} else if latest != nil && latest.Command == cmd {
		return nil
	}
	_, err = s.db.CommandHistoryRepo.InsertHistory(&database.CommandHistoryDB{Command: cmd, Timestamp: time.Now()})
	return err
}

func (s *Service) ClearHistory() error {
	return s.db.CommandHistoryRepo.ClearHistory()
}

func (s *Service) Builtin() map[string][]BuiltinCommand {
	return s.builtin
}

func (s *Service) All() map[string][]Info {
	result := make(map[string][]Info)
	for category, cmds := range s.builtin {
		for _, cmd := range cmds {
			result[category] = append(result[category], Info{
				Category: category, Name: cmd.Name, Command: cmd.Command, Description: cmd.Description,
			})
		}
	}
	for _, cmd := range s.UserCommands() {
		replaced := false
		for i, existing := range result[cmd.Category] {
			if existing.Name == cmd.Name {
				result[cmd.Category][i] = Info{Category: cmd.Category, Name: cmd.Name, Command: cmd.Command, Description: cmd.Description, IsCustom: true}
				replaced = true
				break
			}
		}
		if !replaced {
			result[cmd.Category] = append(result[cmd.Category], Info{Category: cmd.Category, Name: cmd.Name, Command: cmd.Command, Description: cmd.Description, IsCustom: true})
		}
	}
	return result
}

func (s *Service) Send(command string, sessionIDs []string) error {
	if s.sendFn == nil {
		return fmt.Errorf("SSH service not initialized")
	}
	var lastErr error
	for _, id := range sessionIDs {
		if err := s.sendFn(id, command); err != nil {
			s.logger.Error("send command failed", zap.String("session", id), zap.Error(err))
			lastErr = err
		}
	}
	if lastErr == nil {
		_ = s.AddHistory(command)
	}
	return lastErr
}
