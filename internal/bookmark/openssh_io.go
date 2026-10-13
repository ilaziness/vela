package bookmark

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/sshconfig"
)

const ImportedGroupName = "Imported"

// ImportResult summarizes an OpenSSH config import.
type ImportResult struct {
	Created      int      `json:"created"`
	UpdatedJumps int      `json:"updated_jumps"`
	Warnings     []string `json:"warnings"`
	Cancelled    bool     `json:"cancelled"`
}

// ExportResult summarizes an OpenSSH config export.
type ExportResult struct {
	Written   bool     `json:"written"`
	Warnings  []string `json:"warnings"`
	Cancelled bool     `json:"cancelled"`
}

// ImportOpenSSH parses path and creates bookmarks in groupName.
func (s *Service) ImportOpenSSH(path, groupName string) (ImportResult, error) {
	res := ImportResult{Warnings: []string{}}
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		groupName = ImportedGroupName
	}
	if err := s.ensureGroup(groupName); err != nil {
		return res, err
	}

	blocks, parseWarns, err := sshconfig.ParseFile(path)
	for _, w := range parseWarns {
		res.Warnings = append(res.Warnings, w.Message)
	}
	if err != nil {
		s.logger.Error("parse ssh_config failed", zap.String("path", path), zap.Error(err))
		return res, fmt.Errorf("解析 ssh_config 失败: %w", err)
	}
	entries, resolveWarns := sshconfig.Resolve(blocks)
	for _, w := range resolveWarns {
		res.Warnings = append(res.Warnings, w.Message)
	}
	if len(entries) == 0 {
		res.Warnings = append(res.Warnings, "未找到可导入的 Host")
		return res, nil
	}

	aliasToID := map[string]string{}
	jumpRaw := map[string]string{} // bookmark id → ProxyJump raw

	for _, e := range entries {
		title, err := s.uniqueImportTitle(e.Alias, groupName)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("Host %s: %v", e.Alias, err))
			continue
		}
		b := entryToBookmark(e, title, groupName)
		id, err := s.Save(b)
		if err != nil {
			s.logger.Warn("import ssh_config host failed",
				zap.String("alias", e.Alias), zap.Error(err))
			res.Warnings = append(res.Warnings, fmt.Sprintf("Host %s 导入失败: %v", e.Alias, err))
			continue
		}
		aliasToID[e.Alias] = id
		if e.ProxyJump != "" {
			jumpRaw[id] = e.ProxyJump
		}
		res.Created++
	}

	n, warns := s.wireAllProxyJumps(jumpRaw, aliasToID, groupName)
	res.UpdatedJumps += n
	res.Warnings = append(res.Warnings, warns...)
	return res, nil
}

// ExportOpenSSH builds an OpenSSH config for groupName (empty = all groups).
func (s *Service) ExportOpenSSH(groupName string) (string, []string, error) {
	var warnings []string
	groups, err := s.ListGrouped()
	if err != nil {
		return "", nil, err
	}
	idToTitle := map[string]string{}
	var bookmarks []Bookmark
	for _, g := range groups {
		if g == nil {
			continue
		}
		if groupName != "" && g.Name != groupName {
			continue
		}
		for _, b := range g.Bookmarks {
			idToTitle[b.ID] = b.Title
			bookmarks = append(bookmarks, b)
		}
	}
	if len(bookmarks) == 0 {
		return "", []string{"没有可导出的书签"}, nil
	}

	entries := make([]sshconfig.HostEntry, 0, len(bookmarks))
	for _, b := range bookmarks {
		e := sshconfig.HostEntry{
			Alias:         b.Title,
			HostName:      b.Host,
			User:          b.User,
			Port:          b.Port,
			RemoteCommand: b.StartupCmd,
		}
		if b.Certificate != "" {
			e.CertificateFile = b.Certificate
		}
		if b.PrivateKey != "" && b.SshKeyID == "" {
			e.IdentityFiles = []string{b.PrivateKey}
		} else if b.SshKeyID != "" {
			warnings = append(warnings, fmt.Sprintf("%s: 使用应用内密钥，未导出 IdentityFile", b.Title))
		}
		if b.ForwardAgent {
			yes := true
			e.ForwardAgent = &yes
		}
		if b.ProxyJumpID != "" {
			if chain := s.exportProxyJumpChain(b.ProxyJumpID, idToTitle); chain != "" {
				e.ProxyJump = chain
			} else {
				warnings = append(warnings, fmt.Sprintf("%s: 跳板书签缺失，未导出 ProxyJump", b.Title))
			}
		}
		if term := strings.TrimSpace(b.Term); term != "" {
			e.SetEnv = append(e.SetEnv, sshconfig.KV{Key: "TERM", Value: term})
		}
		if env, err := ParseEnvVars(b.EnvVars); err == nil {
			keys := make([]string, 0, len(env))
			for k := range env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				e.SetEnv = append(e.SetEnv, sshconfig.KV{Key: k, Value: env[k]})
			}
		}
		entries = append(entries, e)
	}

	var buf strings.Builder
	if err := sshconfig.Write(&buf, entries); err != nil {
		return "", warnings, err
	}
	return buf.String(), warnings, nil
}

func (s *Service) exportProxyJumpChain(jumpID string, idToTitle map[string]string) string {
	// vela: target.proxy_jump_id = nearest jump. OpenSSH ProxyJump a,b = client→a→b→target.
	var hops []string
	seen := map[string]bool{}
	id := jumpID
	for id != "" && !seen[id] {
		seen[id] = true
		b, err := s.Get(id)
		if err != nil {
			if t := idToTitle[id]; t != "" {
				hops = append(hops, t)
			}
			break
		}
		title := idToTitle[id]
		if title == "" {
			title = b.Title
			idToTitle[id] = title
		}
		hops = append(hops, title)
		id = b.ProxyJumpID
		if len(hops) > 5 {
			break
		}
	}
	for i, j := 0, len(hops)-1; i < j; i, j = i+1, j-1 {
		hops[i], hops[j] = hops[j], hops[i]
	}
	return strings.Join(hops, ",")
}

func entryToBookmark(e sshconfig.HostEntry, title, groupName string) Bookmark {
	host := e.HostName
	if host == "" {
		host = e.Alias
	}
	port := e.Port
	if port <= 0 {
		port = 22
	}
	b := Bookmark{
		GroupName:  groupName,
		Title:      title,
		Host:       host,
		Port:       port,
		User:       e.User,
		ProxyMode:  "inherit",
		StartupCmd: e.RemoteCommand,
	}
	if len(e.IdentityFiles) > 0 {
		b.PrivateKey = e.IdentityFiles[0]
	}
	if e.CertificateFile != "" {
		b.Certificate = e.CertificateFile
	}
	if e.ForwardAgent != nil {
		b.ForwardAgent = *e.ForwardAgent
	}
	if len(e.SetEnv) > 0 {
		m := map[string]string{}
		for _, kv := range e.SetEnv {
			if kv.Key == "" {
				continue
			}
			if strings.EqualFold(kv.Key, "TERM") {
				if b.Term == "" {
					b.Term = kv.Value
				}
				continue
			}
			m[kv.Key] = kv.Value
		}
		if raw, err := FormatEnvVars(m); err == nil {
			b.EnvVars = raw
		}
	}
	return b
}

func (s *Service) wireAllProxyJumps(jumpRaw map[string]string, aliasToID map[string]string, groupName string) (int, []string) {
	type item struct {
		id  string
		raw string
	}
	var single, multi []item
	for id, raw := range jumpRaw {
		if len(splitProxyJump(raw)) <= 1 {
			single = append(single, item{id, raw})
		} else {
			multi = append(multi, item{id, raw})
		}
	}
	updated := 0
	var warnings []string
	// Own single-hop ProxyJump first so multi-hop wiring will not clobber them blindly.
	for _, it := range append(single, multi...) {
		n, warns := s.wireProxyJump(it.id, it.raw, aliasToID, groupName)
		updated += n
		warnings = append(warnings, warns...)
	}
	return updated, warnings
}

func (s *Service) wireProxyJump(targetID, raw string, aliasToID map[string]string, groupName string) (int, []string) {
	var warnings []string
	parts := splitProxyJump(raw)
	if len(parts) == 0 {
		return 0, nil
	}
	if len(parts) > 5 {
		warnings = append(warnings, fmt.Sprintf("书签 %s: ProxyJump 超过 5 跳，已截断", shortID(targetID)))
		parts = parts[:5]
	}
	// OpenSSH: a,b → client→a→b→target
	ids := make([]string, 0, len(parts))
	for _, alias := range parts {
		id, warn := s.resolveJumpAlias(alias, aliasToID, groupName)
		if id == "" {
			if warn != "" {
				warnings = append(warnings, warn)
			}
			warnings = append(warnings, fmt.Sprintf("跳过书签 %s 的 ProxyJump 链", shortID(targetID)))
			return 0, warnings
		}
		ids = append(ids, id)
	}
	// Wire: target → last, last → previous, ...
	// Never overwrite an intermediate hop's existing different ProxyJump.
	updated := 0
	chain := make([]string, 0, len(ids)+1)
	chain = append(chain, ids...)
	chain = append(chain, targetID)
	for i := len(chain) - 1; i > 0; i-- {
		cur := chain[i]
		prev := chain[i-1]
		if cur != targetID {
			existing, err := s.db.BookmarkRepo.GetBookmarkByID(cur)
			if err == nil && existing.ProxyJumpID != "" && existing.ProxyJumpID != prev {
				title := existing.Title
				warnings = append(warnings, fmt.Sprintf(
					"跳板 %q 已有不同 ProxyJump，多跳链未改写该跳板（书签 %s）", title, shortID(targetID)))
				continue
			}
		}
		if err := s.setProxyJumpID(cur, prev); err != nil {
			warnings = append(warnings, fmt.Sprintf("绑定 ProxyJump 失败 %s→%s: %v", shortID(cur), shortID(prev), err))
			continue
		}
		updated++
	}
	return updated, warnings
}

func (s *Service) resolveJumpAlias(alias string, aliasToID map[string]string, groupName string) (string, string) {
	alias = strings.TrimSpace(alias)
	// Strip user@host:port style to host/alias token for lookup
	if at := strings.LastIndex(alias, "@"); at >= 0 {
		alias = alias[at+1:]
	}
	if i := strings.Index(alias, ":"); i >= 0 {
		alias = alias[:i]
	}
	if id, ok := aliasToID[alias]; ok {
		return id, ""
	}
	// Same import group by title
	if g, err := s.db.BookmarkRepo.GetGroupByName(groupName); err == nil {
		if b, err := s.db.BookmarkRepo.GetBookmarkByTitleAndGroup(alias, g.ID); err == nil {
			return b.ID, ""
		}
	}
	// Global unique title
	all, err := s.db.BookmarkRepo.GetAllBookmarks()
	if err != nil {
		return "", fmt.Sprintf("查找跳板 %q 失败: %v", alias, err)
	}
	var matches []*database.BookmarkDB
	for _, b := range all {
		if b.Title == alias {
			matches = append(matches, b)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, ""
	}
	if len(matches) > 1 {
		return "", fmt.Sprintf("跳板别名 %q 匹配多个书签，未绑定", alias)
	}
	return "", fmt.Sprintf("未找到跳板别名 %q", alias)
}

func (s *Service) setProxyJumpID(bookmarkID, jumpID string) error {
	b, err := s.db.BookmarkRepo.GetBookmarkByID(bookmarkID)
	if err != nil {
		return err
	}
	if b.ProxyJumpID == jumpID {
		return nil
	}
	b.ProxyJumpID = jumpID
	b.UpdatedAt = time.Now()
	return s.db.BookmarkRepo.UpdateBookmark(b)
}

func (s *Service) ensureGroup(name string) error {
	if _, err := s.db.BookmarkRepo.GetGroupByName(name); err == nil {
		return nil
	}
	return s.AddGroup(name)
}

func (s *Service) uniqueImportTitle(base, groupName string) (string, error) {
	g, err := s.db.BookmarkRepo.GetGroupByName(groupName)
	if err != nil {
		return "", err
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = "host"
	}
	exists, err := s.titleExistsInGroup(base, g.ID)
	if err != nil {
		return "", err
	}
	if !exists {
		return base, nil
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		exists, err := s.titleExistsInGroup(candidate, g.ID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
}

func splitProxyJump(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
