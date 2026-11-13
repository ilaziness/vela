# AGENTS.md

本项目是一个基于 Go 语言与 go-gui / go-term 构建的 SSH 桌面 GUI 应用，旨在为用户提供跨平台、安全、高效的终端连接体验。本文档描述了项目所采用的技术栈、架构规范、开发约定以及各模块职责。

---

## 1. 技术栈概览

| 类别     | 技术/库                     | 版本要求  |
| -------- | --------------------------- | --------- |
| 后端语言 | Go                          | ≥ 1.27.3  |
| 桌面 UI  | go-gui（**禁止**本地定制）  | v0.95.x   |
| 终端控件 | go-term（replace 本地补丁） | v0.17.x   |
| 图表     | go-charts（go-gui 生态）    | v0.30.x   |
| AI 框架  | Genkit                      | ≥ v1.14.0 |

---

## 2. 项目结构规范

```
├── main.go                 # 应用入口（go-gui + embed logo）
├── assets/                 # 应用图标等静态资源（appicon.png）
├── ui/                     # 桌面 UI（禁止 import Wails；壳层不绑死某协议）
│   ├── app/                # 组合根、State、Controller
│   ├── shell/              # 三栏布局：图标轨 / 侧栏 / 主区
│   ├── session/            # Tab、表单宿主、终端面板、状态栏
│   ├── settings/           # 设置窗口（各 panel + State/Deps）
│   ├── vaultprompt/        # 可复用 Vault 解锁对话框（遮罩密码）
│   ├── layout/             # 即时模式布局辅助（Flush 等）
│   ├── fonts/              # 字体枚举
│   ├── protocol/           # 协议插件接口 + Registry
│   │   ├── ssh/            # SSH 表单 / Connector / HostKey / Keyboard
│   │   └── serial/         # 串口表单 / Connector
│   └── bookmarks/          # 书签树占位
├── internal/               # 引擎与基础设施（禁止 import ui）
│   ├── termio/             # Protocol + Session + Hub
│   ├── termstream/         # termio.Session → go-term Stream
│   ├── ssh/                # 连接、会话、proxy resolve、usererr map
│   ├── serial/             # 串口引擎
│   ├── sftp/ / tunnel/     # SFTP / 端口转发
│   ├── bookmark/           # 书签业务（含 ValidateProxy）
│   ├── applog/             # 进程日志
│   ├── config/             # TOML + BootstrapLanguage
│   ├── i18n/               # leaf：embed 三语；禁止 import config
│   ├── usererr/ / secret/ / database / ai / …
├── third_party/go-term/    # go-term 仅此例外：必须本地补丁（NewWithStream 等）
├── build/                  # 图标与平台打包资源（appicon*、windows/icon.ico）
├── data/                   # 内置默认数据
├── sync-backend/           # 同步服务端（独立 go.mod）
├── Makefile
└── go.mod
```

**第三方库定制硬规则**：

- **禁止** 把 `go-gui` 复制到 `third_party/`、`replace` 到本地 fork，或在本仓库维护 go-gui 定制版。后续升级依赖官方发布版本。
- go-gui 公开 API / 组件能力做不到的交互或视觉效果：直接说明「库目前不支持，只能这样」，用公开 API 能做到的折中；**不要**为对齐、样式等去 fork 改库。
- `third_party/go-term` 是唯一允许的 UI 相关本地补丁（上游无远程 Stream API，业务必须补）；不要把「go-term 可以定制」推广到 go-gui。

---

## 3. 编码规范

模块化，可复用，减少重复；能在引擎层做的不要堆在 UI。

分层硬规则：

- `ui/shell`、`ui/session`：只认 `termio` / `termstream` / go-gui；**禁止** import `internal/ssh`、`internal/serial`。
- `ui/protocol`：协议无关接口；具体引擎只出现在 `ui/protocol/<name>`。
- `ui/app`：组合根，注册协议插件、持有 Hub。
- `internal/`：禁止 import `ui`。SSH 引擎只接收已解析的 hop 链（`[]ssh.Endpoint`）。
- `internal/i18n`：leaf，禁止 import `config` / `ui`。
- 禁止包级可变业务状态。允许：`embed`、ldflags（`internal/buildinfo`）、sentinel、常量。
- 日志：`internal/applog.New()` 创建 `*zap.Logger` 并注入。

### 0. 多语言（i18n）

- 翻译唯一源：`internal/i18n/locales/{en,zh-CN,zh-TW}.json`
- 语言启动：`config.BootstrapLanguage`（在 `config.Load` 之后）
- 用户可见错误：优先 `usererr` 的 `code` + `params`；SSH 侧可用 `ssh.MapUserError`

### 1. 日志

- Go 使用注入的 `*zap.Logger`，不要新增包级 `Logger`。
- 错误要落日志，不要只展示在 UI。

### 2. 终端桥接

- go-term 公开 API 仅本地 PTY；本仓库经 `third_party/go-term` 导出 `NewWithStream`。
- `internal/termstream` 把任意 `termio.Session` 接到 Stream（惰性 Start、Resize 行列转序）。

### 3. AI（引擎保留）

- 使用 Genkit；禁止直接调厂商 API。UI 侧 AI 面板后续再接 go-gui。

---

## 4. UI 规范（go-gui）

- **只用** module 依赖的官方 go-gui；禁止 `third_party/go-gui`、禁止对本库做 replace 定制（详见 §2）。
- 主题：`gui.SetTheme(gui.ThemeDark)`（首期不定制）。
- 状态：`gui.State[app.State](w)`；Term 实例放在 Controller，不进即时模式 State。
- 嵌入终端：`term.Cfg{NoWindowHandler: true}`，由壳层转发事件。
- Connect/Test：后台 goroutine + `Window.QueueCommand` 回写。
- 能力缺口：以公开 API 为准；做不到则告知限制并接受现状，不 fork 改库。

---

## 5. 安全与最佳实践

- 敏感信息不持久化明文；Vault + Argon2id / AES-GCM。
- 日志禁止记录密码、私钥内容。
- 连接参数在引擎层校验。

---

## 6. 开发与调试

**重要**：使用这里列出的命令，不要用其他的。

```bash
# 编译（-s -w 去掉符号表/调试信息，减小体积）
go build -trimpath -ldflags "-s -w" -o ./bin/Vela.exe .
# 或
make build

# 修改后验证
go fmt ./...
golangci-lint run
go build -trimpath -ldflags "-s -w" -o ./bin/Vela.exe .
```

---

## 7. 相关文档

- go-gui：[README](https://github.com/go-gui-org/go-gui) / [llms.txt](https://raw.githubusercontent.com/go-gui-org/go-gui/main/llms.txt)
- go-term：[Embedding wiki](https://github.com/go-gui-org/go-term/wiki/Embedding)
- genkit：[get started](https://genkit.dev/docs/go/get-started)
