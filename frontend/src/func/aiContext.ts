import { useSessionTabsStore } from "../stores/sessionTabs";
import { ConnectionStatus } from "../types/ssh";
import {
  SessionProtocol,
  sessionTabLabel,
  type SessionTab,
} from "../types/session";
import { SSHContext } from "../../bindings/github.com/ilaziness/vela/services/models";

/** Bind state of the currently active session tab (for AI sidebar). */
export enum CurrentSSHBindStatus {
  Connected = "connected",
  Connecting = "connecting",
  Disconnected = "disconnected",
  None = "none",
}

export interface CurrentSSHBindState {
  status: CurrentSSHBindStatus;
  /** e.g. user@host:port or COM3 @ 115200 */
  targetLabel: string;
  protocol?: SessionProtocol;
  /** Present only for SSH when Exec-ready. */
  context?: SSHContext;
}

export const RUN_SSH_COMMAND_TOOL = "run_ssh_command";
export const UPSERT_PLAN_TOOL = "upsert_plan";
export const USE_SKILL_TOOL = "use_skill";
export const READ_LOCAL_FILE_TOOL = "read_local_file";
export const WRITE_LOCAL_FILE_TOOL = "write_local_file";
export const LIST_LOCAL_DIRECTORY_TOOL = "list_local_directory";
export const WEB_SEARCH_TOOL = "web_search";

const KNOWN_CHAT_TOOLS = new Set([
  RUN_SSH_COMMAND_TOOL,
  UPSERT_PLAN_TOOL,
  USE_SKILL_TOOL,
  READ_LOCAL_FILE_TOOL,
  WRITE_LOCAL_FILE_TOOL,
  LIST_LOCAL_DIRECTORY_TOOL,
  WEB_SEARCH_TOOL,
]);

export function isKnownChatTool(toolName: string): boolean {
  return KNOWN_CHAT_TOOLS.has(toolName);
}

/** Legacy UI-only field once merged into tool input; strip from display. */
const LEGACY_SSH_TARGET_INPUT_KEY = "_sshTarget";

/** Remove non-model fields from run_ssh_command tool input before showing in Chat. */
export function sanitizeToolInputForDisplay(
  toolName: string,
  input: unknown,
): unknown {
  if (toolName !== RUN_SSH_COMMAND_TOOL) return input;
  if (!input || typeof input !== "object" || Array.isArray(input)) return input;
  const { [LEGACY_SSH_TARGET_INPUT_KEY]: _removed, ...rest } = input as Record<
    string,
    unknown
  >;
  void _removed;
  return rest;
}

/**
 * Resolve bind state for one session tab — never looks at other tabs.
 * Missing/unknown protocol → unbound. SSHContext only for protocol===ssh.
 */
export function resolveSSHBindState(
  tab: SessionTab | undefined,
): CurrentSSHBindState {
  const sessionLink = tab?.sessionLink;
  const status = tab?.connectionStatus;

  if (
    !sessionLink?.protocol ||
    (sessionLink.protocol !== SessionProtocol.SSH &&
      sessionLink.protocol !== SessionProtocol.Serial)
  ) {
    return { status: CurrentSSHBindStatus.None, targetLabel: "" };
  }

  const targetLabel = sessionTabLabel(sessionLink);

  if (status === ConnectionStatus.Disconnected) {
    return {
      status: CurrentSSHBindStatus.Disconnected,
      targetLabel,
      protocol: sessionLink.protocol,
    };
  }

  if (sessionLink.linkID) {
    const state: CurrentSSHBindState = {
      status: CurrentSSHBindStatus.Connected,
      targetLabel,
      protocol: sessionLink.protocol,
    };
    if (
      sessionLink.protocol === SessionProtocol.SSH &&
      sessionLink.host &&
      sessionLink.user &&
      typeof sessionLink.port === "number" &&
      sessionLink.port > 0
    ) {
      // Full SSHContext only for ssh — backend EnableSSH still gated by Hub protocol.
      state.context = new SSHContext({
        link_id: sessionLink.linkID,
        host: sessionLink.host,
        port: sessionLink.port,
        user: sessionLink.user,
      });
    } else if (sessionLink.protocol === SessionProtocol.Serial) {
      // link_id only: Hub resolves serial Current session; never enables Exec.
      state.context = new SSHContext({
        link_id: sessionLink.linkID,
        host: "",
        port: 0,
        user: "",
      });
    }
    return state;
  }

  if (status === ConnectionStatus.Connecting) {
    return {
      status: CurrentSSHBindStatus.Connecting,
      targetLabel,
      protocol: sessionLink.protocol,
    };
  }

  return { status: CurrentSSHBindStatus.None, targetLabel: "" };
}

/** Active session tab only — never falls back to another tab. */
export function getCurrentSSHBindState(): CurrentSSHBindState {
  const { currentTab, getByIndex } = useSessionTabsStore.getState();
  return resolveSSHBindState(getByIndex(currentTab));
}
