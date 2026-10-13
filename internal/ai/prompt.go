package ai

import (
	"fmt"
	"strings"

	"github.com/ilaziness/vela/internal/ssh"
)

// SSHPromptContext SSH 连接上下文（用于系统提示词）
type SSHPromptContext struct {
	Host string
	Port int
	User string
}

const promptBase = `你是 Vela 内置 AI 助手（Agent）。Vela 是一款 SSH 桌面客户端；用户在侧边栏与你对话，在另一个标签页的终端中操作远程机器。

## 能力
- 解释命令、分析报错、编写脚本/配置、给出 Linux/Unix 运维建议
- 可通过工具 run_ssh_command 在当前已连接主机上执行命令（独立 Exec 通道，输出会回传给你）；执行前必须经用户在聊天中批准
- 可通过工具 upsert_plan 维护多步骤计划任务列表（每次提交完整快照）
- 你不能直接查看交互式终端的实时按键流，也不能在未批准时执行命令

## 工具使用规范
- 需要探测/验证远程状态时，优先用 run_ssh_command；命令应尽量只读、可预期
- 每次 run_ssh_command 只执行一条短命令（建议不超过 300 字符）。禁止把多段探测拼成巨型 one-liner；复杂检查请多次调用工具
- command 必须是可放入 JSON 字符串的单行内容：不要未转义的换行、不要在参数里嵌套未转义双引号
- 多步骤任务先 upsert_plan，再逐步执行并更新计划状态（pending/running/done/failed）
- 破坏性操作（rm、dd、mkfs、fdisk、kill -9、chmod 777、重定向到设备等）必须在回复中标注风险，并尽量先用只读命令确认

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

// BuildSystemPrompt 构建系统提示词
func BuildSystemPrompt(ctx *SSHPromptContext, remote *ssh.RemoteSystemInfo, sshToolsEnabled bool) string {
	if !isValidSSHContext(ctx) || !sshToolsEnabled {
		return promptBase + promptNoSSH
	}

	var b strings.Builder
	b.WriteString(promptBase)
	b.WriteString(promptWithSSHTools)
	b.WriteString("\n")
	fmt.Fprintf(&b, "- 连接：%s@%s:%d\n", ctx.User, ctx.Host, ctx.Port)

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
		fmt.Fprintf(&b, "\n以上信息来自对该主机（%s）的一次性探测；未列出的细节仍需向用户确认。", ctx.Host)
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
