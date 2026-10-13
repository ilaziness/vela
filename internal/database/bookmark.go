package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

const bookmarkSelectCols = `id, bookmark_id, group_id, title, host, port, user, password, private_key, private_key_password, proxy_jump_id, COALESCE(icon, ''), COALESCE(use_agent, 0), COALESCE(ssh_key_id, ''), COALESCE(certificate, ''), COALESCE(forward_agent, 0), COALESCE(proxy_mode, 'inherit'), COALESCE(proxy_type, ''), COALESCE(proxy_host, ''), COALESCE(proxy_port, 0), COALESCE(proxy_user, ''), COALESCE(proxy_password, ''), COALESCE(startup_cmd, ''), COALESCE(env_vars, ''), COALESCE(term, ''), COALESCE(tags, ''), COALESCE(color, ''), created_at, updated_at`

// BookmarkGroupDB 书签分组数据库模型
type BookmarkGroupDB struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"created_at"`
}

// BookmarkDB 书签数据库模型
type BookmarkDB struct {
	ID                 string    `json:"id"` // 字符串业务 ID
	GroupID            int       `json:"group_id"`
	Title              string    `json:"title"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	User               string    `json:"user"`
	Password           string    `json:"password"`
	PrivateKey         string    `json:"private_key"`
	PrivateKeyPassword string    `json:"private_key_password"`
	ProxyJumpID        string    `json:"proxy_jump_id"`
	Icon               string    `json:"icon"`
	UseAgent           bool      `json:"use_agent"`
	SshKeyID           string    `json:"ssh_key_id"`
	Certificate        string    `json:"certificate"`
	ForwardAgent       bool      `json:"forward_agent"`
	ProxyMode          string    `json:"proxy_mode"`
	ProxyType          string    `json:"proxy_type"`
	ProxyHost          string    `json:"proxy_host"`
	ProxyPort          int       `json:"proxy_port"`
	ProxyUser          string    `json:"proxy_user"`
	ProxyPassword      string    `json:"proxy_password"`
	StartupCmd         string    `json:"startup_cmd"`
	EnvVars            string    `json:"env_vars"`
	Term               string    `json:"term"`
	Tags               string    `json:"tags"`
	Color              string    `json:"color"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// BookmarkRepository 书签数据访问接口
type BookmarkRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewBookmarkRepository(db *sql.DB, logger *zap.Logger) *BookmarkRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BookmarkRepository{db: db, logger: logger}
}

func scanBookmark(scanner interface{ Scan(dest ...any) error }) (*BookmarkDB, error) {
	var b BookmarkDB
	var autoID int
	var useAgent int
	var forwardAgent int
	err := scanner.Scan(
		&autoID, &b.ID, &b.GroupID, &b.Title, &b.Host, &b.Port,
		&b.User, &b.Password, &b.PrivateKey, &b.PrivateKeyPassword, &b.ProxyJumpID, &b.Icon,
		&useAgent, &b.SshKeyID, &b.Certificate, &forwardAgent,
		&b.ProxyMode, &b.ProxyType, &b.ProxyHost, &b.ProxyPort, &b.ProxyUser, &b.ProxyPassword,
		&b.StartupCmd, &b.EnvVars, &b.Term, &b.Tags, &b.Color,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.UseAgent = useAgent != 0
	b.ForwardAgent = forwardAgent != 0
	if b.ProxyMode == "" {
		b.ProxyMode = "inherit"
	}
	return &b, nil
}

// GetAllGroups 获取所有分组
func (r *BookmarkRepository) GetAllGroups() ([]*BookmarkGroupDB, error) {
	query := `SELECT id, name, COALESCE(icon, ''), created_at FROM bookmark_groups ORDER BY id`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf(errQuery, tableNameBookmarkGroups, err)
	}
	defer rows.Close()

	groups := make([]*BookmarkGroupDB, 0)
	for rows.Next() {
		var g BookmarkGroupDB
		if err := rows.Scan(&g.ID, &g.Name, &g.Icon, &g.CreatedAt); err != nil {
			r.logger.Error("scan group failed", zap.Error(err))
			continue
		}
		groups = append(groups, &g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(errQuery, tableNameBookmarkGroups, err)
	}

	return groups, nil
}

// GetAllBookmarks 获取所有书签
func (r *BookmarkRepository) GetAllBookmarks() ([]*BookmarkDB, error) {
	query := `SELECT ` + bookmarkSelectCols + ` FROM bookmarks ORDER BY id`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf(errQuery, tableNameBookmarks, err)
	}
	defer rows.Close()

	bookmarks := make([]*BookmarkDB, 0)
	for rows.Next() {
		b, err := scanBookmark(rows)
		if err != nil {
			r.logger.Error("scan bookmark failed", zap.Error(err))
			continue
		}
		bookmarks = append(bookmarks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(errQuery, tableNameBookmarks, err)
	}

	return bookmarks, nil
}

// GetBookmarkByID 按字符串 ID 查询书签
func (r *BookmarkRepository) GetBookmarkByID(id string) (*BookmarkDB, error) {
	query := `SELECT ` + bookmarkSelectCols + ` FROM bookmarks WHERE bookmark_id = ?`
	b, err := scanBookmark(r.db.QueryRow(query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("bookmark not found")
		}
		return nil, fmt.Errorf(errQuery, "bookmark by id", err)
	}
	return b, nil
}

// GetBookmarkByTitleAndGroup 按标题和分组 ID 查询书签（用于检查重复）
func (r *BookmarkRepository) GetBookmarkByTitleAndGroup(title string, groupID int) (*BookmarkDB, error) {
	query := `SELECT ` + bookmarkSelectCols + ` FROM bookmarks WHERE title = ? AND group_id = ?`
	b, err := scanBookmark(r.db.QueryRow(query, title, groupID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("bookmark not found")
		}
		return nil, fmt.Errorf(errQuery, "bookmark by title and group", err)
	}
	return b, nil
}

// InsertBookmark 插入书签
func (r *BookmarkRepository) InsertBookmark(bookmark *BookmarkDB) error {
	query := `INSERT INTO bookmarks (bookmark_id, group_id, title, host, port, user, password, private_key, private_key_password, proxy_jump_id, icon, use_agent, ssh_key_id, certificate, forward_agent, proxy_mode, proxy_type, proxy_host, proxy_port, proxy_user, proxy_password, startup_cmd, env_vars, term, tags, color, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query,
		bookmark.ID, bookmark.GroupID, bookmark.Title, bookmark.Host, bookmark.Port,
		bookmark.User, bookmark.Password, bookmark.PrivateKey, bookmark.PrivateKeyPassword, bookmark.ProxyJumpID, bookmark.Icon, boolInt(bookmark.UseAgent), bookmark.SshKeyID, bookmark.Certificate, boolInt(bookmark.ForwardAgent),
		bookmark.ProxyMode, bookmark.ProxyType, bookmark.ProxyHost, bookmark.ProxyPort, bookmark.ProxyUser, bookmark.ProxyPassword,
		bookmark.StartupCmd, bookmark.EnvVars, bookmark.Term, bookmark.Tags, bookmark.Color,
		bookmark.CreatedAt, bookmark.UpdatedAt)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "bookmark", err)
	}

	r.logger.Debug("bookmark inserted", zap.String("id", bookmark.ID))
	return nil
}

// UpdateBookmark 更新书签（根据字符串 ID）
func (r *BookmarkRepository) UpdateBookmark(bookmark *BookmarkDB) error {
	query := `UPDATE bookmarks 
			  SET group_id = ?, title = ?, host = ?, port = ?, user = ?, password = ?, 
			      private_key = ?, private_key_password = ?, proxy_jump_id = ?, icon = ?, use_agent = ?, ssh_key_id = ?, certificate = ?, forward_agent = ?,
			      proxy_mode = ?, proxy_type = ?, proxy_host = ?, proxy_port = ?, proxy_user = ?, proxy_password = ?,
			      startup_cmd = ?, env_vars = ?, term = ?, tags = ?, color = ?, updated_at = ? 
			  WHERE bookmark_id = ?`
	_, err := r.db.Exec(query,
		bookmark.GroupID, bookmark.Title, bookmark.Host, bookmark.Port,
		bookmark.User, bookmark.Password, bookmark.PrivateKey, bookmark.PrivateKeyPassword, bookmark.ProxyJumpID, bookmark.Icon, boolInt(bookmark.UseAgent), bookmark.SshKeyID, bookmark.Certificate, boolInt(bookmark.ForwardAgent),
		bookmark.ProxyMode, bookmark.ProxyType, bookmark.ProxyHost, bookmark.ProxyPort, bookmark.ProxyUser, bookmark.ProxyPassword,
		bookmark.StartupCmd, bookmark.EnvVars, bookmark.Term, bookmark.Tags, bookmark.Color,
		bookmark.UpdatedAt, bookmark.ID)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "update bookmark", err)
	}

	r.logger.Debug("bookmark updated", zap.String("id", bookmark.ID))
	return nil
}

// DeleteBookmark 删除书签（根据字符串 ID）
func (r *BookmarkRepository) DeleteBookmark(id string) error {
	_, err := r.db.Exec(`DELETE FROM bookmarks WHERE bookmark_id = ?`, id)
	if err != nil {
		return fmt.Errorf(errDeleteQuery, "bookmark", err)
	}

	r.logger.Debug("bookmark deleted", zap.String("id", id))
	return nil
}

// CountBySSHKeyID 统计引用某个应用内密钥的书签数量。
func (r *BookmarkRepository) CountBySSHKeyID(id string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE ssh_key_id = ?`, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf(errQuery, "bookmarks by ssh key", err)
	}
	return count, nil
}

// InsertGroup 插入分组
func (r *BookmarkRepository) InsertGroup(group *BookmarkGroupDB) error {
	query := `INSERT INTO bookmark_groups (name, icon) VALUES (?, ?)`
	_, err := r.db.Exec(query, group.Name, group.Icon)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "group", err)
	}

	r.logger.Debug("group inserted", zap.String("name", group.Name))
	return nil
}

// GetGroupByID 按ID查询分组
func (r *BookmarkRepository) GetGroupByID(id int) (*BookmarkGroupDB, error) {
	query := `SELECT id, name, COALESCE(icon, ''), created_at FROM bookmark_groups WHERE id = ?`
	row := r.db.QueryRow(query, id)

	var g BookmarkGroupDB
	err := row.Scan(&g.ID, &g.Name, &g.Icon, &g.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf(errQuery, "group by id", err)
	}

	return &g, nil
}

// GetGroupByName 按名称查询分组
func (r *BookmarkRepository) GetGroupByName(name string) (*BookmarkGroupDB, error) {
	query := `SELECT id, name, COALESCE(icon, ''), created_at FROM bookmark_groups WHERE name = ?`
	row := r.db.QueryRow(query, name)

	var g BookmarkGroupDB
	err := row.Scan(&g.ID, &g.Name, &g.Icon, &g.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, fmt.Errorf(errQuery, "group by name", err)
	}

	return &g, nil
}

// UpdateGroup 更新分组名称与图标
func (r *BookmarkRepository) UpdateGroup(oldName, newName, icon string) error {
	query := `UPDATE bookmark_groups SET name = ?, icon = ? WHERE name = ?`
	_, err := r.db.Exec(query, newName, icon, oldName)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "update group", err)
	}

	r.logger.Debug("group updated", zap.String("old", oldName), zap.String("new", newName))
	return nil
}

// GetGroupBookmarkCount 获取分组下的书签数量
func (r *BookmarkRepository) GetGroupBookmarkCount(groupName string) (int, error) {
	query := `SELECT COUNT(b.id) FROM bookmarks b
			  INNER JOIN bookmark_groups g ON b.group_id = g.id
			  WHERE g.name = ?`
	var count int
	err := r.db.QueryRow(query, groupName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf(errQuery, "group bookmark count", err)
	}
	return count, nil
}

// DeleteGroup 删除分组
func (r *BookmarkRepository) DeleteGroup(name string) error {
	_, err := r.db.Exec(`DELETE FROM bookmark_groups WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf(errDeleteQuery, "group", err)
	}

	r.logger.Debug("group deleted", zap.String("name", name))
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
