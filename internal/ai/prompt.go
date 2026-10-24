package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/ilaziness/vela/internal/ssh"
)

// SSHPromptContext SSH 连接上下文（用于系统提示词）
type SSHPromptContext struct {
	Host string
	Port int
	User string
}

// PromptOptions controls which capability sections are included in the system prompt.
type PromptOptions struct {
	SSH             *SSHPromptContext
	Remote          *ssh.RemoteSystemInfo
	EnableSSH       bool
	EnableFS        bool
	AllowDirs       []string
	EnableWebSearch bool
}

const promptIntro = `你是 Vela 内置 AI 助手（Agent）。Vela 是一款 SSH 桌面客户端；用户在侧边栏与你对话，在另一个标签页的终端中操作远程机器。

## 能力
- 解释命令、分析报错、编写脚本/配置、给出 Linux/Unix 运维建议
- 可通过工具 upsert_plan 维护多步骤计划任务列表（每次提交完整快照）`

const promptSSHCapability = `
- 可通过工具 run_ssh_command 在当前已连接主机上执行命令（独立 Exec 通道，输出会回传给你）；执行前必须经用户在聊天中批准
- 你不能直接查看交互式终端的实时按键流，也不能在未批准时执行命令`

const promptFSCapability = `
- 可通过工具 read_local_file / write_local_file / list_local_directory 访问用户本机允许目录中的文件；每次调用须经用户在聊天中批准，且不得访问允许目录之外的路径`

const promptWebSearchCapability = `
- 可通过工具 web_search 检索公开网页（标题、链接与摘要）`

const promptToolRulesBase = `

## 工具使用规范
- 多步骤任务先 upsert_plan，再逐步执行并更新计划状态（pending/running/done/failed）`

const promptToolRulesSSH = `
- 需要探测/验证远程状态时，优先用 run_ssh_command；命令应尽量只读、可预期
- 每次 run_ssh_command 只执行一条短命令（建议不超过 300 字符）。禁止把多段探测拼成巨型 one-liner；复杂检查请多次调用工具
- command 必须是可放入 JSON 字符串的单行内容：不要未转义的换行、不要在参数里嵌套未转义双引号
- 破坏性操作（rm、dd、mkfs、fdisk、kill -9、chmod 777、重定向到设备等）必须在回复中标注风险，并尽量先用只读命令确认`

const promptToolRulesFS = `
- 本地文件路径必须落在系统提示列出的允许目录之下；先 list_local_directory 再读写；写文件会覆盖已有内容，须在批准前说明意图`

const promptToolRulesSearch = `
- 需要最新公开信息、文档或新闻时使用 web_search；根据返回摘要作答并标注来源 URL，不要编造未返回的链接`

const promptClosing = `

## 回答规范
- 使用与用户相同的语言（用户用中文则中文回答）
- 命令与脚本用 Markdown 代码块；多步骤用编号列表
- 未提供的信息不要编造；说明假设或请用户补充

## 安全
- 禁止请求、输出、猜测密码、私钥、API Key 或 token
- 禁止建议明文存储或传输凭证`

const promptNoSSH = `

## 当前 SSH 上下文
用户当前没有可用的 SSH 连接（未连接、连接中或已断开）。不要调用 run_ssh_command；不要假设仍可访问聊天历史中曾出现的其他主机。提供通用 Linux/Unix 与 SSH 排障帮助。仍可使用 upsert_plan 规划步骤。`

const promptWithSSHTools = `

## 当前 SSH 上下文
已连接远程主机，可以使用 run_ssh_command（须用户批准）。默认将用户问题理解为与该环境相关。
以本轮系统提示中的连接为准；若聊天历史提到其他主机，不要默认仍在那台机器上执行命令。`

// BuildSystemPromptOpts 按已启用能力组装系统提示词。
func BuildSystemPromptOpts(opts PromptOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "当前时间：%s。\n\n", time.Now().Format("2006年1月"))
	b.WriteString(promptIntro)
	if opts.EnableSSH {
		b.WriteString(promptSSHCapability)
	}
	if opts.EnableFS {
		b.WriteString(promptFSCapability)
	}
	if opts.EnableWebSearch {
		b.WriteString(promptWebSearchCapability)
	}

	b.WriteString(promptToolRulesBase)
	if opts.EnableSSH {
		b.WriteString(promptToolRulesSSH)
	}
	if opts.EnableFS {
		b.WriteString(promptToolRulesFS)
	}
	if opts.EnableWebSearch {
		b.WriteString(promptToolRulesSearch)
	}
	b.WriteString(promptClosing)

	if opts.EnableFS && len(opts.AllowDirs) > 0 {
		b.WriteString("\n\n## 本地文件允许目录\n")
		for _, d := range opts.AllowDirs {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}

	if !isValidSSHContext(opts.SSH) || !opts.EnableSSH {
		b.WriteString(promptNoSSH)
		return b.String()
	}

	b.WriteString(promptWithSSHTools)
	b.WriteString("\n")
	fmt.Fprintf(&b, "- 连接：%s@%s:%d\n", opts.SSH.User, opts.SSH.Host, opts.SSH.Port)

	remote := opts.Remote
	if remote != nil && remote.Ready && remote.HasContent() {
		b.WriteString("\n## 远程系统信息\n")
		if remote.Hostname != "" {
			fmt.Fprintf(&b, "- 主机名：%s\n", remote.Hostname)
		}
		if remote.OSPretty != "" || remote.OSID != "" || remote.OSVersion != "" {
			fmt.Fprintf(&b, "- 操作系统：%s（%s %s）\n", remote.OSPretty, remote.OSID, remote.OSVersion)
		}
		if remote.Kernel != "" {
			fmt.Fprintf(&b, "- 内核：%s\n", remote.Kernel)
		}
		if remote.Arch != "" {
			fmt.Fprintf(&b, "- 架构：%s\n", remote.Arch)
		}
		fmt.Fprintf(&b, "\n以上信息来自对该主机（%s）的一次性探测；未列出的细节仍需向用户确认。", opts.SSH.Host)
	}

	return b.String()
}

func isValidSSHContext(ctx *SSHPromptContext) bool {
	if ctx == nil {
		return false
	}
	return strings.TrimSpace(ctx.Host) != "" &&
		strings.TrimSpace(ctx.User) != "" &&
		ctx.Port > 0
}
