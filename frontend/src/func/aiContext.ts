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

/** Merged into run_ssh_command tool input for approval UI (this Chat run's host). */
export const SSH_TARGET_INPUT_KEY = '_sshTarget';

export const RUN_SSH_COMMAND_TOOL = 'run_ssh_command';

function formatTargetLabel(user: string, host: string, port: number): string {
  return `${user}@${host}:${port}`;
}

/**
 * Resolve bind state for one SSH tab — never looks at other tabs.
 * Exec is ready once linkID exists; do not wait for terminal WebSocket.
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
