package database

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
