package bookmark

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/ssh"
	"github.com/ilaziness/vela/internal/termcharset"
	"github.com/ilaziness/vela/internal/termio"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/utils"
)

const PasswordMask = "********"

// DefaultGroupName is the stable persisted id for the default bookmark group.
// UI must display via i18n key bookmark.group.default.
const DefaultGroupName = "__default__"

// LegacyDefaultGroupName is the pre-i18n Chinese name; migrated to DefaultGroupName.
const LegacyDefaultGroupName = "默认书签"

type PasswordFn func(reason string) (string, error)

// KeyPEMFunc 按应用内密钥 ID 解密私钥 PEM。
type KeyPEMFunc func(id string) (string, error)

type Bookmark struct {
	GroupName          string   `json:"group_name"`
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Protocol           string   `json:"protocol"` // ssh | serial (explicit required)
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
	Charset            string   `json:"charset"` // utf-8 | gbk | gb18030
	SerialDevice       string   `json:"serial_device"`
	BaudRate           int      `json:"baud_rate"`
	DataBits           int      `json:"data_bits"`
	Parity             string   `json:"parity"`
	StopBits           int      `json:"stop_bits"`
	FlowControl        string   `json:"flow_control"`
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
		Charset: normalizeStoredCharset(b.Charset),
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
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
}

type Service struct {
	logger      *zap.Logger
	db          *database.Database
	passwordFn  PasswordFn
	keyPEM      KeyPEMFunc
	onUpdate    func()
	onBadPass   func()
	onEncrypted func()
}

func New(logger *zap.Logger, db *database.Database, passwordFn PasswordFn, onUpdate, onBadPass func(), keyPEM KeyPEMFunc, onEncrypted func()) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{logger: logger, db: db, passwordFn: passwordFn, onUpdate: onUpdate, onBadPass: onBadPass, keyPEM: keyPEM, onEncrypted: onEncrypted}
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
	password, err := s.password(i18n.T("password.encrypt_bookmark"))
	if err != nil {
		return "", err
	}
	encrypted, err := secret.Encrypt(password, value)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt %s: %w", fieldName, err)
	}
	if s.onEncrypted != nil {
		s.onEncrypted()
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
	password, err := s.password(i18n.T("password.encrypt_decrypt_required"))
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
		return nil, usererr.Coded("bookmark.not_found", "Bookmark not found")
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
		ID: b.ID, GroupName: groupName, Title: b.Title, Protocol: b.Protocol,
		Host: b.Host, Port: b.Port,
		User: b.User, Password: b.Password, PrivateKey: b.PrivateKey,
		PrivateKeyPassword: b.PrivateKeyPassword, ProxyJumpID: b.ProxyJumpID, Icon: b.Icon,
		UseAgent: true, SshKeyID: b.SshKeyID, Certificate: b.Certificate, ForwardAgent: b.ForwardAgent,
		ProxyMode: mode, ProxyType: b.ProxyType, ProxyHost: b.ProxyHost, ProxyPort: b.ProxyPort,
		ProxyUser: b.ProxyUser, ProxyPassword: b.ProxyPassword,
		StartupCmd: b.StartupCmd, EnvVars: b.EnvVars, Term: b.Term,
		Charset:      normalizeStoredCharset(b.Charset),
		SerialDevice: b.SerialDevice, BaudRate: b.BaudRate, DataBits: b.DataBits,
		Parity: b.Parity, StopBits: b.StopBits, FlowControl: b.FlowControl,
		Tags: decodeTags(b.Tags), Color: normalizeColor(b.Color),
	}
}

func normalizeStoredCharset(v string) string {
	n, err := termcharset.Normalize(v)
	if err != nil {
		return termcharset.UTF8
	}
	return n
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
			return nil, usererr.Coded("bookmark.jump_too_deep", "ProxyJump chain exceeds the maximum of 5 hops")
		}
		b, err := s.GetDecrypted(id)
		if err != nil {
			return nil, fmt.Errorf("failed to load proxy jump bookmark: %w", err)
		}
		if proto, ok := termio.ParseProtocol(b.Protocol); !ok || proto != termio.ProtocolSSH {
			return nil, usererr.Coded("bookmark.jump_not_ssh", "ProxyJump target must be an SSH bookmark")
		}
		ep, err := s.endpointWithKey(*b)
		if err != nil {
			return nil, err
		}
		// Session options belong only to the interactive target hop.
		ep.StartupCmd = ""
		ep.Env = nil
		ep.Term = ""
		ep.Charset = ""
		ep.ForwardAgent = false
		if seen[ep.Addr()] {
			return nil, usererr.Coded("bookmark.jump_cycle", "ProxyJump cycle detected: %s", ep.Addr())
		}
		if ep.Host == hops[len(hops)-1].Host && ep.Port == hops[len(hops)-1].Port {
			return nil, usererr.Coded("bookmark.jump_same_host", "ProxyJump host cannot be the same as the target")
		}
		seen[ep.Addr()] = true
		hops = append([]ssh.Endpoint{ep}, hops...)
		id = b.ProxyJumpID
	}
	return hops, nil
}

// ResolveHopsBookmark builds the hop chain from an already-decrypted bookmark.
func (s *Service) ResolveHopsBookmark(b Bookmark) ([]ssh.Endpoint, error) {
	if proto, ok := termio.ParseProtocol(b.Protocol); !ok || proto != termio.ProtocolSSH {
		return nil, usererr.Coded("bookmark.jump_not_ssh", "ProxyJump target must be an SSH bookmark")
	}
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
		return ssh.Endpoint{}, usererr.Coded("bookmark.key_service_nil", "Key service is not initialized")
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
		items = append(items, &ListItem{
			ID: b.ID, Title: b.Title, GroupName: names[b.GroupID], Protocol: b.Protocol,
			Host: b.Host, Port: b.Port, User: b.User,
		})
	}
	return items, nil
}

func (s *Service) Save(b Bookmark) (string, error) {
	if err := validateBookmarkProtocol(&b); err != nil {
		return "", err
	}
	if termio.Protocol(b.Protocol) == termio.ProtocolSSH {
		var err error
		b, err = s.normalizeKeySource(b)
		if err != nil {
			return "", err
		}
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

func validateBookmarkProtocol(b *Bookmark) error {
	proto, ok := termio.ParseProtocol(strings.TrimSpace(b.Protocol))
	if !ok {
		return usererr.Coded("session.unknown_protocol", "Unknown session protocol: %s", b.Protocol)
	}
	b.Protocol = string(proto)
	switch proto {
	case termio.ProtocolSSH:
		if strings.TrimSpace(b.Host) == "" {
			return usererr.Coded("bookmark.host_required", "Host is required")
		}
		if b.Port <= 0 {
			return usererr.Coded("bookmark.port_required", "Port is required")
		}
		if strings.TrimSpace(b.User) == "" {
			return usererr.Coded("bookmark.user_required", "User is required")
		}
		b.SerialDevice = ""
		b.BaudRate = 0
		b.DataBits = 0
		b.Parity = ""
		b.StopBits = 0
		b.FlowControl = ""
	case termio.ProtocolSerial:
		if strings.TrimSpace(b.SerialDevice) == "" {
			return usererr.Coded("serial.device_required", "Serial device is required")
		}
		if b.BaudRate <= 0 {
			return usererr.Coded("serial.baud_required", "Baud rate is required")
		}
		if b.DataBits == 0 {
			b.DataBits = 8
		}
		if b.StopBits == 0 {
			b.StopBits = 1
		}
		if strings.TrimSpace(b.Parity) == "" {
			b.Parity = "none"
		}
		if strings.TrimSpace(b.FlowControl) == "" {
			b.FlowControl = "none"
		}
		// Do not store COM name in host; keep SSH dial fields empty.
		b.Host = ""
		b.Port = 0
		b.User = ""
		b.Password = ""
		b.PrivateKey = ""
		b.PrivateKeyPassword = ""
		b.ProxyJumpID = ""
		b.SshKeyID = ""
		b.Certificate = ""
		b.ForwardAgent = false
		b.ProxyMode = "inherit"
		b.ProxyType = ""
		b.ProxyHost = ""
		b.ProxyPort = 0
		b.ProxyUser = ""
		b.ProxyPassword = ""
		b.StartupCmd = ""
		b.EnvVars = ""
		b.Term = ""
	}
	return nil
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
			return usererr.Coded("bookmark.group_not_found", "Group '%s' not found", b.GroupName)
		}
		groupID = g.ID
	}
	if existing.Title != b.Title || existing.GroupID != groupID {
		dup, err := s.db.BookmarkRepo.GetBookmarkByTitleAndGroup(b.Title, groupID)
		if err == nil && dup.ID != b.ID {
			return usererr.Coded("bookmark.title_exists", "Bookmark '%s' already exists in group '%s'", b.Title, b.GroupName)
		}
		if err != nil && err.Error() != "bookmark not found" {
			return err
		}
	}
	dbBookmark := &database.BookmarkDB{
		ID: processed.ID, GroupID: groupID, Title: processed.Title, Protocol: processed.Protocol,
		Host: processed.Host, Port: processed.Port,
		User: processed.User, Password: processed.Password, PrivateKey: processed.PrivateKey,
		PrivateKeyPassword: processed.PrivateKeyPassword, ProxyJumpID: processed.ProxyJumpID,
		Icon: processed.Icon, UseAgent: true, SshKeyID: processed.SshKeyID, Certificate: processed.Certificate,
		ForwardAgent: processed.ForwardAgent,
		ProxyMode:    normalizeProxyMode(processed.ProxyMode), ProxyType: processed.ProxyType,
		ProxyHost: processed.ProxyHost, ProxyPort: processed.ProxyPort,
		ProxyUser: processed.ProxyUser, ProxyPassword: processed.ProxyPassword,
		StartupCmd: processed.StartupCmd, EnvVars: processed.EnvVars, Term: processed.Term,
		Charset:      processed.Charset,
		SerialDevice: processed.SerialDevice, BaudRate: processed.BaudRate, DataBits: processed.DataBits,
		Parity: processed.Parity, StopBits: processed.StopBits, FlowControl: processed.FlowControl,
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
		return "", usererr.Coded("bookmark.group_not_found", "Group '%s' not found", groupName)
	}
	exists, err := s.titleExistsInGroup(b.Title, group.ID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", usererr.Coded("bookmark.title_exists", "Bookmark '%s' already exists in group '%s'", b.Title, groupName)
	}
	processed, err := s.encryptBookmark(b)
	if err != nil {
		return "", err
	}
	id := utils.GenerateRandomID()
	now := time.Now()
	if err := s.db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: id, GroupID: group.ID, Title: processed.Title, Protocol: processed.Protocol,
		Host: processed.Host, Port: processed.Port,
		User: processed.User, Password: processed.Password, PrivateKey: processed.PrivateKey,
		PrivateKeyPassword: processed.PrivateKeyPassword, ProxyJumpID: processed.ProxyJumpID,
		Icon: processed.Icon, UseAgent: true, SshKeyID: processed.SshKeyID, Certificate: processed.Certificate,
		ForwardAgent: processed.ForwardAgent,
		ProxyMode:    normalizeProxyMode(processed.ProxyMode), ProxyType: processed.ProxyType,
		ProxyHost: processed.ProxyHost, ProxyPort: processed.ProxyPort,
		ProxyUser: processed.ProxyUser, ProxyPassword: processed.ProxyPassword,
		StartupCmd: processed.StartupCmd, EnvVars: processed.EnvVars, Term: processed.Term,
		Charset:      processed.Charset,
		SerialDevice: processed.SerialDevice, BaudRate: processed.BaudRate, DataBits: processed.DataBits,
		Parity: processed.Parity, StopBits: processed.StopBits, FlowControl: processed.FlowControl,
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
		return usererr.Coded("bookmark.startup_cmd_newline", "Startup command cannot contain newlines")
	}
	b.Term = strings.TrimSpace(b.Term)
	charset, err := termcharset.Normalize(b.Charset)
	if err != nil {
		return err
	}
	b.Charset = charset
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
		return usererr.Coded("bookmark.group_exists", "Group '%s' already exists", name)
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
	if (oldName == DefaultGroupName || oldName == LegacyDefaultGroupName) && newName != oldName {
		return usererr.Coded("bookmark.cannot_rename_default", "Cannot rename the default group")
	}
	if newName != oldName {
		if _, err := s.db.BookmarkRepo.GetGroupByName(newName); err == nil {
			return usererr.Coded("bookmark.group_exists", "Group '%s' already exists", newName)
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
		return nil, usererr.Coded("bookmark.not_found", "Bookmark not found")
	}
	title, err := s.uniqueCopyTitle(src.Title, src.GroupID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	newID := utils.GenerateRandomID()
	if err := s.db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: newID, GroupID: src.GroupID, Title: title, Protocol: src.Protocol,
		Host: src.Host, Port: src.Port,
		User: src.User, Password: src.Password, PrivateKey: src.PrivateKey,
		PrivateKeyPassword: src.PrivateKeyPassword, ProxyJumpID: src.ProxyJumpID,
		Icon: src.Icon, UseAgent: true, SshKeyID: src.SshKeyID, Certificate: src.Certificate,
		ForwardAgent: src.ForwardAgent,
		ProxyMode:    normalizeProxyMode(src.ProxyMode), ProxyType: src.ProxyType,
		ProxyHost: src.ProxyHost, ProxyPort: src.ProxyPort,
		ProxyUser: src.ProxyUser, ProxyPassword: src.ProxyPassword,
		StartupCmd: src.StartupCmd, EnvVars: src.EnvVars, Term: src.Term,
		Charset:      normalizeStoredCharset(src.Charset),
		SerialDevice: src.SerialDevice, BaudRate: src.BaudRate, DataBits: src.DataBits,
		Parity: src.Parity, StopBits: src.StopBits, FlowControl: src.FlowControl,
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
	title := base + " copy"
	exists, err := s.titleExistsInGroup(title, groupID)
	if err != nil {
		return "", err
	}
	if !exists {
		return title, nil
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s copy%d", base, i)
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
	if name == DefaultGroupName || name == LegacyDefaultGroupName {
		return usererr.Coded("bookmark.cannot_delete_default", "Cannot delete the default group")
	}
	count, err := s.db.BookmarkRepo.GetGroupBookmarkCount(name)
	if err != nil {
		return err
	}
	if count > 0 {
		return usererr.Coded("bookmark.group_not_empty", "Group '%s' still has %s bookmarks and cannot be deleted", name, strconv.Itoa(count))
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
	ep.Charset = normalizeStoredCharset(b.Charset)
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
		return b, usererr.Coded("bookmark.app_key_missing", "Application SSH key not found")
	}
	b.PrivateKey = ""
	b.PrivateKeyPassword = ""
	return b, nil
}
