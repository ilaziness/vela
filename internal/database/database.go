package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	_ "modernc.org/sqlite" // SQLite driver
)

var dbFileName = "vela.db"

// legacyDBFileName is the pre-rename database filename; migrated once to vela.db.
const legacyDBFileName = "vexo.db"

type Database struct {
	db                 *sql.DB
	dbPath             string
	userDataDir        string
	logger             *zap.Logger
	BookmarkRepo       *BookmarkRepository
	SSHKeyRepo         *SSHKeyRepository
	UserCommandRepo    *UserCommandRepository
	CommandHistoryRepo *CommandHistoryRepository
	AISessionRepo      AISessionRepository
	SftpTransferRepo   *SftpTransferRepository
}

func NewDatabase(userDataDir string, logger *zap.Logger) *Database {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Database{
		userDataDir: userDataDir,
		dbPath:      filepath.Join(userDataDir, dbFileName),
		logger:      logger,
	}
}

// resolveDBPath prefers vela.db; if only the pre-rename vexo.db exists, rename it.
// On rename failure (or unusable destination), keep using the legacy file so
// Initialize never opens an empty DB while old data still exists.
func resolveDBPath(userDataDir string, logger *zap.Logger) string {
	dbPath := filepath.Join(userDataDir, dbFileName)
	legacy := filepath.Join(userDataDir, legacyDBFileName)

	dbInfo, dbErr := os.Stat(dbPath)
	dbOK := dbErr == nil && !dbInfo.IsDir()
	legInfo, legErr := os.Stat(legacy)
	legOK := legErr == nil && !legInfo.IsDir()

	if dbOK && legOK {
		// Sync restore of an old archive can leave vexo.db next to a fresh empty vela.db.
		if dbInfo.Size() == 0 && legInfo.Size() > 0 {
			if err := os.Remove(dbPath); err != nil {
				logger.Warn("remove empty vela.db failed; using legacy path",
					zap.String("path", dbPath), zap.Error(err))
				return legacy
			}
			dbOK = false
		} else {
			logger.Warn("both vela.db and legacy vexo.db present; using vela.db",
				zap.String("vela", dbPath), zap.String("legacy", legacy))
			return dbPath
		}
	}
	if dbOK || !legOK {
		return dbPath
	}

	if err := os.Rename(legacy, dbPath); err != nil {
		logger.Warn("migrate legacy database failed; using legacy path",
			zap.String("from", legacy), zap.String("to", dbPath), zap.Error(err))
		return legacy
	}
	logger.Info("migrated legacy database", zap.String("from", legacy), zap.String("to", dbPath))
	return dbPath
}

func (d *Database) Initialize() error {
	d.dbPath = resolveDBPath(d.userDataDir, d.logger)
	var err error
	d.db, err = sql.Open("sqlite", d.dbPath)
	if err != nil {
		return fmt.Errorf("open db failed: %w", err)
	}
	d.db.SetMaxOpenConns(25)
	d.db.SetMaxIdleConns(5)
	if err := d.db.Ping(); err != nil {
		return fmt.Errorf("ping db failed: %w", err)
	}
	d.logger.Debug("db opened successfully", zap.String("dbPath", d.dbPath))
	if err := runMigrations(d.db, d.logger); err != nil {
		return fmt.Errorf("run migrations failed: %w", err)
	}
	d.logger.Debug("db initialized successfully")
	d.BookmarkRepo = NewBookmarkRepository(d.db, d.logger)
	d.SSHKeyRepo = NewSSHKeyRepository(d.db, d.logger)
	d.UserCommandRepo = NewUserCommandRepository(d.db, d.logger)
	d.CommandHistoryRepo = NewCommandHistoryRepository(d.db, d.logger)
	d.AISessionRepo = NewSQLiteAISessionRepository(d.db)
	d.SftpTransferRepo = NewSftpTransferRepository(d.db, d.logger)
	return nil
}

func (d *Database) Close() error {
	if d.db != nil {
		if err := d.db.Close(); err != nil {
			return fmt.Errorf("close db failed: %w", err)
		}
		d.db = nil
		d.logger.Debug("db closed successfully")
	}
	return nil
}
