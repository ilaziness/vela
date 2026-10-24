import { useSSHTabsStore } from '../stores/ssh';
import { ConnectionStatus, type SSHTab } from '../types/ssh';
import { SSHContext } from '../../bindings/github.com/ilaziness/vela/services/models';

/** SSH bind state of the currently active SSH tab (for AI sidebar). */
export enum CurrentSSHBindStatus {
  Connected = 'connected',
  Connecting = 'connecting',
  Disconnected = 'disconnected',
  None = 'none',
}

export interface CurrentSSHBindState {
  status: CurrentSSHBindStatus;
  /** e.g. user@host:port when host info exists */
  targetLabel: string;
  /** Present only when this tab has an Exec-ready session. */
  context?: SSHContext;
}

export const RUN_SSH_COMMAND_TOOL = 'run_ssh_command';
export const UPSERT_PLAN_TOOL = 'upsert_plan';
export const USE_SKILL_TOOL = 'use_skill';

/** Legacy UI-only field once merged into tool input; strip from display. */
const LEGACY_SSH_TARGET_INPUT_KEY = '_sshTarget';

/** Remove non-model fields from run_ssh_command tool input before showing in Chat. */
export function sanitizeToolInputForDisplay(toolName: string, input: unknown): unknown {
  if (toolName !== RUN_SSH_COMMAND_TOOL) return input;
  if (!input || typeof input !== 'object' || Array.isArray(input)) return input;
  const { [LEGACY_SSH_TARGET_INPUT_KEY]: _removed, ...rest } = input as Record<string, unknown>;
  void _removed;
  return rest;
}

function formatTargetLabel(user: string, host: string, port: number): string {
  return `${user}@${host}:${port}`;
}

/**
 * Resolve bind state for one SSH tab — never looks at other tabs.
 * Exec is ready once store sshInfo.linkID exists (SSH handshake), not waiting
 * for terminal WebSocket. Disconnected clears that linkID in the tabs store.
 */
export function resolveSSHBindState(tab: SSHTab | undefined): CurrentSSHBindState {
  const sshInfo = tab?.sshInfo;
  const status = tab?.connectionStatus;

  if (!sshInfo?.host || !sshInfo.user || typeof sshInfo.port !== 'number' || sshInfo.port <= 0) {
    return { status: CurrentSSHBindStatus.None, targetLabel: '' };
  }

  const targetLabel = formatTargetLabel(sshInfo.user, sshInfo.host, sshInfo.port);

  if (status === ConnectionStatus.Disconnected) {
    return { status: CurrentSSHBindStatus.Disconnected, targetLabel };
  }

  if (sshInfo.linkID) {
    return {
      status: CurrentSSHBindStatus.Connected,
      targetLabel,
      context: new SSHContext({
        link_id: sshInfo.linkID,
        host: sshInfo.host,
        port: sshInfo.port,
        user: sshInfo.user,
      }),
    };
  }

  if (status === ConnectionStatus.Connecting) {
    return { status: CurrentSSHBindStatus.Connecting, targetLabel };
  }

  return { status: CurrentSSHBindStatus.None, targetLabel: '' };
}

/** Active SSH tab only — never falls back to another tab. */
export function getCurrentSSHBindState(): CurrentSSHBindState {
  const { currentTab, getByIndex } = useSSHTabsStore.getState();
  return resolveSSHBindState(getByIndex(currentTab));
}
