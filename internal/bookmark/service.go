package bookmark

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/utils"
)

const PasswordMask = "********"
const DefaultGroupName = "默认书签"

type PasswordFn func(reason string) (string, error)

// KeyPEMFunc 按应用内密钥 ID 解密私钥 PEM。
type KeyPEMFunc func(id string) (string, error)

type Bookmark struct {
	GroupName          string   `json:"group_name"`
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Host               string   `json:"host"`
	Port               int      `json:"port"`
	PrivateKey         string   `json:"private_key"`
	PrivateKeyPassword string   `json:"private_key_password"`
	ProxyJumpID        string   `json:"proxy_jump_id"`
	User               string   `json:"user"`
	Password           string   `json:"password"`
	Icon               string   `json:"icon"`
	UseAgent           bool     `json:"use_agent"` // DB 兼容字段，拨号时始终尝试 Agent，运行时忽略
	SshKeyID           string   `json:"ssh_key_id"`
	Certificate        string   `json:"certificate"`
	ForwardAgent       bool     `json:"forward_agent"`
	ProxyMode          string   `json:"proxy_mode"` // inherit | none | custom
	ProxyType          string   `json:"proxy_type"` // http | socks5
	ProxyHost          string   `json:"proxy_host"`
	ProxyPort          int      `json:"proxy_port"`
	ProxyUser          string   `json:"proxy_user"`
	ProxyPassword      string   `json:"proxy_password"`
	StartupCmd         string   `json:"startup_cmd"`
	EnvVars            string   `json:"env_vars"` // JSON object
	Term               string   `json:"term"`
	Tags               []string `json:"tags"`
	Color              string   `json:"color"`
}

func (b Bookmark) Endpoint() ssh.Endpoint {
	env, _ := ParseEnvVars(b.EnvVars)
	return ssh.Endpoint{
		Host: b.Host, Port: b.Port, User: b.User,
		Password: b.Password, Key: b.PrivateKey, KeyPassword: b.PrivateKeyPassword,
		Certificate: b.Certificate, ForwardAgent: b.ForwardAgent,
		StartupCmd: b.StartupCmd, Env: env, Term: b.Term,
	}
}

type Group struct {
	Name      string     `json:"name"`
	Icon      string     `json:"icon"`
	Bookmarks []Bookmark `json:"bookmarks"`
}

type ListItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	GroupName string `json:"group_name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
}

type Service struct {
	logger     *zap.Logger
	db         *database.Database
	passwordFn PasswordFn
	keyPEM     KeyPEMFunc
	onUpdate   func()
	onBadPass  func()
}

func New(logger *zap.Logger, db *database.Database, passwordFn PasswordFn, onUpdate, onBadPass func(), keyPEM KeyPEMFunc) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{logger: logger, db: db, passwordFn: passwordFn, onUpdate: onUpdate, onBadPass: onBadPass, keyPEM: keyPEM}
}

func (s *Service) emit() {
	if s.onUpdate != nil {
		s.onUpdate()
	}
}

func mask(p string) string {
	if p == "" {
		return ""
	}
	return PasswordMask
}

func (s *Service) password(reason string) (string, error) {
	if s.passwordFn == nil {
		return "", errors.New("password not entered")
	}
	return s.passwordFn(reason)
}

func (s *Service) encryptField(value, fieldName string) (string, error) {
	password, err := s.password("需要密码来加密书签相关密码")
	if err != nil {
		return "", err
	}
	encrypted, err := secret.Encrypt(password, value)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt %s: %w", fieldName, err)
	}
	return encrypted, nil
}

func (s *Service) encryptBookmark(b Bookmark) (Bookmark, error) {
	if b.PrivateKeyPassword != "" {
		v, err := s.encryptField(b.PrivateKeyPassword, "private key password")
		if err != nil {
			return b, err
		}
		b.PrivateKeyPassword = v
	}
	if b.Password != "" {
		v, err := s.encryptField(b.Password, "login password")
		if err != nil {
			return b, err
		}
		b.Password = v
	}
	if b.ProxyPassword != "" {
		v, err := s.encryptField(b.ProxyPassword, "proxy password")
		if err != nil {
			return b, err
		}
		b.ProxyPassword = v
	}
	return b, nil
}

func (s *Service) encryptFieldIfNeeded(newValue, existing, fieldName string) (string, error) {
	if newValue == "" {
		return "", nil
	}
	if newValue == PasswordMask {
		return existing, nil
	}
	return s.encryptField(newValue, fieldName)
}

func (s *Service) decryptField(encrypted, fieldName string) (string, error) {
	password, err := s.password("需要密码来加密书签相关密码")
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", errors.New("password not entered")
	}
	decrypted, err := secret.Decrypt(password, encrypted)
	if err != nil {
		if s.onBadPass != nil {
			s.onBadPass()
		}
		return "", fmt.Errorf("failed to decrypt %s: %w", fieldName, err)
	}
	return decrypted, nil
}

func (s *Service) decryptBookmark(b Bookmark) (Bookmark, error) {
	if b.PrivateKeyPassword != "" {
		v, err := s.decryptField(b.PrivateKeyPassword, "private key password")
		if err != nil {
			return b, err
		}
		b.PrivateKeyPassword = v
	}
	if b.Password != "" {
		v, err := s.decryptField(b.Password, "login password")
		if err != nil {
			return b, err
		}
		b.Password = v
	}
	if b.ProxyPassword != "" {
		v, err := s.decryptField(b.ProxyPassword, "proxy password")
		if err != nil {
			return b, err
		}
		b.ProxyPassword = v
	}
	return b, nil
}

func (s *Service) Get(id string) (*Bookmark, error) {
	dbBookmark, err := s.db.BookmarkRepo.GetBookmarkByID(id)
	if err != nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的书签", id)
	}
	groupName := ""
	if group, _ := s.db.BookmarkRepo.GetGroupByID(dbBookmark.GroupID); group != nil {
		groupName = group.Name
	}
	return fromDB(dbBookmark, groupName), nil
}

func fromDB(b *database.BookmarkDB, groupName string) *Bookmark {
	mode := b.ProxyMode
	if mode == "" {
		mode = "inherit"
	}
	return &Bookmark{
		ID: b.ID, GroupName: groupName, Title: b.Title, Host: b.Host, Port: b.Port,
		User: b.User, Password: b.Password, PrivateKey: b.PrivateKey,
		PrivateKeyPassword: b.PrivateKeyPassword, ProxyJumpID: b.ProxyJumpID, Icon: b.Icon,
		UseAgent: true, SshKeyID: b.SshKeyID, Certificate: b.Certificate, ForwardAgent: b.ForwardAgent,
		ProxyMode: mode, ProxyType: b.ProxyType, ProxyHost: b.ProxyHost, ProxyPort: b.ProxyPort,
		ProxyUser: b.ProxyUser, ProxyPassword: b.ProxyPassword,
		StartupCmd: b.StartupCmd, EnvVars: b.EnvVars, Term: b.Term,
		Tags: decodeTags(b.Tags), Color: normalizeColor(b.Color),
	}
}

func (s *Service) GetMasked(id string) (*Bookmark, error) {
	b, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	b.Password = mask(b.Password)
	b.PrivateKeyPassword = mask(b.PrivateKeyPassword)
	b.ProxyPassword = mask(b.ProxyPassword)
	return b, nil
}

func (s *Service) GetDecrypted(id string) (*Bookmark, error) {
	b, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	d, err := s.decryptBookmark(*b)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Service) ResolveHops(target ssh.Endpoint, jumpID string) ([]ssh.Endpoint, error) {
	hops := []ssh.Endpoint{target}
	seen := map[string]bool{target.Addr(): true}
	id := jumpID
	for id != "" {
		if len(hops) > 6 {
			return nil, fmt.Errorf("跳板机层数超过最大限制 (5)")
		}
		b, err := s.GetDecrypted(id)
		if err != nil {
			return nil, fmt.Errorf("failed to load proxy jump bookmark: %w", err)
		}
		ep, err := s.endpointWithKey(*b)
		if err != nil {
			return nil, err
		}
		// Session options belong only to the interactive target hop.
		ep.StartupCmd = ""
		ep.Env = nil
		ep.Term = ""
		ep.ForwardAgent = false
		if seen[ep.Addr()] {
			return nil, fmt.Errorf("检测到跳板机循环引用: %s", ep.Addr())
		}
		if ep.Host == hops[len(hops)-1].Host && ep.Port == hops[len(hops)-1].Port {
			return nil, fmt.Errorf("跳板机不能与目标主机相同")
		}
		seen[ep.Addr()] = true
		hops = append([]ssh.Endpoint{ep}, hops...)
		id = b.ProxyJumpID
	}
	return hops, nil
}

// ResolveHopsBookmark builds the hop chain from an already-decrypted bookmark.
func (s *Service) ResolveHopsBookmark(b Bookmark) ([]ssh.Endpoint, error) {
	ep, err := s.endpointWithKey(b)
	if err != nil {
		return nil, err
	}
	return s.ResolveHops(ep, b.ProxyJumpID)
}

func (s *Service) endpointWithKey(b Bookmark) (ssh.Endpoint, error) {
	return s.attachKey(b.Endpoint(), b.SshKeyID)
}

func (s *Service) attachKey(ep ssh.Endpoint, keyID string) (ssh.Endpoint, error) {
	if keyID == "" {
		return ep, nil
	}
	if s.keyPEM == nil {
		return ssh.Endpoint{}, fmt.Errorf("密钥服务未初始化")
	}
	pem, err := s.keyPEM(keyID)
	if err != nil {
		s.logger.Error("load stored ssh key failed", zap.String("key_id", keyID), zap.Error(err))
		return ssh.Endpoint{}, err
	}
	ep.KeyPEM = pem
	ep.Key = ""
	ep.KeyPassword = ""
	return ep, nil
}

func (s *Service) ListGrouped() ([]*Group, error) {
	dbGroups, err := s.db.BookmarkRepo.GetAllGroups()
	if err != nil {
		return nil, err
	}
	dbBookmarks, err := s.db.BookmarkRepo.GetAllBookmarks()
	if err != nil {
		return nil, err
	}
	groupIDToName := make(map[int]string)
	groupMap := make(map[int]*Group)
	for _, g := range dbGroups {
		groupIDToName[g.ID] = g.Name
		groupMap[g.ID] = &Group{Name: g.Name, Icon: g.Icon, Bookmarks: []Bookmark{}}
	}
	for _, b := range dbBookmarks {
		item := fromDB(b, groupIDToName[b.GroupID])
		item.Password = mask(item.Password)
		item.PrivateKeyPassword = mask(item.PrivateKeyPassword)
		item.ProxyPassword = mask(item.ProxyPassword)
		if g, ok := groupMap[b.GroupID]; ok {
			g.Bookmarks = append(g.Bookmarks, *item)
		}
	}
	groups := make([]*Group, 0, len(dbGroups))
	for _, g := range dbGroups {
		if group, ok := groupMap[g.ID]; ok {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (s *Service) ListItems() ([]*ListItem, error) {
	dbGroups, err := s.db.BookmarkRepo.GetAllGroups()
	if err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, g := range dbGroups {
		names[g.ID] = g.Name
	}
	dbBookmarks, err := s.db.BookmarkRepo.GetAllBookmarks()
	if err != nil {
		return nil, err
	}
	items := make([]*ListItem, 0, len(dbBookmarks))
	for _, b := range dbBookmarks {
		items = append(items, &ListItem{ID: b.ID, Title: b.Title, GroupName: names[b.GroupID], Host: b.Host, Port: b.Port, User: b.User})
	}
	return items, nil
}

func (s *Service) Save(b Bookmark) (string, error) {
	b, err := s.normalizeKeySource(b)
	if err != nil {
		return "", err
	}
	if err := validateSessionOptions(&b); err != nil {
		return "", err
	}
	normalizeMeta(&b)
	if b.ID != "" {
		existing, err := s.db.BookmarkRepo.GetBookmarkByID(b.ID)
		if err == nil && existing != nil {
			return b.ID, s.update(b, existing)
		}
	}
	return s.insert(b)
}

func (s *Service) update(b Bookmark, existing *database.BookmarkDB) error {
	processed, err := s.encryptBookmarkForSave(b, &Bookmark{
		Password: existing.Password, PrivateKeyPassword: existing.PrivateKeyPassword, ProxyPassword: existing.ProxyPassword,
	})
	if err != nil {
		return err
	}
	groupID := existing.GroupID
	if b.GroupName != "" {
		g, err := s.db.BookmarkRepo.GetGroupByName(b.GroupName)
		if err != nil {
			return fmt.Errorf("未找到分组 '%s'", b.GroupName)
		}
		groupID = g.ID
	}
	if existing.Title != b.Title || existing.GroupID != groupID {
		dup, err := s.db.BookmarkRepo.GetBookmarkByTitleAndGroup(b.Title, groupID)
		if err == nil && dup.ID != b.ID {
			return fmt.Errorf("分组 '%s' 中已存在名称为 '%s' 的书签", b.GroupName, b.Title)
		}
		if err != nil && err.Error() != "bookmark not found" {
			return err
		}
	}
	dbBookmark := &database.BookmarkDB{
		ID: processed.ID, GroupID: groupID, Title: processed.Title, Host: processed.Host, Port: processed.Port,
		User: processed.User, Password: processed.Password, PrivateKey: processed.PrivateKey,
		PrivateKeyPassword: processed.PrivateKeyPassword, ProxyJumpID: processed.ProxyJumpID,
		Icon: processed.Icon, UseAgent: true, SshKeyID: processed.SshKeyID, Certificate: processed.Certificate,
		ForwardAgent: processed.ForwardAgent,
		ProxyMode:    normalizeProxyMode(processed.ProxyMode), ProxyType: processed.ProxyType,
		ProxyHost: processed.ProxyHost, ProxyPort: processed.ProxyPort,
		ProxyUser: processed.ProxyUser, ProxyPassword: processed.ProxyPassword,
		StartupCmd: processed.StartupCmd, EnvVars: processed.EnvVars, Term: processed.Term,
		Tags: encodeTags(processed.Tags), Color: processed.Color,
		UpdatedAt: time.Now(),
	}
	if err := s.db.BookmarkRepo.UpdateBookmark(dbBookmark); err != nil {
		return err
	}
	s.emit()
	return nil
}

func normalizeProxyMode(mode string) string {
	switch mode {
	case "none", "custom":
		return mode
	default:
		return "inherit"
	}
}

func (s *Service) encryptBookmarkForSave(b Bookmark, existing *Bookmark) (Bookmark, error) {
	if existing == nil {
		return s.encryptBookmark(b)
	}
	var err error
	b.PrivateKeyPassword, err = s.encryptFieldIfNeeded(b.PrivateKeyPassword, existing.PrivateKeyPassword, "private key password")
	if err != nil {
		return b, err
	}
	b.Password, err = s.encryptFieldIfNeeded(b.Password, existing.Password, "login password")
	if err != nil {
		return b, err
	}
	b.ProxyPassword, err = s.encryptFieldIfNeeded(b.ProxyPassword, existing.ProxyPassword, "proxy password")
	return b, err
}

func (s *Service) insert(b Bookmark) (string, error) {
	groupName := b.GroupName
	if groupName == "" {
		groupName = DefaultGroupName
	}
	group, err := s.db.BookmarkRepo.GetGroupByName(groupName)
	if err != nil {
		return "", fmt.Errorf("未找到分组 '%s'", groupName)
	}
	exists, err := s.titleExistsInGroup(b.Title, group.ID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("分组 '%s' 中已存在名称为 '%s' 的书签", groupName, b.Title)
	}
	processed, err := s.encryptBookmark(b)
	if err != nil {
		return "", err
	}
	id := utils.GenerateRandomID()
	now := time.Now()
	if err := s.db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: id, GroupID: group.ID, Title: processed.Title, Host: processed.Host, Port: processed.Port,
		User: processed.User, Password: processed.Password, PrivateKey: processed.PrivateKey,
		PrivateKeyPassword: processed.PrivateKeyPassword, ProxyJumpID: processed.ProxyJumpID,
		Icon: processed.Icon, UseAgent: true, SshKeyID: processed.SshKeyID, Certificate: processed.Certificate,
		ForwardAgent: processed.ForwardAgent,
		ProxyMode:    normalizeProxyMode(processed.ProxyMode), ProxyType: processed.ProxyType,
		ProxyHost: processed.ProxyHost, ProxyPort: processed.ProxyPort,
		ProxyUser: processed.ProxyUser, ProxyPassword: processed.ProxyPassword,
		StartupCmd: processed.StartupCmd, EnvVars: processed.EnvVars, Term: processed.Term,
		Tags: encodeTags(processed.Tags), Color: processed.Color,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return "", err
	}
	s.emit()
	return id, nil
}

func validateSessionOptions(b *Bookmark) error {
	m, err := ParseEnvVars(b.EnvVars)
	if err != nil {
		return err
	}
	normalized, err := FormatEnvVars(m)
	if err != nil {
		return err
	}
	b.EnvVars = normalized
	b.StartupCmd = strings.TrimSpace(b.StartupCmd)
	if strings.ContainsAny(b.StartupCmd, "\r\n") {
		return fmt.Errorf("启动命令不能包含换行")
	}
	b.Term = strings.TrimSpace(b.Term)
	return nil
}

func (s *Service) Delete(id string) error {
	if err := s.db.BookmarkRepo.DeleteBookmark(id); err != nil {
		return err
	}
	s.emit()
	return nil
}

func (s *Service) AddGroup(name string) error {
	if _, err := s.db.BookmarkRepo.GetGroupByName(name); err == nil {
		return fmt.Errorf("分组 '%s' 已存在", name)
	}
	if err := s.db.BookmarkRepo.InsertGroup(&database.BookmarkGroupDB{Name: name}); err != nil {
		return err
	}
	s.emit()
	return nil
}

func (s *Service) UpdateGroup(oldName, newName, icon string) error {
	if newName == "" {
		newName = oldName
	}
	if oldName == DefaultGroupName && newName != oldName {
		return fmt.Errorf("不能修改默认分组名称")
	}
	if newName != oldName {
		if _, err := s.db.BookmarkRepo.GetGroupByName(newName); err == nil {
			return fmt.Errorf("分组 '%s' 已存在", newName)
		}
	}
	if err := s.db.BookmarkRepo.UpdateGroup(oldName, newName, icon); err != nil {
		return err
	}
	s.emit()
	return nil
}

func (s *Service) Copy(id string) (*Bookmark, error) {
	src, err := s.db.BookmarkRepo.GetBookmarkByID(id)
	if err != nil {
		return nil, fmt.Errorf("未找到 ID 为 '%s' 的书签", id)
	}
	title, err := s.uniqueCopyTitle(src.Title, src.GroupID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	newID := utils.GenerateRandomID()
	if err := s.db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: newID, GroupID: src.GroupID, Title: title, Host: src.Host, Port: src.Port,
		User: src.User, Password: src.Password, PrivateKey: src.PrivateKey,
		PrivateKeyPassword: src.PrivateKeyPassword, ProxyJumpID: src.ProxyJumpID,
		Icon: src.Icon, UseAgent: true, SshKeyID: src.SshKeyID, Certificate: src.Certificate,
		ForwardAgent: src.ForwardAgent,
		ProxyMode:    normalizeProxyMode(src.ProxyMode), ProxyType: src.ProxyType,
		ProxyHost: src.ProxyHost, ProxyPort: src.ProxyPort,
		ProxyUser: src.ProxyUser, ProxyPassword: src.ProxyPassword,
		StartupCmd: src.StartupCmd, EnvVars: src.EnvVars, Term: src.Term,
		Tags: src.Tags, Color: src.Color,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return nil, err
	}
	s.emit()
	groupName := ""
	if group, _ := s.db.BookmarkRepo.GetGroupByID(src.GroupID); group != nil {
		groupName = group.Name
	}
	copied := fromDB(src, groupName)
	copied.ID = newID
	copied.Title = title
	copied.Password = mask(copied.Password)
	copied.PrivateKeyPassword = mask(copied.PrivateKeyPassword)
	copied.ProxyPassword = mask(copied.ProxyPassword)
	return copied, nil
}

func (s *Service) titleExistsInGroup(title string, groupID int) (bool, error) {
	_, err := s.db.BookmarkRepo.GetBookmarkByTitleAndGroup(title, groupID)
	if err == nil {
		return true, nil
	}
	if err.Error() == "bookmark not found" {
		return false, nil
	}
	return false, err
}

func (s *Service) uniqueCopyTitle(base string, groupID int) (string, error) {
	title := base + " 复制"
	exists, err := s.titleExistsInGroup(title, groupID)
	if err != nil {
		return "", err
	}
	if !exists {
		return title, nil
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s 复制%d", base, i)
		exists, err := s.titleExistsInGroup(candidate, groupID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
}

func (s *Service) DeleteGroup(name string) error {
	if name == DefaultGroupName {
		return fmt.Errorf("不能删除默认分组")
	}
	count, err := s.db.BookmarkRepo.GetGroupBookmarkCount(name)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("分组 '%s' 下还有 %d 个书签，无法删除", name, count)
	}
	if err := s.db.BookmarkRepo.DeleteGroup(name); err != nil {
		return err
	}
	s.emit()
	return nil
}

// PrepareTestResult holds dial credentials ready for a connection test.
// ProxyPassword is restored from storage when the form still shows the mask.
type PrepareTestResult struct {
	Endpoint      ssh.Endpoint
	JumpID        string
	ProxyPassword string
}

func (s *Service) PrepareTest(b Bookmark) (PrepareTestResult, error) {
	ep := b.Endpoint()
	jumpID := b.ProxyJumpID
	proxyPass := b.ProxyPassword
	if b.ID != "" {
		existing, err := s.Get(b.ID)
		if err != nil {
			return PrepareTestResult{}, err
		}
		sameHost := b.Host == existing.Host && b.Port == existing.Port && b.User == existing.User
		passwordUnchanged := b.Password == "" || b.Password == PasswordMask
		keyPassUnchanged := b.PrivateKeyPassword == "" || b.PrivateKeyPassword == PasswordMask
		proxyPassUnchanged := b.ProxyPassword == "" || b.ProxyPassword == PasswordMask
		sameKeyFile := b.PrivateKey == existing.PrivateKey
		sameStoredKey := b.SshKeyID == existing.SshKeyID
		fullReuse := sameHost && passwordUnchanged && keyPassUnchanged && sameKeyFile && sameStoredKey
		switch {
		case fullReuse:
			decrypted, err := s.GetDecrypted(b.ID)
			if err != nil {
				return PrepareTestResult{}, err
			}
			ep = decrypted.Endpoint()
			if proxyPassUnchanged {
				proxyPass = decrypted.ProxyPassword
			}
		case sameHost && b.Password == PasswordMask:
			decrypted, err := s.GetDecrypted(b.ID)
			if err != nil {
				return PrepareTestResult{}, err
			}
			ep.Password = decrypted.Password
			if sameKeyFile && keyPassUnchanged {
				ep.KeyPassword = decrypted.PrivateKeyPassword
			} else if b.PrivateKeyPassword == PasswordMask {
				ep.KeyPassword = ""
			}
			if proxyPassUnchanged {
				proxyPass = decrypted.ProxyPassword
			}
		case b.ProxyPassword == PasswordMask:
			decrypted, err := s.GetDecrypted(b.ID)
			if err != nil {
				return PrepareTestResult{}, err
			}
			proxyPass = decrypted.ProxyPassword
		}
	}
	ep, err := s.attachKey(ep, b.SshKeyID)
	if err != nil {
		return PrepareTestResult{}, err
	}
	ep.Certificate = b.Certificate
	ep.ForwardAgent = b.ForwardAgent
	ep.StartupCmd = b.StartupCmd
	ep.Term = b.Term
	if env, err := ParseEnvVars(b.EnvVars); err == nil {
		ep.Env = env
	}
	return PrepareTestResult{Endpoint: ep, JumpID: jumpID, ProxyPassword: proxyPass}, nil
}

func (s *Service) normalizeKeySource(b Bookmark) (Bookmark, error) {
	if b.SshKeyID == "" {
		return b, nil
	}
	if _, err := s.db.SSHKeyRepo.Get(b.SshKeyID); err != nil {
		return b, fmt.Errorf("应用内密钥不存在")
	}
	b.PrivateKey = ""
	b.PrivateKeyPassword = ""
	return b, nil
}
