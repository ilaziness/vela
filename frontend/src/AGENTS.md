# 前端项目规范

## 组件使用

- AI 聊天使用`MUI X Chat`相关组件来实现，不要自己重复造轮子

## 组件目录

按职责分目录（一次搬家后保持）：

- `session/` — SessionTabs / SessionTabBody / SessionTabText
- `terminal/` — XTerm 终端与搜索/右键/命令提示
- `bookmark/` — 书签树/表单/连接面板
- `sftp/` — SFTP 与传输列表
- `ssh/` — SSH 专属（隧道、hostkey、状态栏监控等）
- `serial/` — 串口专用 UI（若有）
- `command/` — 命令管理
- `tools/` — 工具箱
- `common/` — Header / OpBar / ConnectionForm 等共用
- 已有 `ai/` `settings/` `hostResources/` `subwindow/` `icons/` 不动

## 会话协议

- 类型：`types/session.ts`（`SessionProtocol` / `SessionLink` / `SessionTab`）
- Store：`stores/sessionTabs.ts`（`setSessionLink`）
- SSH 专属 UI/自动重连/AI Exec：仅当 `protocol === SessionProtocol.SSH`
- 连接与书签表单必须显式选协议；禁止空/未知回落为 SSH
