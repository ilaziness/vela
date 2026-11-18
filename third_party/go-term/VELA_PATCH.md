# Vela patch

基于上游 `github.com/go-gui-org/go-term` **v0.18.0**。

升级流程：整目录覆盖为上游版本 → 按下面 5 项重新打补丁 → 跑焦点/Stream 相关测试 → 手工验证。

## 补丁清单

| # | 文件 | 类型 | 目的 |
| - | ---- | ---- | ---- |
| 1 | `term/stream.go` | 新增文件 | 导出 `Stream` / `NewWithStream`，远程会话（SSH/串口）不建本地 PTY 也能驱动仿真器 |
| 2 | `term/widget.go` | 修改 | `focusID` 改为含 `:` 的绝对 ID |
| 3 | `term/widget.go` | 修改 | `View()` 不再抢占其它控件的焦点 |
| 4 | `term/widget_state.go` | 仅注释 | 记录 #2 的原因，防止被改回裸 ID |
| 5 | `term/widget_test.go` | 测试 | 同步 #3 的语义 |

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

原因：`View()` 每帧重建，`SetView` 会清空窗口焦点。上游「只要不是我 → 抢回来」会让同窗口的 AI 输入框、表单输入框点进去下一帧就被夺走。改成只在没人持焦点时收回；用户点击终端由 go-gui 的鼠标按下聚焦处理。`contextMenuFocused(w)` 是原有条件（右键菜单需要键盘焦点），改写时不要丢；若新版上游没有该辅助函数需一并移植。弹窗与右键菜单的豁免必须保留。

## 4. term/widget_state.go（注释）

`focusID` 字段注释说明「含 `:` 是故意的、不要改成裸 ID」，防止后续被清理。

## 5. term/widget_test.go

`TestTerm_View_RestoresFocus`：上游断言「外来焦点会被抢回」，改为断言仍为 `"some-other-widget"`。

## 升级后自检

```bash
rg -n 'term-pane:' third_party/go-term/term/widget.go
rg -n 'FocusID\(\) == ""' third_party/go-term/term/widget.go
```

手工：连 SSH 后直接打字有输出；AI 面板输入框能输入；点回终端继续可输入；右键菜单 / 弹窗键盘可用。
