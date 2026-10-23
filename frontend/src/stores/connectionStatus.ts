import { ConnectionStatus, type SSHTab } from "../types/ssh";

/**
 * Tab lifecycle status. Disconnected drops sshInfo.linkID (end Exec/AI session).
 */
export function tabWithConnectionStatus(
  tab: SSHTab,
  status: ConnectionStatus,
): SSHTab {
  const sshInfo =
    status === ConnectionStatus.Disconnected && tab.sshInfo
      ? { ...tab.sshInfo, linkID: undefined }
      : tab.sshInfo;
  return { ...tab, connectionStatus: status, sshInfo };
}

/**
 * WebSocket / session path. Never clears linkID (tab lifecycle owns that).
 * Ignores Disconnected while the tab is Connecting so a closing socket cannot
 * clobber an in-flight reconnect (StrictMode remount or reload unmount).
 */
export function tabWithSessionConnectionStatus(
  tab: SSHTab,
  status: ConnectionStatus,
): SSHTab | null {
  if (
    status === ConnectionStatus.Disconnected &&
    tab.connectionStatus === ConnectionStatus.Connecting
  ) {
    return null;
  }
  return { ...tab, connectionStatus: status };
}
