# Vela patch

基于上游 `github.com/go-gui-org/go-term` **v0.19.3**。

升级流程：整目录覆盖为上游版本 → 按下面补丁清单重新打补丁 → 跑焦点/Stream 相关测试 → 手工验证。

## 补丁清单

| # | 文件 | 类型 | 目的 |
| - | ---- | ---- | ---- |
| 1 | `term/stream.go` | 新增文件 | 导出 `Stream` / `NewWithStream`，远程会话（SSH/串口）不建本地 PTY 也能驱动仿真器 |
| 2 | `term/widget_view.go`、`term/widget_state.go`、`term/widget.go` | 修改 | `View()` 只在窗口焦点为空或仍是本 pane 的过期焦点声明时收回焦点，不抢其它控件 |
| 3 | `term/widget_view.go` | 修改 | IME 预编辑只在窗口键盘焦点位于本 pane 时绘制 |
| 4 | `term/widget_test.go` | 测试 | 同步 #2 的语义 |

## 1. term/stream.go（新增）

上游无此文件，升级后保留 / 重新放回即可。

## 2. View() 只在窗口焦点为空或本 pane 过期时收回

`term/widget_view.go` 中，`columnView()` 内上游：

```go
if !w.DialogIsVisible() && w.FocusID() != t.effFocusID {
	w.SetFocus(t.effFocusID)
}
```

改为：先把 `t.effFocusID = w.EffID(t.focusID)` 提到函数开头（IME 门也要用本帧的有效 ID），然后：

```go
focusID := w.FocusID()
if focusID == t.effFocusID {
	t.claimedFocusID = t.effFocusID
} else {
	stale := focusID == t.focusID ||
		(t.claimedFocusID != "" && focusID == t.claimedFocusID)
	if !w.DialogIsVisible() && !contextMenuFocused(w) &&
		(focusID == "" || stale) {
		w.SetFocus(t.effFocusID)
		t.claimedFocusID = t.effFocusID
	}
}
```

配套改动：

- `term/widget_state.go` 新增 `claimedFocusID` 字段，记录本 pane 最近一次成功 `SetFocus` 的有效 ID。
- `term/widget.go` 的 `SetFocused` 在 queued command 里 `SetFocus` 后同步写 `claimedFocusID`。

原因：`View()` 每帧重建，`SetView` 会清空窗口焦点。上游「只要不是我 → 抢回来」会让同窗口的 AI 输入框、表单输入框点进去下一帧就被夺走。本地改成只在两种情况收回：

1. 窗口焦点为空（`SetView` 清掉后需要 pane 重新拿回，保证打字可用）。
2 焦点是本 pane 的过期声明——`New` 写入的裸 leaf（嵌套在带 ID 祖先下时不是有效 ID，go-gui 修复会把它推给第一个 tab stop），或上一次 scope 下 claim 的有效 ID（pane 换 scope 后旧 ID 已死）。

其它控件的有效焦点一律不动。`claimedFocusID` 用来区分「焦点还停在本 pane 旧 scope」和「恰好同名 leaf 的外来控件」。

注意点击终端的聚焦**不是**由 go-gui 的按下事件兜住的：终端画布的 `onClick` 会 `Consume` 按下事件，父级可聚焦容器的 `pressFocuses` 因此走不到。Vela 在 `term.Cfg.OnClickFocus` 里显式 `ResolveID + SetFocus`（见 `ui/app/controller.go`）。该回调属于宿主代码，本库升级不受影响，但不要因为「go-gui 会处理」而删掉它。

`contextMenuFocused(w)` 是补丁的一部分：右键菜单需要键盘焦点，弹窗与右键菜单的豁免必须保留。

## 3. IME 预编辑只在窗口焦点下绘制

`columnView()` 开头，上游：

```go
if t.focused.Load() && w != nil {
	composing = w.IMEComposing()
	compText = w.IMECompText()
	compCursor = w.IMECompCursor()
}
```

改为先解析本帧有效 ID，再用精确匹配判断窗口焦点是否属于本 pane：

```go
focused := t.focused.Load()
if focused && w != nil {
	t.effFocusID = w.EffID(t.focusID)
	if t.windowFocused(w) {
		composing = w.IMEComposing()
		compText = w.IMECompText()
		compCursor = w.IMECompCursor()
	}
}
```

`windowFocused` 不再用 `":"+focusID` 后缀匹配（会误伤同名 leaf 的外来控件），改为：

```go
func (t *Term) windowFocused(w *gui.Window) bool {
	if w == nil {
		return false
	}
	id := w.FocusID()
	return id == t.focusID || id == t.effFocusID || id == t.claimedFocusID
}
```

原因：`t.focused` 是 pane 级状态（`SetFocused`），只表示「这是该 tab 的活动终端」，不代表窗口键盘焦点在它身上。IME 预编辑是窗口级的，AI 输入框或同窗口任何文本框持有焦点时，终端仍然 `focused=true`，会把 `w.IMECompText()` 画到自己的光标处，表现为「终端和输入框同时出现刚打的字」。改为按 `w.FocusID()` 判断；裸 leaf、当前有效 ID、上一次 claim 的有效 ID 都算本 pane。

注意：即使窗口焦点在别处，**不要**顺手把 `colCfg.AcceptsTextInput` 也门控掉——go-gui 只对焦点 shape 激活输入法，这里只决定是否读取并绘制预编辑。

## 4. term/widget_test.go

`TestTerm_View_RestoresFocus`：上游断言「外来焦点会被抢回」，改为断言仍为 `"some-other-widget"`。

## 升级后自检

```bash
rg -n 'claimedFocusID' third_party/go-term/term/widget_view.go third_party/go-term/term/widget_state.go third_party/go-term/term/widget.go
rg -n 'windowFocused' third_party/go-term/term/widget_view.go
rg -n 'NewWithStream' third_party/go-term/term/stream.go
```

手工：连 SSH 后直接打字有输出；AI 面板输入框能输入；点回终端继续可输入；中文输入法在 AI 输入框打字时终端不再出现预编辑；右键菜单 / 弹窗键盘可用。
