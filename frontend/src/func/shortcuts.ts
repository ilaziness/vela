export const EVENT_SHORTCUT = "eventShortcut";

export enum ShortcutAction {
  TerminalFind = "terminal.find",
  TerminalCopy = "terminal.copy",
  TerminalPaste = "terminal.paste",
  TerminalAddToChat = "terminal.addToChat",
  TerminalToggleLog = "terminal.toggleLog",
  TerminalClear = "terminal.clear",
  PanelAI = "panel.ai",
  PanelBookmarks = "panel.bookmarks",
  PanelBookmarkConnect = "panel.bookmarkConnect",
  PanelCommand = "panel.command",
  PanelTools = "panel.tools",
  PanelSettings = "panel.settings",
}

export function parseShortcutAction(data: unknown): string | null {
  try {
    const payload =
      typeof data === "string"
        ? (JSON.parse(data) as { action?: string })
        : (data as { action?: string } | undefined);
    return payload?.action ?? null;
  } catch {
    return null;
  }
}

export function toShellCodeBlock(text: string): string {
  return "```shell\n" + text + "\n```";
}

const NAMED_KEYS: Record<string, string> = {
  ",": ",",
  Escape: "Escape",
  Enter: "Enter",
  " ": "Space",
  Spacebar: "Space",
  Tab: "Tab",
  Backspace: "Backspace",
  Delete: "Delete",
  ArrowUp: "Up",
  ArrowDown: "Down",
  ArrowLeft: "Left",
  ArrowRight: "Right",
};

/** Convert a KeyboardEvent into a logical accel (CmdOrCtrl+…), or null if incomplete. */
export function keyboardEventToLogical(e: KeyboardEvent): string | null {
  if (
    e.repeat ||
    e.key === "Escape" ||
    e.key === "Shift" ||
    e.key === "Control" ||
    e.key === "Meta" ||
    e.key === "Alt"
  ) {
    return null;
  }

  const parts: string[] = [];
  const primary = e.ctrlKey || e.metaKey;
  if (primary) {
    parts.push("CmdOrCtrl");
  }
  if (e.shiftKey) {
    parts.push("Shift");
  }
  if (e.altKey) {
    parts.push("Alt");
  }

  let key: string;
  if (e.code === "Comma" || e.key === ",") {
    key = ",";
  } else if (NAMED_KEYS[e.key]) {
    key = NAMED_KEYS[e.key];
  } else if (/^F\d{1,2}$/i.test(e.key)) {
    key = e.key.toUpperCase();
  } else if (e.key.length === 1) {
    key = e.key.toUpperCase();
  } else {
    return null;
  }

  const isFunc = /^F\d{1,2}$/i.test(key);
  // Require Ctrl/Cmd or Alt (or F-key); Shift alone must not steal typing.
  if (!primary && !e.altKey && !isFunc) {
    return null;
  }
  parts.push(key);
  return parts.join("+");
}

/** Whether the event matches a stored logical binding (e.g. CmdOrCtrl+Shift+V). */
export function eventMatchesLogical(
  e: KeyboardEvent,
  logical: string | undefined | null,
): boolean {
  if (!logical) {
    return false;
  }
  const parts = logical.split("+");
  if (parts.length === 0) {
    return false;
  }
  const key = parts[parts.length - 1];
  const mods = new Set(parts.slice(0, -1).map((p) => p.toLowerCase()));

  const wantPrimary = mods.has("cmdorctrl") || mods.has("cmd") || mods.has("command") || mods.has("ctrl") || mods.has("control");
  const wantShift = mods.has("shift");
  const wantAlt = mods.has("alt") || mods.has("option");

  const hasPrimary = e.ctrlKey || e.metaKey;
  if (wantPrimary !== hasPrimary) {
    return false;
  }
  if (wantShift !== e.shiftKey) {
    return false;
  }
  if (wantAlt !== e.altKey) {
    return false;
  }

  const eventKey =
    e.code === "Comma" || e.key === ","
      ? ","
      : (NAMED_KEYS[e.key] ??
        (/^F\d{1,2}$/i.test(e.key)
          ? e.key.toUpperCase()
          : e.key.length === 1
            ? e.key.toUpperCase()
            : e.key));

  return eventKey.toUpperCase() === key.toUpperCase();
}

/** True when focus is in a real UI editor, not xterm's helper textarea. */
export function isEditableFocus(): boolean {
  const el = document.activeElement;
  if (!el || !(el instanceof HTMLElement)) {
    return false;
  }
  // xterm keeps focus on a hidden textarea inside .xterm.
  if (el.closest(".xterm")) {
    return false;
  }
  const tag = el.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
    return true;
  }
  if (el.isContentEditable) {
    return true;
  }
  return Boolean(el.closest("[contenteditable='true']"));
}
