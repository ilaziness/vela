export const SIDEBAR_WIDTH_STORAGE_KEY = 'vela.ai.sidebarWidth';
const LEGACY_SIDEBAR_WIDTH_STORAGE_KEY = 'vexo.ai.sidebarWidth';

/** 与 SSHTabBody StatusBar 高度保持一致 */
export const SSH_STATUS_BAR_HEIGHT = 25;

/** 与 Header / SSHTabs 左侧栏宽度保持一致 */
export const APP_HEADER_RAIL_WIDTH = 36;

export const SIDEBAR_RESIZE_HANDLE_WIDTH = 4;

export const SIDEBAR_WIDTH = {
  MIN: 280,
  MAX_RATIO: 0.5,
  MIN_TERMINAL: 480,
  DEFAULT_VIEWPORT_RATIO: 0.3,
} as const;

export function clampSidebarWidth(width: number): number {
  const minTerminal = Math.min(
    SIDEBAR_WIDTH.MIN_TERMINAL,
    Math.max(320, Math.round(window.innerWidth * 0.45)),
  );
  // 挤压布局下终端可用宽 = 视口 - 左侧栏 - 侧边栏，需从 max 中扣掉 header rail
  const maxByTerminal = window.innerWidth - APP_HEADER_RAIL_WIDTH - minTerminal;
  const max = Math.max(
    SIDEBAR_WIDTH.MIN,
    Math.min(window.innerWidth * SIDEBAR_WIDTH.MAX_RATIO, maxByTerminal),
  );
  return Math.max(SIDEBAR_WIDTH.MIN, Math.min(width, max));
}

export function getDefaultSidebarWidth(): number {
  return clampSidebarWidth(
    Math.round(window.innerWidth * SIDEBAR_WIDTH.DEFAULT_VIEWPORT_RATIO),
  );
}

export function loadStoredSidebarWidth(): number {
  try {
    let stored = localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY);
    if (!stored) {
      stored = localStorage.getItem(LEGACY_SIDEBAR_WIDTH_STORAGE_KEY);
      if (stored) {
        localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, stored);
        localStorage.removeItem(LEGACY_SIDEBAR_WIDTH_STORAGE_KEY);
      }
    }
    if (stored) {
      const parsed = Number.parseInt(stored, 10);
      if (!Number.isNaN(parsed)) {
        return clampSidebarWidth(parsed);
      }
    }
  } catch {
    // ignore
  }
  return getDefaultSidebarWidth();
}

export function persistSidebarWidth(width: number): void {
  try {
    localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, String(width));
  } catch {
    // ignore
  }
}
