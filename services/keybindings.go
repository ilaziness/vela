package services

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

const (
	EventShortcut    = "eventShortcut"
	EventKeybindings = "eventKeybindings"
)

const (
	ShortcutTerminalFind         = "terminal.find"
	ShortcutTerminalCopy         = "terminal.copy"
	ShortcutTerminalPaste        = "terminal.paste"
	ShortcutTerminalAddToChat    = "terminal.addToChat"
	ShortcutTerminalToggleLog    = "terminal.toggleLog"
	ShortcutTerminalClear        = "terminal.clear"
	ShortcutPanelAI              = "panel.ai"
	ShortcutPanelBookmarks       = "panel.bookmarks"
	ShortcutPanelBookmarkConnect = "panel.bookmarkConnect"
	ShortcutPanelCommand         = "panel.command"
	ShortcutPanelTools           = "panel.tools"
	ShortcutPanelSettings        = "panel.settings"
)

const (
	CategoryTerminal = "keybinding.category.terminal"
	CategoryPanel    = "keybinding.category.panel"
)

type ShortcutEvent struct {
	Action string `json:"action"`
}

// KeyBindingInfo 快捷键说明，供设置页与前端提示展示。
type KeyBindingInfo struct {
	Action      string `json:"action"`
	Keys        string `json:"keys"`
	Logical     string `json:"logical"`
	DefaultKeys string `json:"defaultKeys"`
	Custom      bool   `json:"custom"`
	Label       string `json:"label"`
	Category    string `json:"category"`
}

type bindingDef struct {
	action      string
	logical     string // CmdOrCtrl[+Shift]+Key 或 CmdOrCtrl+,
	labelKey    string
	categoryKey string
	toggle      func(*Windows) // 非空则 Go 侧 Toggle；空则向聚焦窗口派发 action
}

func init() {
	application.RegisterEvent[ShortcutEvent](EventShortcut)
	application.RegisterEvent[struct{}](EventKeybindings)
}

func builtinBindings() []bindingDef {
	return []bindingDef{
		// 终端：复制/粘贴沿用常见终端约定；查找/设置沿用编辑器约定。
		{action: ShortcutTerminalFind, logical: "CmdOrCtrl+F", labelKey: "keybinding.terminal.find", categoryKey: CategoryTerminal},
		{action: ShortcutTerminalCopy, logical: "CmdOrCtrl+Shift+C", labelKey: "keybinding.terminal.copy", categoryKey: CategoryTerminal},
		{action: ShortcutTerminalPaste, logical: "CmdOrCtrl+Shift+V", labelKey: "keybinding.terminal.paste", categoryKey: CategoryTerminal},
		{action: ShortcutTerminalAddToChat, logical: "CmdOrCtrl+Shift+L", labelKey: "keybinding.terminal.addToChat", categoryKey: CategoryTerminal},
		// 不用 Ctrl+Shift+R（浏览器强刷）；不用 Ctrl+Alt+*（欧陆键盘 AltGr=Ctrl+Alt）。
		{action: ShortcutTerminalToggleLog, logical: "CmdOrCtrl+Shift+Y", labelKey: "keybinding.terminal.toggleLog", categoryKey: CategoryTerminal},
		{action: ShortcutTerminalClear, logical: "CmdOrCtrl+Shift+K", labelKey: "keybinding.terminal.clear", categoryKey: CategoryTerminal},
		{action: ShortcutPanelAI, logical: "CmdOrCtrl+Shift+A", labelKey: "keybinding.panel.ai", categoryKey: CategoryPanel},
		{action: ShortcutPanelBookmarkConnect, logical: "CmdOrCtrl+B", labelKey: "keybinding.panel.bookmarkConnect", categoryKey: CategoryPanel},
		{action: ShortcutPanelBookmarks, logical: "CmdOrCtrl+Shift+B", labelKey: "keybinding.panel.bookmarks", categoryKey: CategoryPanel},
		{action: ShortcutPanelCommand, logical: "CmdOrCtrl+Shift+P", labelKey: "keybinding.panel.command", categoryKey: CategoryPanel, toggle: (*Windows).ToggleCommand},
		// 不用 Ctrl+Shift+T：与浏览器「恢复关闭的标签」冲突。
		{action: ShortcutPanelTools, logical: "CmdOrCtrl+Shift+U", labelKey: "keybinding.panel.tools", categoryKey: CategoryPanel, toggle: (*Windows).ToggleTool},
		{action: ShortcutPanelSettings, logical: "CmdOrCtrl+,", labelKey: "keybinding.panel.settings", categoryKey: CategoryPanel, toggle: (*Windows).ToggleSetting},
	}
}

func defByAction() map[string]bindingDef {
	defs := builtinBindings()
	out := make(map[string]bindingDef, len(defs))
	for _, d := range defs {
		out[d.action] = d
	}
	return out
}

// KeyBindingManager 合并默认与配置覆盖，并支持运行时改绑。
type KeyBindingManager struct {
	app       *application.App
	windows   *Windows
	store     *config.Store
	logger    *zap.Logger
	mu        sync.Mutex
	current   map[string]string // action -> effective logical
	defs      map[string]bindingDef
	suspended bool
}

func NewKeyBindingManager(app *application.App, windows *Windows, store *config.Store, logger *zap.Logger) *KeyBindingManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	m := &KeyBindingManager{
		app:     app,
		windows: windows,
		store:   store,
		logger:  logger,
		current: make(map[string]string),
		defs:    defByAction(),
	}
	return m
}

// Register 按 defaults⊕overrides 注册全部快捷键。
func (m *KeyBindingManager) Register() {
	m.mu.Lock()
	defer m.mu.Unlock()

	overrides := m.sanitizeOverridesLocked(m.storeOverrides())
	m.persistOverridesLocked(overrides)

	for _, d := range builtinBindings() {
		logical := d.logical
		if ov, ok := overrides[d.action]; ok && ov != "" {
			logical = ov
		}
		m.current[d.action] = logical
		m.addBindingLocked(d.action, logical)
	}
}

func (m *KeyBindingManager) storeOverrides() map[string]string {
	if m.store == nil || m.store.Config == nil || m.store.Config.Keybindings == nil {
		return nil
	}
	return m.store.Config.Keybindings
}

func (m *KeyBindingManager) sanitizeOverridesLocked(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return nil
	}

	// Pass 1: validate / canonicalize candidates (deterministic action order).
	type candidate struct {
		action string
		canon  string
		norm   string
	}
	var candidates []candidate
	actions := make([]string, 0, len(raw))
	for action := range raw {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	for _, action := range actions {
		logical := strings.TrimSpace(raw[action])
		if logical == "" {
			continue
		}
		def, ok := m.defs[action]
		if !ok {
			m.logger.Warn("keybinding override: unknown action", zap.String("action", action))
			continue
		}
		if err := validateLogical(logical); err != nil {
			m.logger.Warn("keybinding override: invalid", zap.String("action", action), zap.Error(err))
			continue
		}
		canon := canonicalizeLogical(logical)
		if normalizeAccel(canon) == normalizeAccel(def.logical) {
			continue // redundant
		}
		candidates = append(candidates, candidate{
			action: action,
			canon:  canon,
			norm:   normalizeAccel(canon),
		})
	}

	// Pass 2: start from defaults, free slots for every candidate action, then assign.
	used := make(map[string]string) // normalizeAccel -> action
	for _, d := range builtinBindings() {
		used[normalizeAccel(d.logical)] = d.action
	}
	overridden := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		overridden[c.action] = struct{}{}
	}
	for action := range overridden {
		defNorm := normalizeAccel(m.defs[action].logical)
		if owner, ok := used[defNorm]; ok && owner == action {
			delete(used, defNorm)
		}
	}

	clean := make(map[string]string)
	for _, c := range candidates {
		if other, clash := used[c.norm]; clash && other != c.action {
			m.logger.Warn("keybinding override: conflict",
				zap.String("action", c.action),
				zap.String("logical", c.canon),
				zap.String("conflictWith", other),
			)
			continue
		}
		used[c.norm] = c.action
		clean[c.action] = c.canon
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func (m *KeyBindingManager) persistOverridesLocked(overrides map[string]string) {
	if m.store == nil || m.store.Config == nil {
		return
	}
	prev := m.store.Config.Keybindings
	if mapsEqualString(prev, overrides) {
		return
	}
	m.store.Config.Keybindings = overrides
	if err := m.store.Save(); err != nil {
		m.logger.Error("save sanitized keybindings failed", zap.Error(err))
	}
}

func mapsEqualString(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (m *KeyBindingManager) List() []KeyBindingInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	defs := builtinBindings()
	out := make([]KeyBindingInfo, 0, len(defs))
	for _, d := range defs {
		logical := m.current[d.action]
		if logical == "" {
			logical = d.logical
		}
		custom := normalizeAccel(logical) != normalizeAccel(d.logical)
		out = append(out, KeyBindingInfo{
			Action:      d.action,
			Keys:        displayKeys(logical),
			Logical:     logical,
			DefaultKeys: displayKeys(d.logical),
			Custom:      custom,
			Label:       i18n.T(d.labelKey),
			Category:    i18n.T(d.categoryKey),
		})
	}
	return out
}

func (m *KeyBindingManager) Suspend() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.suspended = true
}

func (m *KeyBindingManager) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.suspended = false
}

func (m *KeyBindingManager) Set(action, logical string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	def, ok := m.defs[action]
	if !ok {
		return usererr.Coded("keybinding.unknown_action", "Unknown shortcut action: %s", action)
	}

	logical = strings.TrimSpace(logical)
	oldLogical := m.current[action]
	if oldLogical == "" {
		oldLogical = def.logical
	}

	var newLogical string
	clearOverride := logical == ""
	if clearOverride {
		newLogical = def.logical
	} else {
		if err := validateLogical(logical); err != nil {
			return err
		}
		newLogical = canonicalizeLogical(logical)
		if normalizeAccel(newLogical) == normalizeAccel(def.logical) {
			clearOverride = true
			newLogical = def.logical
		}
	}

	if normalizeAccel(newLogical) == normalizeAccel(oldLogical) {
		// no-op; still clear redundant override if needed
		if clearOverride {
			return m.writeOverrideLocked(action, "")
		}
		return nil
	}

	newNorm := normalizeAccel(newLogical)
	for act, cur := range m.current {
		if act == action {
			continue
		}
		if normalizeAccel(cur) == newNorm {
			label := act
			if d, ok := m.defs[act]; ok {
				label = i18n.T(d.labelKey)
			}
			return usererr.Coded("keybinding.conflict", "Shortcut conflicts with \"%s\"", label)
		}
	}

	m.removeBindingLocked(oldLogical)
	m.addBindingLocked(action, newLogical)
	// Wails Add 无错误返回；失败时尽量恢复旧绑定。
	m.current[action] = newLogical

	var persistErr error
	if clearOverride {
		persistErr = m.writeOverrideLocked(action, "")
	} else {
		persistErr = m.writeOverrideLocked(action, newLogical)
	}
	if persistErr != nil {
		m.removeBindingLocked(newLogical)
		m.addBindingLocked(action, oldLogical)
		m.current[action] = oldLogical
		return persistErr
	}
	m.emitChangedLocked()
	return nil
}

func (m *KeyBindingManager) ResetAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先落盘，避免运行时已恢复默认但配置仍保留覆盖。
	if m.store != nil && m.store.Config != nil {
		prev := m.store.Config.Keybindings
		m.store.Config.Keybindings = nil
		if err := m.store.Save(); err != nil {
			m.store.Config.Keybindings = prev
			return usererr.Coded("keybinding.save_failed", "Failed to save shortcuts: %s", err.Error())
		}
	}

	for action, cur := range m.current {
		defLogical := m.defs[action].logical
		if normalizeAccel(cur) == normalizeAccel(defLogical) {
			continue
		}
		m.removeBindingLocked(cur)
		m.addBindingLocked(action, defLogical)
		m.current[action] = defLogical
	}
	m.emitChangedLocked()
	return nil
}

func (m *KeyBindingManager) writeOverrideLocked(action, logical string) error {
	if m.store == nil || m.store.Config == nil {
		return nil
	}
	prev := m.store.Config.Keybindings
	kb := make(map[string]string, len(prev)+1)
	for k, v := range prev {
		kb[k] = v
	}
	if logical == "" {
		delete(kb, action)
	} else {
		kb[action] = logical
	}
	var next map[string]string
	if len(kb) > 0 {
		next = kb
	}
	if mapsEqualString(prev, next) {
		return nil
	}
	m.store.Config.Keybindings = next
	if err := m.store.Save(); err != nil {
		m.store.Config.Keybindings = prev
		m.logger.Error("save keybindings failed", zap.Error(err))
		return usererr.Coded("keybinding.save_failed", "Failed to save shortcuts: %s", err.Error())
	}
	return nil
}

func (m *KeyBindingManager) emitChangedLocked() {
	if m.app == nil {
		return
	}
	m.app.Event.Emit(EventKeybindings, struct{}{})
}

func (m *KeyBindingManager) addBindingLocked(action, logical string) {
	def := m.defs[action]
	cb := m.makeCallback(action, def.toggle)
	addKeyBinding(m.app, logical, cb)
}

func (m *KeyBindingManager) removeBindingLocked(logical string) {
	removeKeyBinding(m.app, logical)
}

func (m *KeyBindingManager) makeCallback(action string, toggle func(*Windows)) func(application.Window) {
	return func(window application.Window) {
		m.mu.Lock()
		suspended := m.suspended
		m.mu.Unlock()
		if suspended {
			return
		}
		if toggle != nil {
			// 必须离开 AcceleratorKeyPressed 回调后再建窗：同步 NewWithOptions
			// 会在 WebView2 COM 回调栈上嵌套初始化，导致卡住并超时退出。
			go func() {
				application.InvokeAsync(func() {
					m.mu.Lock()
					still := m.suspended
					m.mu.Unlock()
					if still {
						return
					}
					toggle(m.windows)
				})
			}()
			return
		}
		window.DispatchWailsEvent(&application.CustomEvent{
			Name: EventShortcut,
			Data: ShortcutEvent{Action: action},
		})
	}
}

func addKeyBinding(app *application.App, logical string, cb func(window application.Window)) {
	if app == nil {
		return
	}
	accel := normalizeAccel(logical)
	app.KeyBinding.Add(accel, cb)
	for _, alias := range accelAliases(accel) {
		app.KeyBinding.Add(alias, cb)
	}
}

func removeKeyBinding(app *application.App, logical string) {
	if app == nil {
		return
	}
	accel := normalizeAccel(logical)
	app.KeyBinding.Remove(accel)
	for _, alias := range accelAliases(accel) {
		app.KeyBinding.Remove(alias)
	}
}

func accelAliases(accel string) []string {
	if runtime.GOOS != "windows" || !strings.HasSuffix(accel, "+,") {
		return nil
	}
	return []string{strings.TrimSuffix(accel, ",") + "OEM_COMMA"}
}

// canonicalizeLogical 将平台主键修饰统一为 CmdOrCtrl，便于跨平台存储。
func canonicalizeLogical(logical string) string {
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	hasPrimary := false
	hasShift := false
	hasAlt := false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "ctrl", "control":
			hasPrimary = true
		case "shift":
			hasShift = true
		case "alt", "option":
			hasAlt = true
		}
	}
	named := map[string]string{
		",": ",", "f1": "F1", "f2": "F2", "f3": "F3", "f4": "F4",
		"f5": "F5", "f6": "F6", "f7": "F7", "f8": "F8", "f9": "F9",
		"f10": "F10", "f11": "F11", "f12": "F12",
		"escape": "Escape", "enter": "Enter", "space": "Space",
		"tab": "Tab", "backspace": "Backspace", "delete": "Delete",
		"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	}
	if v, ok := named[strings.ToLower(key)]; ok {
		key = v
	} else {
		key = strings.ToUpper(key)
	}
	var out []string
	if hasPrimary {
		out = append(out, "CmdOrCtrl")
	}
	if hasShift {
		out = append(out, "Shift")
	}
	if hasAlt {
		out = append(out, "Alt")
	}
	if len(out) == 0 {
		return key
	}
	return strings.Join(append(out, key), "+")
}

func isNamedKey(key string) bool {
	k := strings.ToLower(key)
	switch k {
	case ",", "escape", "enter", "space", "tab", "backspace", "delete",
		"up", "down", "left", "right",
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
		return true
	}
	return false
}

func validateLogical(logical string) error {
	logical = strings.TrimSpace(logical)
	if logical == "" {
		return usererr.Coded("keybinding.empty", "Shortcut cannot be empty")
	}
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return usererr.Coded("keybinding.invalid", "Invalid shortcut")
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if key == "" {
		return usererr.Coded("keybinding.invalid", "Invalid shortcut")
	}
	hasPrimary := false
	hasAlt := false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "ctrl", "control":
			hasPrimary = true
		case "alt", "option":
			hasAlt = true
		case "shift":
			// Shift alone is not enough; tracked only so the chord parses.
		default:
			return usererr.Coded("keybinding.unsupported_modifier", "Unsupported modifier: %s", p)
		}
	}
	isFunc := len(key) >= 2 && (key[0] == 'F' || key[0] == 'f')
	if isFunc {
		rest := key[1:]
		if rest != "" {
			allDigit := true
			for _, r := range rest {
				if !unicode.IsDigit(r) {
					allDigit = false
					break
				}
			}
			if allDigit {
				return nil // F-keys allowed without modifier
			}
		}
	}
	if isNamedKey(key) && strings.EqualFold(key, "Escape") {
		return usererr.Coded("keybinding.no_escape", "Cannot bind Escape")
	}
	// 禁止仅 Shift+字母（会抢走普通输入）；需 Ctrl/Cmd、Alt 或功能键。
	if !hasPrimary && !hasAlt && !isFunc {
		return usererr.Coded("keybinding.need_modifier", "Shortcut must include Ctrl/Cmd or Alt, or be a function key")
	}
	if len(key) == 1 {
		r := rune(key[0])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || key == "," {
			return nil
		}
	}
	if isNamedKey(key) {
		return nil
	}
	// allow single printable after mods
	if (hasPrimary || hasAlt) && len(key) >= 1 {
		return nil
	}
	return usererr.Coded("keybinding.unsupported_key", "Unsupported key: %s", key)
}

// normalizeAccel 将逻辑加速键转为 KeyBinding.Add 可用的平台字符串。
// 修饰键顺序与 Wails accelerator.String() 一致：字母序 + 主键。
func normalizeAccel(logical string) string {
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := parts[len(parts)-1]
	modSet := map[string]struct{}{}
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "cmd", "command":
			if runtime.GOOS == "darwin" {
				modSet["Cmd"] = struct{}{}
			} else {
				modSet["Ctrl"] = struct{}{}
			}
		case "ctrl", "control":
			modSet["Ctrl"] = struct{}{}
		case "shift":
			modSet["Shift"] = struct{}{}
		case "alt", "option":
			if runtime.GOOS == "darwin" {
				modSet["Option"] = struct{}{}
			} else {
				modSet["Alt"] = struct{}{}
			}
		}
	}
	order := []string{"Alt", "Cmd", "Ctrl", "Option", "Shift", "Win"}
	var mods []string
	for _, m := range order {
		if _, ok := modSet[m]; ok {
			mods = append(mods, m)
		}
	}
	key = strings.ToUpper(key)
	if len(mods) == 0 {
		return key
	}
	return strings.Join(append(mods, key), "+")
}

func displayKeys(logical string) string {
	isMac := runtime.GOOS == "darwin"
	parts := strings.Split(logical, "+")
	if len(parts) == 0 {
		return logical
	}
	key := parts[len(parts)-1]
	var out []string
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "cmdorctrl", "cmd", "command":
			if isMac {
				out = append(out, "⌘")
			} else {
				out = append(out, "Ctrl")
			}
		case "ctrl", "control":
			if isMac {
				out = append(out, "⌃")
			} else {
				out = append(out, "Ctrl")
			}
		case "shift":
			if isMac {
				out = append(out, "⇧")
			} else {
				out = append(out, "Shift")
			}
		case "alt", "option":
			if isMac {
				out = append(out, "⌥")
			} else {
				out = append(out, "Alt")
			}
		default:
			out = append(out, p)
		}
	}
	if isMac {
		return strings.Join(append(out, strings.ToUpper(key)), "")
	}
	return strings.Join(append(out, strings.ToUpper(key)), "+")
}
