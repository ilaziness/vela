# AGENTS.md

本项目是一个基于 Go 语言与 go-gui / go-term 构建的 SSH 桌面 GUI 应用，旨在为用户提供跨平台、安全、高效的终端连接体验。本文档描述了项目所采用的技术栈、架构规范、开发约定以及各模块职责。

---

## 1. 技术栈概览

| 类别     | 技术/库                     | 版本要求  |
| -------- | --------------------------- | --------- |
| 后端语言 | Go                          | ≥ 1.27.3  |
| 桌面 UI  | go-gui（**禁止**本地定制）  | v0.97.x   |
| 终端控件 | go-term（replace 本地补丁） | v0.19.x   |
| 图表     | go-charts（go-gui 生态）    | v0.32.x   |
| AI 框架  | Genkit                      | ≥ v1.14.0 |

---

## 2. 项目结构规范

```shell
├── main.go                 # 应用入口（go-gui + embed logo）
├── main_window.go          # 主窗口装配
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
│   ├── bookmarks/          # 书签树 / 编辑器 / 侧栏
│   ├── agent/              # AI Agent 面板（含 model/）
│   ├── backup/             # 备份 / 恢复窗口
│   ├── chrome/             # 自绘标题栏与窗口控制
│   ├── sftp/               # SFTP 文件面板
│   ├── tools/              # 工具箱（codec/net/ops/text/time）
│   ├── transfer/           # 文件传输队列面板
│   ├── tunnel/             # 端口转发面板
│   └── update/             # 应用更新对话框
├── internal/               # 引擎与基础设施（禁止 import ui）
│   ├── termio/             # Protocol + Session + Hub
│   ├── termstream/         # termio.Session → go-term Stream
│   ├── termws/             # 终端 WebSocket 服务
│   ├── termcharset/        # 终端字符集处理
│   ├── ssh/                # 连接、会话、agent forward、proxy resolve、usererr map
│   ├── serial/             # 串口引擎
│   ├── sshconfig/          # ~/.ssh/config 解析 / 写入
│   ├── sshkey/             # SSH 密钥生成与管理
│   ├── sftp/ / tunnel/     # SFTP / 端口转发管理器
│   ├── transfer/           # 传输队列引擎
│   ├── zmodem/             # ZMODEM 检测与收发
│   ├── bookmark/           # 书签业务（含 ValidateProxy、OpenSSH 导入）
│   ├── agent/              # AI Agent 服务（MCP、持久化）
│   ├── ai/                 # Genkit 引擎、历史、checkpoint、event/
│   ├── command/            # 命令片段 / 行捕获
│   ├── keybind/            # 快捷键目录与管理器
│   ├── sync/               # 配置同步（store/：S3 / WebDAV / 自建）
│   ├── updater/            # 应用更新（GitHub release、下载、重启）
│   ├── tools/              # 内置工具集（转换、CIDR、cron、JWT 等）
│   ├── localfs/            # 本地文件系统辅助
│   ├── httpproxy/          # 系统代理探测（平台分文件）
│   ├── system/             # 系统集成（打开路径等）
│   ├── vaultcheck/         # Vault 状态校验
│   ├── applog/             # 进程日志
│   ├── config/             # TOML + BootstrapLanguage + local_state
│   ├── database/           # SQLite 持久化与迁移
│   ├── secret/             # 密钥 / Vault（Argon2id + AES-GCM）
│   ├── i18n/               # leaf：embed 三语；禁止 import config
│   ├── usererr/ / utils/ / buildinfo/   # 错误码 / 小工具 / ldflags 版本
├── third_party/go-term/    # go-term 仅此例外：必须本地补丁（NewWithStream 等）
├── build/                  # 图标与平台打包资源（appicon*、windows/icon.ico）
├── data/commands/          # 内置默认命令片段
├── testdata/               # 测试数据
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

函数和关键代码需要合理注释，注释不要过多的代码逻辑描述，合理说明即可。

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

- 全局日志：`internal/applog` 提供进程级共享 logger，`applog.L()` 返回全局 `*zap.Logger`（`applog.New()` 创建时即安装为全局实例；`L()` 首次调用会懒初始化）。新增需要打日志的代码直接用 `applog.L()`，**不要**再为此新增 `*zap.Logger` 参数注入。
- 已有 `*zap.Logger` 注入签名的调用链保持不变，不做批量替换。
- 不要新增包级 `Logger` 变量（`applog.L()` 是唯一共享入口）。
- 错误要落日志，不要只展示在 UI。
- 开发过程中需要增加一些合理的debug日志来辅助开发调试，错误类的根据具体情况添加warn或者error级别的日志

### 2. 终端桥接

- go-term 公开 API 仅本地 PTY；本仓库经 `third_party/go-term` 导出 `NewWithStream`。
- `internal/termstream` 把任意 `termio.Session` 接到 Stream（惰性 Start、Resize 行列转序）。

### 3. AI（引擎保留）

- 使用 Genkit；禁止直接调厂商 API。UI 侧 AI 面板后续再接 go-gui。

---

## 4. UI 规范（go-gui）

- **只用** module 依赖的官方 go-gui；禁止 `third_party/go-gui`、禁止对本库做 replace 定制（详见 §2）。
- 主题：应用主题由 `general.theme` 在 go-gui 注册主题中选择，`fonts.Apply` 应用并叠加字体覆盖；终端配色由 `terminal.theme` 在 go-term `Default`+`BundledThemes` 中选择，保存后应用到所有终端。
- 状态：`gui.State[app.State](w)`；Term 实例放在 Controller，不进即时模式 State。
- 嵌入终端：`term.Cfg{NoWindowHandler: true}`，由壳层转发事件。
- Connect/Test：后台 goroutine + `Window.QueueCommand` 回写。
- 能力缺口：以公开 API 为准；做不到则告知限制并接受现状，不 fork 改库。
- 禁止使用emoji文字代替`go-gui`图标，图标需要使用`go-gui`库的`Feather`图标。
- UI组件能通用的尽量创建公共组件使用，不要在不同地方写逻辑重复的UI组件
- 改完 UI 不要靠读代码猜效果：按 §6 的「UI 无头验证」渲染出图并量出几何数字，再回报结论。

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

### UI 无头验证（改界面必须出图 + 量几何）

go-gui 自带软件渲染后端，可以在没有窗口、没有事件循环的情况下把任意 View 渲染成 PNG，并读出真实布局几何。用临时 `*_test.go` 驱动，验证完删除，不要提交。

```go
import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func TestShot(t *testing.T) {
	win := gui.NewTestWindow(t, gui.WindowCfg{
		Width: 460, Height: 620,
		// view：要验证的 View 函数（整块面板，或像 messageView 这样的单个组件）
		OnInit: func(w *gui.Window) { w.SetView(view) },
	})
	// 跑一帧并出图：同时装上软件字体系统，布局用的就是真实字体度量
	if err := soft.RenderToPNG(win, 2, "shot.png"); err != nil {
		t.Fatal(err)
	}
	// 量几何：叶子 ID → 有效 ID → Shape 的 x/y/w/h
	root := win.TestRender(nil)
	for _, id := range win.ResolveID("agent-msg-1") {
		if ly, ok := root.FindByID(id); ok {
			t.Logf("%s x=%.1f y=%.1f w=%.1f h=%.1f", id,
				ly.Shape.X, ly.Shape.Y, ly.Shape.Width, ly.Shape.Height)
		}
	}
}
```

- `gui.NewTestWindow` 自己注册 cleanup，不需要 `soft.Release`；窗口没有 backend，`TestClick` / `TestFocus` / `TestScroll` 等动作可以在这里驱动状态后重新出图。
- 中文文案先 `i18n.SetLanguage("zh-CN")`；渲染 AI 面板这类 View 直接构造 `*app.State`（如 `AgentChat`、`AgentExpanded`）传进去，不需要真 Controller。
- 用 2 倍缩放出图（`RenderToPNG(win, 2, …)`）复核观感；判断内边距、高度、对齐要拿 `Shape` 的数字做前后对比，不要肉眼估。
- 字体来自宿主机字体目录，所以**不要**提交 PNG 或像素级 golden；需要长期回归就断言几何与文本。
- 临时脚本与图片用完删掉（给人看的对比图放仓库外或根目录，附在回复里），不要留在 diff 中。

---

## 7. 相关文档

- go-gui：[README](https://github.com/go-gui-org/go-gui) / [llms.txt](https://raw.githubusercontent.com/go-gui-org/go-gui/main/llms.txt)
- go-gui 无头渲染：`go doc github.com/go-gui-org/go-gui/gui/backend/soft`（`RenderToImage` / `RenderToPNG`）与 `go doc github.com/go-gui-org/go-gui/gui.NewTestWindow`
- go-term：[Embedding wiki](https://github.com/go-gui-org/go-term/wiki/Embedding)
- genkit：[get started](https://genkit.dev/docs/go/get-started)
