// Genkit Adapter - 将 MUI X Chat 连接到后端 AIService（Agent 流）
import type { ChatAdapter, ChatMessageChunk, ChatStreamEnvelope, ChatUser } from '@mui/x-chat/headless';
import { Events } from '@wailsio/runtime';
import { AIService } from '../../../bindings/github.com/ilaziness/vela/services';
import {
  ChatRequest,
  AIMessage,
  ToolApprovalRequest,
  ChatAttachment,
} from '../../../bindings/github.com/ilaziness/vela/services/models';
import { formatAIChatError } from '../../func/aiChatError';
import { parseCallServiceError } from '../../func/service';
import {
  getCurrentSSHBindState,
  isKnownChatTool,
  sanitizeToolInputForDisplay,
} from '../../func/aiContext';
import { fileToDataURL, localFilePath } from '../../func/aiAttachments';
import { useMessageStore } from '../../stores/message';
import { useAIAssistantStore } from '../../stores/aiAssistant';

const EVENT_AI_STREAM_CHUNK = 'eventAIStreamChunk';

function toolPartType(toolName: string): 'tool' | 'dynamic-tool' {
  return isKnownChatTool(toolName) ? 'tool' : 'dynamic-tool';
}

function isToolMessagePart(part: { type?: string } | null | undefined): boolean {
  return part?.type === 'tool' || part?.type === 'dynamic-tool';
}

export const currentUser: ChatUser = {
  id: 'user',
  displayName: 'You',
};

export const aiUser: ChatUser = {
  id: 'assistant',
  displayName: 'AI',
};

type SendMessageInput = Parameters<ChatAdapter['sendMessage']>[0];
type ListMessagesInput = Parameters<NonNullable<ChatAdapter['listMessages']>>[0];
type ListMessagesResult = Awaited<ReturnType<NonNullable<ChatAdapter['listMessages']>>>;
type AddToolApprovalInput = Parameters<NonNullable<ChatAdapter['addToolApprovalResponse']>>[0];
type MessageParts = ListMessagesResult['messages'][number]['parts'];

interface StreamEventData {
  sessionId?: string;
  type?: string;
  id?: string;
  delta?: string;
  chunk?: string;
  toolCallId?: string;
  toolName?: string;
  approvalId?: string;
  input?: unknown;
  output?: unknown;
  errorText?: string;
  reason?: string;
}

export class GenkitAdapter implements ChatAdapter {
  private activeSessionId = '';

  async sendMessage(input: SendMessageInput): Promise<ReadableStream<ChatMessageChunk | ChatStreamEnvelope>> {
    const { signal } = input;
    const textPart = input.message.parts.find((part) => part.type === 'text');
    const newMessage = (textPart?.text || '').trim();
    if (!newMessage) {
      const message = '消息内容不能为空';
      useMessageStore.getState().errorMessage(message);
      throw new Error(message);
    }

    const sessionId = input.conversationId || '';
    if (!sessionId) {
      const message = '会话未就绪，请稍后重试';
      useMessageStore.getState().errorMessage(message);
      throw new Error(message);
    }

    this.activeSessionId = sessionId;

    let chatPromise: ReturnType<typeof AIService.Chat>;
    let messageId: string;
    const capturedSessionId = sessionId;
    const runID = `run-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;

    try {
      const draftAtts = input.attachments ?? [];
      const attachments: ChatAttachment[] = [];
      for (const att of draftAtts) {
        const path = localFilePath(att.file);
        if (path) {
          attachments.push(
            new ChatAttachment({
              path,
              filename: att.file.name || undefined,
              mime: att.file.type || undefined,
            }),
          );
          continue;
        }
        const dataURL = await fileToDataURL(att.file);
        attachments.push(
          new ChatAttachment({
            data_url: dataURL,
            filename: att.file.name || undefined,
            mime: att.file.type || undefined,
          }),
        );
      }

      const bind = getCurrentSSHBindState();
      const request = new ChatRequest({
        session_id: sessionId,
        run_id: runID,
        new_message: newMessage,
        ...(attachments.length > 0 ? { attachments } : {}),
        ...(bind.context ? { ssh_context: bind.context } : {}),
      });

      messageId = `msg-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
      chatPromise = AIService.Chat(request);
    } catch (err) {
      const message = parseCallServiceError(err);
      useMessageStore.getState().errorMessage(message);
      throw new Error(message, { cause: err });
    }

    return new ReadableStream({
      start(controller) {
        const reasoningId = `reasoning-${messageId}`;
        const textId = `text-${messageId}`;
        let hasReasoningStarted = false;
        let hasTextStarted = false;
        let hasStarted = false;
        let aborted = false;

        const endStreaming = () => {
          useAIAssistantStore.getState().setStreaming(false);
        };

        const ensureStarted = () => {
          if (!hasStarted) {
            hasStarted = true;
            useAIAssistantStore.getState().setStreaming(true);
            controller.enqueue({ type: 'start', messageId, author: aiUser } as ChatMessageChunk);
          }
        };

        const closeOpenTextParts = () => {
          try {
            if (hasReasoningStarted) {
              controller.enqueue({ type: 'reasoning-end', id: reasoningId } as ChatMessageChunk);
            }
            if (hasTextStarted) {
              controller.enqueue({ type: 'text-end', id: textId } as ChatMessageChunk);
            }
          } catch {
            // stream already closed
          }
          hasReasoningStarted = false;
          hasTextStarted = false;
        };

        const enqueue = (chunk: ChatMessageChunk) => {
          if (aborted) return;
          try {
            controller.enqueue(chunk);
          } catch {
            aborted = true;
          }
        };

        const mapEvent = (data: StreamEventData) => {
          ensureStarted();
          const type = data.type || '';

          switch (type) {
            case 'start-step':
              closeOpenTextParts();
              enqueue({ type: 'start-step' });
              return;
            case 'finish-step':
              closeOpenTextParts();
              enqueue({ type: 'finish-step' });
              return;
            case 'reasoning-delta':
            case 'reasoning': {
              const delta = data.delta || data.chunk || '';
              if (!hasReasoningStarted) {
                hasReasoningStarted = true;
                enqueue({ type: 'reasoning-start', id: reasoningId });
              }
              if (delta) {
                enqueue({ type: 'reasoning-delta', id: reasoningId, delta });
              }
              return;
            }
            case 'text-delta':
            case 'text': {
              const delta = data.delta || data.chunk || '';
              if (!hasTextStarted) {
                hasTextStarted = true;
                enqueue({ type: 'text-start', id: textId });
              }
              if (delta) {
                enqueue({ type: 'text-delta', id: textId, delta });
              }
              return;
            }
            case 'tool-input-start': {
              closeOpenTextParts();
              const toolName = data.toolName || '';
              enqueue({
                type: 'tool-input-start',
                toolCallId: data.toolCallId || '',
                toolName,
                ...(isKnownChatTool(toolName) ? {} : { dynamic: true }),
              } as ChatMessageChunk);
              return;
            }
            case 'tool-input-available': {
              const toolName = data.toolName || '';
              enqueue({
                type: 'tool-input-available',
                toolCallId: data.toolCallId || '',
                toolName,
                input: sanitizeToolInputForDisplay(toolName, parseJSONValue(data.input) ?? {}),
                ...(isKnownChatTool(toolName) ? {} : { dynamic: true }),
              } as ChatMessageChunk);
              return;
            }
            case 'tool-approval-request': {
              const toolName = data.toolName || '';
              enqueue({
                type: 'tool-approval-request',
                toolCallId: data.toolCallId || '',
                toolName,
                approvalId: data.approvalId || data.toolCallId,
                input: sanitizeToolInputForDisplay(toolName, parseJSONValue(data.input) ?? {}),
                ...(isKnownChatTool(toolName) ? {} : { dynamic: true }),
              } as ChatMessageChunk);
              return;
            }
            case 'tool-output-available':
              enqueue({
                type: 'tool-output-available',
                toolCallId: data.toolCallId || '',
                output: parseJSONValue(data.output) ?? {},
              } as ChatMessageChunk);
              return;
            case 'tool-output-error':
              enqueue({
                type: 'tool-output-error',
                toolCallId: data.toolCallId || '',
                errorText: data.errorText || 'tool error',
              } as ChatMessageChunk);
              return;
            case 'tool-output-denied':
              enqueue({
                type: 'tool-output-denied',
                toolCallId: data.toolCallId || '',
                approvalId: data.approvalId || data.toolCallId,
                reason: data.reason,
              } as ChatMessageChunk);
              return;
            default:
              return;
          }
        };

        const unsubscribe = Events.On(EVENT_AI_STREAM_CHUNK, (event: any) => {
          if (aborted) return;
          if (useAIAssistantStore.getState().activeSessionId !== capturedSessionId) return;
          const data = event.data as StreamEventData;
          if (data?.sessionId !== capturedSessionId) return;
          mapEvent(data);
        });

        let chatDone = false;

        const closeAsAborted = () => {
          if (aborted) return;
          aborted = true;
          unsubscribe();
          signal.removeEventListener('abort', onAbort);
          try {
            ensureStarted();
            closeOpenTextParts();
            controller.enqueue({ type: 'abort', messageId } as ChatMessageChunk);
            controller.close();
          } catch {
            // stream already closed
          }
          endStreaming();
        };

        const onAbort = () => {
          if (aborted) return;
          // Chat 已结束后再 CancelChat 会把 cancelled 标记写回去并泄漏。
          if (!chatDone) {
            void AIService.CancelChat(capturedSessionId, runID).catch(() => {});
          }
          closeAsAborted();
        };
        signal.addEventListener('abort', onAbort);
        if (signal.aborted) {
          onAbort();
          return;
        }

        chatPromise
          .then(() => {
            chatDone = true;
            if (aborted) return;
            unsubscribe();
            signal.removeEventListener('abort', onAbort);
            if (useAIAssistantStore.getState().activeSessionId !== capturedSessionId) {
              endStreaming();
              controller.close();
              return;
            }
            ensureStarted();
            closeOpenTextParts();
            enqueue({ type: 'finish', messageId });
            endStreaming();
            controller.close();
          })
          .catch((err: any) => {
            chatDone = true;
            if (aborted) return;
            const errText = parseCallServiceError(err);
            if (signal.aborted || /generation stopped/i.test(errText)) {
              closeAsAborted();
              return;
            }
            unsubscribe();
            signal.removeEventListener('abort', onAbort);
            if (useAIAssistantStore.getState().activeSessionId !== capturedSessionId) {
              endStreaming();
              controller.close();
              return;
            }
            ensureStarted();
            const message = formatAIChatError(errText);
            useMessageStore.getState().errorMessage(message);
            closeOpenTextParts();
            enqueue({ type: 'text-start', id: textId });
            enqueue({ type: 'text-delta', id: textId, delta: message });
            enqueue({ type: 'text-end', id: textId });
            enqueue({ type: 'finish', messageId });
            endStreaming();
            controller.close();
          });
      },
    });
  }

  async listMessages(input: ListMessagesInput): Promise<ListMessagesResult> {
    try {
      const msgs = await AIService.ListMessages(input.conversationId);
      const rows = (msgs || []).filter((m): m is AIMessage => m !== null);
      return { messages: genkitRowsToMui(rows) };
    } catch (err) {
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
      return { messages: [] };
    }
  }

  async addToolApprovalResponse(input: AddToolApprovalInput): Promise<void> {
    const sessionId = this.activeSessionId || useAIAssistantStore.getState().activeSessionId || '';
    if (!sessionId) {
      throw new Error('会话未就绪');
    }
    try {
      await AIService.RespondToolApproval(
        new ToolApprovalRequest({
          session_id: sessionId,
          approval_id: input.id,
          approved: input.approved,
          reason: input.reason,
        }),
      );
    } catch (err) {
      const message = parseCallServiceError(err);
      useMessageStore.getState().errorMessage(message);
      throw new Error(message, { cause: err });
    }
  }

  stop(): void {
    const sessionId = this.activeSessionId || useAIAssistantStore.getState().activeSessionId || '';
    if (!sessionId) return;
    void AIService.StopGeneration(sessionId).catch(() => {});
  }
}

type ListChatMessage = ListMessagesResult['messages'][number];

/** Map persisted Genkit messages (user/model/tool) onto MUI Chat bubbles. */
function genkitRowsToMui(rows: AIMessage[]): ListChatMessage[] {
  const out: ListChatMessage[] = [];
  let pending: ListChatMessage | null = null;

  const flush = () => {
    if (pending) {
      out.push(pending);
      pending = null;
    }
  };

  for (const m of rows) {
    if (m.role === 'system') continue;
    if (m.role === 'user') {
      flush();
      out.push({
        id: m.id,
        role: 'user',
        status: m.status === 'cancelled' ? 'cancelled' : 'sent',
        parts: genkitPartsToMui(parseGenkitParts(m.parts), m.content),
        createdAt: new Date(m.timestamp * 1000).toISOString(),
        conversationId: m.session_id,
        author: currentUser,
      });
      continue;
    }

    const nextParts = genkitPartsToMui(parseGenkitParts(m.parts), m.content, m.status);
    if (!pending) {
      pending = {
        id: m.id,
        role: 'assistant',
        status: m.status === 'cancelled' ? 'cancelled' : 'sent',
        parts: nextParts,
        createdAt: new Date(m.timestamp * 1000).toISOString(),
        conversationId: m.session_id,
        author: aiUser,
      };
    } else {
      pending.parts = mergeToolParts(pending.parts, nextParts);
      if (m.status === 'cancelled') {
        pending.status = 'cancelled';
      }
    }
  }
  flush();
  return out;
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

function parseGenkitParts(raw: string): any[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function genkitPartsToMui(parts: any[], fallbackText: string, rowStatus?: string): MessageParts {
  const out: any[] = [];
  for (const p of parts) {
    if (!p || typeof p !== 'object') continue;
    if (p.reasoning != null) {
      out.push({ type: 'reasoning', text: String(p.reasoning) });
      continue;
    }
    if (p.toolRequest) {
      const req = p.toolRequest;
      const toolName = String(req.name || '');
      out.push({
        type: toolPartType(toolName),
        toolInvocation: {
          toolCallId: String(req.ref || req.name || ''),
          toolName,
          state: 'input-available',
          input: sanitizeToolInputForDisplay(toolName, req.input ?? {}),
        },
      });
      continue;
    }
    if (p.toolResponse) {
      const res = p.toolResponse;
      const toolName = String(res.name || '');
      // Mixed success+deny rows keep status=""; per-part text decides. All-denied
      // rows use status=denied (every toolResponse is a denial).
      const reason = denialReasonFromToolOutput(res.output);
      const denied = reason !== null || rowStatus === 'denied';
      out.push({
        type: toolPartType(toolName),
        toolInvocation: denied
          ? {
              toolCallId: String(res.ref || res.name || ''),
              toolName,
              state: 'output-denied',
              input: {},
              approval: { approved: false as const, reason: reason ?? 'user denied' },
            }
          : {
              toolCallId: String(res.ref || res.name || ''),
              toolName,
              state: 'output-available',
              output: res.output ?? {},
            },
      });
      continue;
    }
    if (p.media && typeof p.media === 'object') {
      const media = p.media as { contentType?: string; url?: string };
      const filename =
        p.metadata && typeof p.metadata === 'object' && 'filename' in p.metadata
          ? String((p.metadata as { filename?: unknown }).filename || '')
          : '';
      if (media.url) {
        out.push({
          type: 'file',
          mediaType: String(media.contentType || 'application/octet-stream'),
          url: String(media.url),
          ...(filename ? { filename } : {}),
        });
      }
      continue;
    }
    if (typeof p.text === 'string' && p.text !== '') {
      out.push({ type: 'text', text: p.text });
    }
  }
  if (out.length === 0 && fallbackText) {
    out.push({ type: 'text', text: fallbackText });
  }
  return out as MessageParts;
}

/** Returns the user-facing reason when output is a known denial string; otherwise null. */
function denialReasonFromToolOutput(output: unknown): string | null {
  let text = '';
  if (typeof output === 'string') {
    text = output;
  } else if (output != null && typeof output !== 'object') {
    text = String(output);
  } else if (output && typeof output === 'object' && 'output' in output) {
    const inner = (output as { output?: unknown }).output;
    if (typeof inner === 'string') {
      text = inner;
    }
  }
  text = text.trim();
  if (!text) {
    return null;
  }
  // Omit trailing space so "…execution:" and "…execution: reason" both match after trim.
  const prefixes = ['User denied command execution:', 'User denied tool execution:'];
  for (const prefix of prefixes) {
    if (text.startsWith(prefix)) {
      return text.slice(prefix.length).trim() || 'user denied';
    }
  }
  return null;
}

function mergeToolParts(existing: MessageParts, incoming: MessageParts): MessageParts {
  const merged = [...existing] as any[];
  for (const part of incoming as any[]) {
    const state = part.toolInvocation?.state;
    if (isToolMessagePart(part) && (state === 'output-available' || state === 'output-denied')) {
      const id = String(part.toolInvocation.toolCallId || '');
      if (id) {
        const idx = merged.findLastIndex(
          (p: any) => isToolMessagePart(p) && p.toolInvocation?.toolCallId === id,
        );
        if (idx >= 0) {
          merged[idx] = {
            ...merged[idx],
            toolInvocation: {
              ...merged[idx].toolInvocation,
              ...part.toolInvocation,
              input: merged[idx].toolInvocation?.input ?? part.toolInvocation.input,
            },
          };
          continue;
        }
      }
    }
    merged.push(part);
  }
  return merged as MessageParts;
}
