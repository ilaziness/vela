# Vela patch

基于上游 `github.com/go-gui-org/go-term` **v0.19.1**。

升级流程：整目录覆盖为上游版本 → 按下面 6 项重新打补丁 → 跑焦点/Stream 相关测试 → 手工验证。

## 补丁清单

| # | 文件 | 类型 | 目的 |
| - | ---- | ---- | ---- |
| 1 | `term/stream.go` | 新增文件 | 导出 `Stream` / `NewWithStream`，远程会话（SSH/串口）不建本地 PTY 也能驱动仿真器 |
| 2 | `term/widget.go` | 修改 | `focusID` 改为含 `:` 的绝对 ID |
| 3 | `term/widget.go` | 修改 | `View()` 不再抢占其它控件的焦点 |
| 4 | `term/widget_state.go` | 仅注释 | 记录 #2 的原因，防止被改回裸 ID |
| 5 | `term/widget_test.go` | 测试 | 同步 #3 的语义 |
| 6 | `term/widget.go` | 修改 | IME 预编辑只在窗口键盘焦点位于本 pane 时绘制 |

## 1. term/stream.go（新增）

上游无此文件，升级后保留 / 重新放回即可。

## 2. focusID 必须是绝对 ID

`newWithPTY` 中，上游：

```go
focusID:     "term-" + strconv.FormatUint(seqID, 10),
```

改为：

```go
focusID:     "term-pane:" + strconv.FormatUint(seqID, 10),
```

原因：`SetFocus` 收的是 **effective ID**。裸 ID 会被带 ID 的祖先重新作用域化成 `父:term-N`（如 `term-ctx-<sess>:term-1`），而 `SetFocus("term-1")` 设的裸 ID 在布局里不存在，go-gui 的 `fixupFocusLocked` 每帧把它兜底到 DFS 第一个 tab stop（表现为焦点跑到 `side-panel:qc-refresh`），键盘事件按 ID 精确投递 → 终端收不到按键。含 `:` 的 ID 被 go-gui 视为绝对 ID，不会被再作用域化。

`strconv` 已导入，无 import 变更。

## 3. View() 只在窗口焦点为空时收回

`View()` 中，上游：

```go
if !w.DialogIsVisible() && w.FocusID() != t.focusID {
	w.SetFocus(t.focusID)
}
```

改为：

```go
if !w.DialogIsVisible() && w.FocusID() == "" && !contextMenuFocused(w) {
	w.SetFocus(t.focusID)
}
```

原因：`View()` 每帧重建，`SetView` 会清空窗口焦点。上游「只要不是我 → 抢回来」会让同窗口的 AI 输入框、表单输入框点进去下一帧就被夺走。改成只在没人持焦点时收回。

注意点击终端的聚焦**不是**由 go-gui 的按下事件兜住的：终端画布的 `onClick` 会 `Consume` 按下事件，父级可聚焦容器的 `pressFocuses` 因此走不到。Vela 在 `term.Cfg.OnClickFocus` 里显式 `ResolveID + SetFocus`（见 `ui/app/controller.go`）。该回调属于宿主代码，本库升级不受影响，但不要因为「go-gui 会处理」而删掉它。

`contextMenuFocused(w)` 是原有条件（右键菜单需要键盘焦点），改写时不要丢；若新版上游没有该辅助函数需一并移植。弹窗与右键菜单的豁免必须保留。

## 4. term/widget_state.go（注释）

`focusID` 字段注释说明「含 `:` 是故意的、不要改成裸 ID」，防止后续被清理。

## 5. term/widget_test.go

`TestTerm_View_RestoresFocus`：上游断言「外来焦点会被抢回」，改为断言仍为 `"some-other-widget"`。

## 6. term/widget.go：IME 预编辑只在窗口焦点下绘制

`View()` 中，上游（本库此段可能已随版本变化，逻辑等价）：

```go
if t.focused.Load() && w != nil {
	composing = w.IMEComposing()
	compText = w.IMECompText()
	compCursor = w.IMECompCursor()
}
```

改为：

```go
if t.focused.Load() && w != nil && t.windowFocused(w) {
	composing = w.IMEComposing()
	compText = w.IMECompText()
	compCursor = w.IMECompCursor()
}
```

并新增辅助函数：

```go
func (t *Term) windowFocused(w *gui.Window) bool {
	if w == nil {
		return false
	}
	id := w.FocusID()
	return id == t.focusID || strings.HasSuffix(id, ":"+t.focusID)
}
```

原因：`t.focused` 是 pane 级状态（`SetFocused`），只表示「这是该 tab 的活动终端」，不代表窗口键盘焦点在它身上。IME 预编辑是窗口级的，AI 输入框或同窗口任何文本框持有焦点时，终端仍然 `focused=true`，会把 `w.IMECompText()`（例如拼音 `zai`）画到自己的光标处，表现为「终端和输入框同时出现刚打的字」。改为按 `w.FocusID()` 判断；#2 之后 `focusID` 含 `:` 是绝对 ID，通常精确相等即可，后缀匹配是兼容写法。

注意：即使窗口焦点在别处，**不要**顺手把 `colCfg.AcceptsTextInput` 也门控掉——go-gui 只对焦点 shape 激活输入法，这里只决定是否读取并绘制预编辑。

## 升级后自检

```bash
rg -n 'term-pane:' third_party/go-term/term/widget.go
rg -n 'FocusID\(\) == ""' third_party/go-term/term/widget.go
rg -n 'windowFocused' third_party/go-term/term/widget.go
```

手工：连 SSH 后直接打字有输出；AI 面板输入框能输入；点回终端继续可输入；中文输入法在 AI 输入框打字时终端不再出现预编辑；右键菜单 / 弹窗键盘可用。
