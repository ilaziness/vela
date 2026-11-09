import { ConnectionStatus } from "../types/ssh";
import type { SessionTab } from "../types/session";

/**
 * Tab lifecycle status. Disconnected drops sessionLink.linkID (end Exec/AI session).
 */
export function tabWithConnectionStatus(
  tab: SessionTab,
  status: ConnectionStatus,
): SessionTab {
  const sessionLink =
    status === ConnectionStatus.Disconnected && tab.sessionLink
      ? { ...tab.sessionLink, linkID: undefined }
      : tab.sessionLink;
  return { ...tab, connectionStatus: status, sessionLink };
}

/**
 * WebSocket / session path. Never clears linkID (tab lifecycle owns that).
 * Ignores Disconnected while the tab is Connecting so a closing socket cannot
 * clobber an in-flight reconnect (StrictMode remount or reload unmount).
 */
export function tabWithSessionConnectionStatus(
  tab: SessionTab,
  status: ConnectionStatus,
): SessionTab | null {
  if (
    status === ConnectionStatus.Disconnected &&
    tab.connectionStatus === ConnectionStatus.Connecting
  ) {
    return null;
  }
  return { ...tab, connectionStatus: status };
}
