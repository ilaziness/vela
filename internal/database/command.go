package database

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// UserCommandDB 用户命令数据库模型
type UserCommandDB struct {
	ID          int       `json:"id"`
	Category    string    `json:"category"`
	Name        string    `json:"name"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// CommandHistoryDB 命令历史数据库模型
type CommandHistoryDB struct {
	ID        int       `json:"id"`
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
}

// UserCommandRepository 用户命令数据访问接口
type UserCommandRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewUserCommandRepository(db *sql.DB, logger *zap.Logger) *UserCommandRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &UserCommandRepository{db: db, logger: logger}
}

// GetAllCommands 获取所有用户命令
func (r *UserCommandRepository) GetAllCommands() ([]*UserCommandDB, error) {
	query := `SELECT id, category, name, command, description, created_at FROM user_commands ORDER BY category, name`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf(errQuery, tableNameUserCommands, err)
	}
	defer rows.Close()

	commands := make([]*UserCommandDB, 0)
	for rows.Next() {
		var cmd UserCommandDB
		if err := rows.Scan(&cmd.ID, &cmd.Category, &cmd.Name, &cmd.Command, &cmd.Description, &cmd.CreatedAt); err != nil {
			r.logger.Error("scan user command failed", zap.Error(err))
			continue
		}
		commands = append(commands, &cmd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(errQuery, tableNameUserCommands, err)
	}

	return commands, nil
}

// SaveCommands 保存用户命令（全量覆盖，保留用于批量导入场景）
func (r *UserCommandRepository) SaveCommands(commands []*UserCommandDB) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf(errBeginTx, err)
	}
	defer func() { _ = tx.Rollback() }()

	// 清空现有数据
	if _, err := tx.Exec(`DELETE FROM user_commands`); err != nil {
		return fmt.Errorf(errDeleteQuery, tableNameUserCommands, err)
	}

	// 插入新数据
	for _, cmd := range commands {
		_, err := tx.Exec(
			`INSERT INTO user_commands (category, name, command, description, created_at) VALUES (?, ?, ?, ?, ?)`,
			cmd.Category, cmd.Name, cmd.Command, cmd.Description, cmd.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf(errInsertQuery, errTableNameUserCommand, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(errCommitTx, err)
	}

	r.logger.Debug("user commands saved to db")
	return nil
}

// InsertCommand 插入用户命令
func (r *UserCommandRepository) InsertCommand(command *UserCommandDB) (int64, error) {
	query := `INSERT INTO user_commands (category, name, command, description, created_at) VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.Exec(query, command.Category, command.Name, command.Command, command.Description, command.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf(errInsertQuery, errTableNameUserCommand, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id failed: %w", err)
	}

	r.logger.Debug("user command inserted", zap.String("name", command.Name))
	return id, nil
}

// UpdateCommand 更新用户命令
func (r *UserCommandRepository) UpdateCommand(command *UserCommandDB) error {
	query := `UPDATE user_commands SET category = ?, name = ?, command = ?, description = ? WHERE id = ?`
	_, err := r.db.Exec(query, command.Category, command.Name, command.Command, command.Description, command.ID)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "update user command", err)
	}

	r.logger.Debug("user command updated", zap.String("name", command.Name))
	return nil
}

// DeleteCommand 删除用户命令
func (r *UserCommandRepository) DeleteCommand(id int) error {
	_, err := r.db.Exec(`DELETE FROM user_commands WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf(errDeleteQuery, errTableNameUserCommand, err)
	}

	r.logger.Debug("user command deleted", zap.Int("id", id))
	return nil
}

// GetCommandByCategoryAndName 按分类和名称查询命令
func (r *UserCommandRepository) GetCommandByCategoryAndName(category, name string) (*UserCommandDB, error) {
	query := `SELECT id, category, name, command, description, created_at FROM user_commands WHERE category = ? AND name = ?`
	row := r.db.QueryRow(query, category, name)

	var cmd UserCommandDB
	err := row.Scan(&cmd.ID, &cmd.Category, &cmd.Name, &cmd.Command, &cmd.Description, &cmd.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found")
		}
		return nil, fmt.Errorf(errQuery, "command by category and name", err)
	}

	return &cmd, nil
}

// CommandHistoryRepository 命令历史数据访问接口
type CommandHistoryRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewCommandHistoryRepository(db *sql.DB, logger *zap.Logger) *CommandHistoryRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CommandHistoryRepository{db: db, logger: logger}
}

// GetLatestHistory returns the most recent history row, or nil if empty.
func (r *CommandHistoryRepository) GetLatestHistory() (*CommandHistoryDB, error) {
	query := `SELECT id, command, timestamp FROM command_history ORDER BY timestamp DESC LIMIT 1`
	row := r.db.QueryRow(query)
	var h CommandHistoryDB
	err := row.Scan(&h.ID, &h.Command, &h.Timestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf(errQuery, tableNameCommandHistory, err)
	}
	return &h, nil
}

// GetHistory 获取命令历史（最新 100 条）
func (r *CommandHistoryRepository) GetHistory() ([]*CommandHistoryDB, error) {
	query := `SELECT id, command, timestamp FROM command_history ORDER BY timestamp DESC LIMIT 100`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf(errQuery, tableNameCommandHistory, err)
	}
	defer rows.Close()

	history := make([]*CommandHistoryDB, 0)
	for rows.Next() {
		var h CommandHistoryDB
		if err := rows.Scan(&h.ID, &h.Command, &h.Timestamp); err != nil {
			r.logger.Error("scan command history failed", zap.Error(err))
			continue
		}
		history = append(history, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(errQuery, tableNameCommandHistory, err)
	}

	return history, nil
}

// SaveHistory 保存命令历史（全量覆盖，保留用于批量导入场景）
func (r *CommandHistoryRepository) SaveHistory(history []*CommandHistoryDB) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf(errBeginTx, err)
	}
	defer func() { _ = tx.Rollback() }()

	// 清空现有数据
	if _, err := tx.Exec(`DELETE FROM command_history`); err != nil {
		return fmt.Errorf(errDeleteQuery, tableNameCommandHistory, err)
	}

	// 插入新数据
	for _, h := range history {
		_, err := tx.Exec(
			`INSERT INTO command_history (command, timestamp) VALUES (?, ?)`,
			h.Command, h.Timestamp,
		)
		if err != nil {
			return fmt.Errorf(errInsertQuery, tableNameCommandHistory, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(errCommitTx, err)
	}

	r.logger.Debug("command history saved to db")
	return nil
}

// InsertHistory 插入命令历史
func (r *CommandHistoryRepository) InsertHistory(history *CommandHistoryDB) (int64, error) {
	query := `INSERT INTO command_history (command, timestamp) VALUES (?, ?)`
	result, err := r.db.Exec(query, history.Command, history.Timestamp)
	if err != nil {
		return 0, fmt.Errorf(errInsertQuery, "command history", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id failed: %w", err)
	}

	r.logger.Debug("command history inserted", zap.Int64("id", id))
	return id, nil
}

// ClearHistory 清空命令历史
func (r *CommandHistoryRepository) ClearHistory() error {
	_, err := r.db.Exec(`DELETE FROM command_history`)
	if err != nil {
		return fmt.Errorf(errDeleteQuery, tableNameCommandHistory, err)
	}

	r.logger.Debug("command history cleared")
	return nil
}
