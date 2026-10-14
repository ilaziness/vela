import { Box, Typography } from '@mui/material';
import { ChatConfirmation } from '@mui/x-chat';
import { useChat, type ChatPartRendererMap } from '@mui/x-chat/headless';
import { planFromToolPart } from './PlanTaskPart';
import { RUN_SSH_COMMAND_TOOL, SSH_TARGET_INPUT_KEY } from '../../func/aiContext';

/** Custom renderers must cover every tool of that part type — returning null hides the built-in widget. */
export const aiPartRenderers: ChatPartRendererMap = {
  'dynamic-tool': ({ part }) => <AgentToolPart part={part as any} />,
  tool: ({ part }) => <AgentToolPart part={part as any} />,
};

function AgentToolPart({ part }: { part: any }) {
  const { addToolApprovalResponse } = useChat();
  const inv = part?.toolInvocation;
  if (!inv) return null;

  if (planFromToolPart(part)) {
    return null;
  }

  const approvalId = String(inv.approvalId || inv.toolCallId || '');
  const title = String(inv.toolName || 'tool');
  const isSSH = title === RUN_SSH_COMMAND_TOOL;
  const command = isSSH ? sshCommandFromInput(inv.input) : '';
  const sshTarget = isSSH ? sshTargetFromInput(inv.input) : '';

  if (inv.state === 'approval-requested') {
    let message: string;
    if (isSSH && command) {
      const hostLine = sshTarget ? `在 ${sshTarget} 执行？` : '批准执行 SSH 命令？';
      message = `${hostLine}\n$ ${command}`;
    } else {
      const inputPreview = formatApprovalInput(inv.input);
      message = inputPreview
        ? `批准调用 ${title}？\n${inputPreview}`
        : `批准调用 ${title}？`;
    }
    return (
      <ChatConfirmation
        message={message}
        confirmLabel="批准"
        cancelLabel="拒绝"
        onConfirm={() => {
          void addToolApprovalResponse({ id: approvalId, approved: true });
        }}
        onCancel={() => {
          void addToolApprovalResponse({
            id: approvalId,
            approved: false,
            reason: 'user denied',
          });
        }}
      />
    );
  }

  return (
    <Box
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        px: 1.25,
        py: 0.75,
        bgcolor: 'background.paper',
      }}
    >
      <Typography variant="caption" color="text.secondary">
        {title} · {toolStateLabel(inv.state)}
        {sshTarget ? ` · ${sshTarget}` : ''}
      </Typography>
      {command ? (
        <Typography variant="body2" sx={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
          $ {command}
        </Typography>
      ) : null}
      {inv.output != null ? (
        <Typography
          variant="body2"
          sx={{
            whiteSpace: 'pre-wrap',
            mt: 0.5,
            wordBreak: 'break-word',
            fontFamily: 'monospace',
            fontSize: '0.75rem',
          }}
        >
          {truncateToolDisplay(formatUnknown(inv.output))}
        </Typography>
      ) : null}
      {inv.errorText ? (
        <Typography variant="body2" color="error" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
          {truncateToolDisplay(String(inv.errorText))}
        </Typography>
      ) : null}
      {inv.approval?.reason ? (
        <Typography variant="caption" color="text.secondary">
          {String(inv.approval.reason)}
        </Typography>
      ) : null}
    </Box>
  );
}

function sshCommandFromInput(input: unknown): string {
  const v = parseJSONValue(input);
  if (v && typeof v === 'object' && 'command' in (v as object)) {
    return String((v as { command?: unknown }).command ?? '');
  }
  return '';
}

function sshTargetFromInput(input: unknown): string {
  const v = parseJSONValue(input);
  if (v && typeof v === 'object' && SSH_TARGET_INPUT_KEY in (v as object)) {
    return String((v as Record<string, unknown>)[SSH_TARGET_INPUT_KEY] ?? '').trim();
  }
  return '';
}

function parseJSONValue(v: unknown): unknown {
  if (typeof v === 'string') {
    try {
      return JSON.parse(v);
    } catch {
      return v;
    }
  }
  return v;
}

/** Max characters shown for tool output/error in the chat card (full result still goes to the model). */
const TOOL_OUTPUT_PREVIEW_CHARS = 600;

function truncateToolDisplay(text: string, maxChars = TOOL_OUTPUT_PREVIEW_CHARS): string {
  if (text.length <= maxChars) return text;
  return `${text.slice(0, maxChars)}…`;
}

function formatUnknown(v: unknown): string {
  const parsed = parseJSONValue(v);
  if (parsed == null) return '';
  // Stream wrapper from Go: { output: <tool result> }
  if (typeof parsed === 'object' && !Array.isArray(parsed)) {
    const keys = Object.keys(parsed as object);
    if (keys.length === 1 && keys[0] === 'output') {
      return formatUnknown((parsed as { output?: unknown }).output);
    }
  }
  // MCP CallToolResult: prefer content[].text
  const mcpText = mcpResultText(parsed);
  if (mcpText) return mcpText;
  if (typeof parsed === 'string') return parsed;
  if (typeof parsed === 'number' || typeof parsed === 'boolean') return String(parsed);
  try {
    return JSON.stringify(parsed, null, 2);
  } catch {
    return String(parsed);
  }
}

/** Extract readable text from MCP CallToolResult-shaped payloads. */
function mcpResultText(v: unknown): string {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return '';
  const content = (v as { content?: unknown }).content;
  if (!Array.isArray(content) || content.length === 0) return '';
  const parts: string[] = [];
  for (const item of content) {
    if (!item || typeof item !== 'object') continue;
    const text = (item as { text?: unknown }).text;
    if (typeof text === 'string' && text.trim()) {
      parts.push(text);
    }
  }
  return parts.join('\n\n');
}

/** Compact JSON for MCP/other tool approval prompts (skip empty objects). */
function formatApprovalInput(input: unknown): string {
  const parsed = parseJSONValue(input);
  if (parsed == null) return '';
  if (typeof parsed === 'string') return parsed.trim();
  if (typeof parsed === 'object' && !Array.isArray(parsed) && Object.keys(parsed as object).length === 0) {
    return '';
  }
  try {
    const text = JSON.stringify(parsed, null, 2);
    return text === '{}' || text === 'null' ? '' : text;
  } catch {
    return String(parsed);
  }
}

function toolStateLabel(state: string): string {
  switch (state) {
    case 'input-streaming':
      return '准备中';
    case 'input-available':
      return '待执行';
    case 'approval-responded':
      return '已批准';
    case 'output-available':
      return '已完成';
    case 'output-error':
      return '失败';
    case 'output-denied':
      return '已拒绝';
    default:
      return state || '';
  }
}
