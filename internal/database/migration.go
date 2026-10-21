package database

import (
	"database/sql"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// Migration 表示一个数据库迁移
type Migration struct {
	Version int
	Name    string
	Up      func(*sql.DB, *zap.Logger) error
}

// migrations 注册所有数据库迁移，按版本号升序排列
var migrations = []Migration{
	{Version: 1, Name: "init schema", Up: migrateInitSchema},
	{Version: 2, Name: "add proxy_jump_id", Up: migrateAddProxyJumpID},
	{Version: 3, Name: "add ai sessions", Up: migrateAddAISessions},
	{Version: 4, Name: "add bookmark icon", Up: migrateAddBookmarkIcon},
	{Version: 5, Name: "add bookmark use_agent", Up: migrateAddBookmarkUseAgent},
	{Version: 6, Name: "add ssh keys", Up: migrateAddSSHKeys},
	{Version: 7, Name: "add bookmark certificate", Up: migrateAddBookmarkCertificate},
	{Version: 8, Name: "add bookmark forward_agent", Up: migrateAddBookmarkForwardAgent},
	{Version: 9, Name: "add bookmark dial proxy", Up: migrateAddBookmarkDialProxy},
	{Version: 10, Name: "add bookmark session options", Up: migrateAddBookmarkSessionOptions},
	{Version: 11, Name: "add sftp transfers queue", Up: migrateAddSftpTransfers},
	{Version: 12, Name: "add bookmark tags and color", Up: migrateAddBookmarkTagsColor},
	{Version: 13, Name: "add ai_messages status", Up: migrateAddAIMessageStatus},
	{Version: 14, Name: "add sftp transfer overwrite", Up: migrateAddSftpTransferOverwrite},
	{Version: 15, Name: "add bookmark charset", Up: migrateAddBookmarkCharset},
}

// migrateInitSchema 初始化数据库表结构（幂等）
func migrateInitSchema(db *sql.DB, logger *zap.Logger) error {
	createGroupTable := `
	CREATE TABLE IF NOT EXISTS bookmark_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	createBookmarkTable := `
	CREATE TABLE IF NOT EXISTS bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bookmark_id TEXT UNIQUE NOT NULL,
		group_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		user TEXT,
		password TEXT,
		private_key TEXT,
		private_key_password TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (group_id) REFERENCES bookmark_groups(id) ON DELETE CASCADE
	);`

	createUserCommandTable := `
	CREATE TABLE IF NOT EXISTS user_commands (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT NOT NULL,
		name TEXT NOT NULL,
		command TEXT NOT NULL,
		description TEXT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	createCommandHistoryTable := `
	CREATE TABLE IF NOT EXISTS command_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		command TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	createIndexes := `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_bookmark_id ON bookmarks(bookmark_id);
	CREATE INDEX IF NOT EXISTS idx_bookmarks_group_id ON bookmarks(group_id);
	CREATE INDEX IF NOT EXISTS idx_user_commands_category ON user_commands(category);
	CREATE INDEX IF NOT EXISTS idx_command_history_timestamp ON command_history(timestamp DESC);
	`

	tables := []string{
		createGroupTable,
		createBookmarkTable,
		createUserCommandTable,
		createCommandHistoryTable,
		createIndexes,
	}

	for _, sql := range tables {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("exec sql failed: %w", err)
		}
	}

	logger.Debug("migration: init schema applied")
	return nil
}

// migrateAddProxyJumpID 添加 proxy_jump_id 列（幂等）
func migrateAddProxyJumpID(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'proxy_jump_id'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		_, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN proxy_jump_id TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("add column proxy_jump_id failed: %w", err)
		}
		logger.Debug("migration: added proxy_jump_id column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column proxy_jump_id failed: %w", err)
	}
	return nil
}

// migrateAddAISessions 添加 AI 会话和消息表（幂等）
func migrateAddAISessions(db *sql.DB, logger *zap.Logger) error {
	createSessionsTable := `
	CREATE TABLE IF NOT EXISTS ai_sessions (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`

	createMessagesTable := `
	CREATE TABLE IF NOT EXISTS ai_messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		parts TEXT DEFAULT '',
		timestamp INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE CASCADE
	);`

	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_ai_messages_session_id ON ai_messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_ai_sessions_updated_at ON ai_sessions(updated_at DESC);
	`

	sqls := []string{
		createSessionsTable,
		createMessagesTable,
		createIndexes,
	}

	for _, sql := range sqls {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("exec sql failed: %w", err)
		}
	}

	logger.Debug("migration: added ai_sessions and ai_messages tables")
	return nil
}

// migrateAddBookmarkIcon 为书签和分组添加 icon 列（幂等）
func migrateAddBookmarkIcon(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'icon'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN icon TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("add column bookmarks.icon failed: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check column bookmarks.icon failed: %w", err)
	}

	err = db.QueryRow(`SELECT name FROM pragma_table_info('bookmark_groups') WHERE name = 'icon'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmark_groups ADD COLUMN icon TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("add column bookmark_groups.icon failed: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check column bookmark_groups.icon failed: %w", err)
	}

	logger.Debug("migration: added bookmark/group icon columns")
	return nil
}

// migrateAddBookmarkUseAgent 为书签增加 SSH Agent 认证开关（幂等）
func migrateAddBookmarkUseAgent(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'use_agent'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN use_agent INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add column bookmarks.use_agent failed: %w", err)
		}
		logger.Debug("migration: added bookmarks.use_agent column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column bookmarks.use_agent failed: %w", err)
	}
	return nil
}

// migrateAddSSHKeys 增加应用内密钥表，以及书签对密钥的引用。
func migrateAddSSHKeys(db *sql.DB, logger *zap.Logger) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		algorithm TEXT NOT NULL,
		comment TEXT NOT NULL DEFAULT '',
		public_key TEXT NOT NULL,
		private_key TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return fmt.Errorf("create ssh_keys failed: %w", err)
	}

	var columnName string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'ssh_key_id'`).Scan(&columnName)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN ssh_key_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add column bookmarks.ssh_key_id failed: %w", err)
		}
		logger.Debug("migration: added bookmarks.ssh_key_id column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column bookmarks.ssh_key_id failed: %w", err)
	}
	return nil
}

// migrateAddBookmarkCertificate 为书签增加 OpenSSH 用户证书文件路径（幂等）。
func migrateAddBookmarkCertificate(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'certificate'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN certificate TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add column bookmarks.certificate failed: %w", err)
		}
		logger.Debug("migration: added bookmarks.certificate column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column bookmarks.certificate failed: %w", err)
	}
	return nil
}

// migrateAddBookmarkForwardAgent 为书签增加 SSH Agent 转发开关（幂等）。
func migrateAddBookmarkForwardAgent(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'forward_agent'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN forward_agent INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add column bookmarks.forward_agent failed: %w", err)
		}
		logger.Debug("migration: added bookmarks.forward_agent column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column bookmarks.forward_agent failed: %w", err)
	}
	return nil
}

// migrateAddBookmarkDialProxy 为书签增加拨号代理字段（幂等）。
func migrateAddBookmarkDialProxy(db *sql.DB, logger *zap.Logger) error {
	cols := []struct {
		name string
		ddl  string
	}{
		{"proxy_mode", `ALTER TABLE bookmarks ADD COLUMN proxy_mode TEXT NOT NULL DEFAULT 'inherit'`},
		{"proxy_type", `ALTER TABLE bookmarks ADD COLUMN proxy_type TEXT NOT NULL DEFAULT ''`},
		{"proxy_host", `ALTER TABLE bookmarks ADD COLUMN proxy_host TEXT NOT NULL DEFAULT ''`},
		{"proxy_port", `ALTER TABLE bookmarks ADD COLUMN proxy_port INTEGER NOT NULL DEFAULT 0`},
		{"proxy_user", `ALTER TABLE bookmarks ADD COLUMN proxy_user TEXT NOT NULL DEFAULT ''`},
		{"proxy_password", `ALTER TABLE bookmarks ADD COLUMN proxy_password TEXT NOT NULL DEFAULT ''`},
	}
	for _, c := range cols {
		var columnName string
		err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = ?`, c.name).Scan(&columnName)
		if err == sql.ErrNoRows {
			if _, err := db.Exec(c.ddl); err != nil {
				return fmt.Errorf("add column bookmarks.%s failed: %w", c.name, err)
			}
			logger.Debug("migration: added bookmarks column", zap.String("column", c.name))
			continue
		}
		if err != nil {
			return fmt.Errorf("check column bookmarks.%s failed: %w", c.name, err)
		}
	}
	return nil
}

// migrateAddBookmarkSessionOptions 为书签增加启动命令、环境变量与 TERM（幂等）。
func migrateAddBookmarkSessionOptions(db *sql.DB, logger *zap.Logger) error {
	cols := []struct {
		name string
		ddl  string
	}{
		{"startup_cmd", `ALTER TABLE bookmarks ADD COLUMN startup_cmd TEXT NOT NULL DEFAULT ''`},
		{"env_vars", `ALTER TABLE bookmarks ADD COLUMN env_vars TEXT NOT NULL DEFAULT ''`},
		{"term", `ALTER TABLE bookmarks ADD COLUMN term TEXT NOT NULL DEFAULT ''`},
	}
	for _, c := range cols {
		var columnName string
		err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = ?`, c.name).Scan(&columnName)
		if err == sql.ErrNoRows {
			if _, err := db.Exec(c.ddl); err != nil {
				return fmt.Errorf("add column bookmarks.%s failed: %w", c.name, err)
			}
			logger.Debug("migration: added bookmarks column", zap.String("column", c.name))
			continue
		}
		if err != nil {
			return fmt.Errorf("check column bookmarks.%s failed: %w", c.name, err)
		}
	}
	return nil
}

func migrateAddSftpTransfers(db *sql.DB, logger *zap.Logger) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sftp_transfers (
			id TEXT PRIMARY KEY NOT NULL,
			owner_key TEXT NOT NULL,
			transfer_type TEXT NOT NULL,
			local_file TEXT NOT NULL,
			remote_file TEXT NOT NULL,
			total_size INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sftp_transfers_owner_key ON sftp_transfers(owner_key);
	`)
	if err != nil {
		return fmt.Errorf("create sftp_transfers failed: %w", err)
	}
	logger.Debug("migration: sftp_transfers table ready")
	return nil
}

func migrateAddBookmarkTagsColor(db *sql.DB, logger *zap.Logger) error {
	cols := []struct {
		name string
		ddl  string
	}{
		{"tags", `ALTER TABLE bookmarks ADD COLUMN tags TEXT NOT NULL DEFAULT ''`},
		{"color", `ALTER TABLE bookmarks ADD COLUMN color TEXT NOT NULL DEFAULT ''`},
	}
	for _, c := range cols {
		var columnName string
		err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = ?`, c.name).Scan(&columnName)
		if err == sql.ErrNoRows {
			if _, err := db.Exec(c.ddl); err != nil {
				return fmt.Errorf("add column bookmarks.%s failed: %w", c.name, err)
			}
			logger.Debug("migration: added bookmarks column", zap.String("column", c.name))
			continue
		}
		if err != nil {
			return fmt.Errorf("check column bookmarks.%s failed: %w", c.name, err)
		}
	}
	return nil
}

func migrateAddSftpTransferOverwrite(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('sftp_transfers') WHERE name = 'overwrite'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE sftp_transfers ADD COLUMN overwrite INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add column sftp_transfers.overwrite failed: %w", err)
		}
		logger.Debug("migration: added sftp_transfers.overwrite column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column sftp_transfers.overwrite failed: %w", err)
	}
	return nil
}

// migrateAddBookmarkCharset adds per-bookmark terminal charset (idempotent).
func migrateAddBookmarkCharset(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('bookmarks') WHERE name = 'charset'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE bookmarks ADD COLUMN charset TEXT NOT NULL DEFAULT 'utf-8'`); err != nil {
			return fmt.Errorf("add column bookmarks.charset failed: %w", err)
		}
		logger.Debug("migration: added bookmarks.charset column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column bookmarks.charset failed: %w", err)
	}
	return nil
}

// migrateAddAIMessageStatus adds ui status for cancel/deny replay (idempotent).
func migrateAddAIMessageStatus(db *sql.DB, logger *zap.Logger) error {
	var columnName string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('ai_messages') WHERE name = 'status'`).Scan(&columnName)
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`ALTER TABLE ai_messages ADD COLUMN status TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add column ai_messages.status failed: %w", err)
		}
		logger.Debug("migration: added ai_messages.status column")
		return nil
	}
	if err != nil {
		return fmt.Errorf("check column ai_messages.status failed: %w", err)
	}
	return nil
}

// createSchemaMigrationsTable 创建迁移记录表
func createSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table failed: %w", err)
	}
	return nil
}

// getAppliedMigrations 获取已执行的迁移版本
func getAppliedMigrations(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations failed: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version failed: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration versions failed: %w", err)
	}

	return applied, nil
}

// runMigrations 执行所有未执行的迁移
func runMigrations(db *sql.DB, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	if err := createSchemaMigrationsTable(db); err != nil {
		return err
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		logger.Debug("running migration", zap.Int("version", m.Version), zap.String("name", m.Name))

		if err := m.Up(db, logger); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}

		_, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.Version)
		if err != nil {
			return fmt.Errorf("record migration %d failed: %w", m.Version, err)
		}

		logger.Debug("migration completed", zap.Int("version", m.Version), zap.String("name", m.Name))
	}

	return nil
}
