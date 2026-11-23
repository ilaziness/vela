package database

import (
	"database/sql"
	"database/sql/driver"
	"errors"

	"modernc.org/sqlite"
)

const (
	errBeginTx     = "begin tx failed: %w"
	errCommitTx    = "commit tx failed: %w"
	errDeleteQuery = "delete %s failed: %w"
	errInsertQuery = "insert %s failed: %w"
	errQuery       = "query %s failed: %w"
	errUpdateQuery = "update %s failed: %w"

	tableNameBookmarks      = "bookmarks"
	tableNameBookmarkGroups = "bookmark_groups"
	tableNameUserCommands   = "user commands"
	tableNameCommandHistory = "command history"

	errTableNameUserCommand = "user command"
)

// IsNotFound reports whether err indicates a missing row. Repositories
// return sql.ErrNoRows for absent records; callers map it to their domain
// not-found error instead of comparing message strings.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// IsStorageError reports whether err carries a storage-layer failure
// (SQLite driver error or database/sql plumbing) anywhere in its Unwrap
// chain. The UI uses it to replace driver details with a generic message.
func IsStorageError(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return true
	}
	return IsNotFound(err) ||
		errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, driver.ErrBadConn)
}
