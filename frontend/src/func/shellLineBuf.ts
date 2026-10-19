/** Local line buffer for terminal history hints (mirrors backend LineCapture lightly). */

/** No hint row highlighted; Enter/Tab pass through to the shell. */
export const HINT_INDEX_NONE = -1;

/** Wait for the PTY redraw to go quiet before reading the screen line. */
export const SCREEN_SYNC_QUIET_MS = 40;

/** Give up waiting for the shell redraw if it never arrives. */
export const SCREEN_SYNC_WATCH_MS = 500;

export type HintInputPlan =
  | { kind: "submit" }
  | { kind: "local"; buf: string; reveal: boolean }
  | { kind: "screen"; reveal: boolean };

/**
 * One decision for a key chunk.
 * reveal is true only when the chunk edits command text (typing, backspace, delete).
 * History and cursor keys update the tracked line without opening hints.
 */
export function planHintInput(
  buf: string,
  data: string,
  midLine: boolean,
  syncing: boolean,
): HintInputPlan {
  const applied = applyShellInput(buf, data);
  if (applied.submitted) {
    return { kind: "submit" };
  }
  // While a shell redraw is in flight, keep reading the screen so the local
  // buffer does not diverge. reveal still follows this chunk only.
  if (applied.needsScreenSync || midLine || syncing) {
    return { kind: "screen", reveal: applied.reveal };
  }
  return { kind: "local", buf: applied.buf, reveal: applied.reveal };
}

/** Skip one ESC sequence. Delete (\x1b[3~) edits text; arrows do not. */
function consumeEscape(
  data: string,
  i: number,
): { next: number; editsText: boolean } {
  const start = i;
  i += 1;
  if (i < data.length && (data[i] === "[" || data[i] === "O")) {
    const ss3 = data[i] === "O";
    i += 1;
    if (ss3) {
      if (i < data.length) {
        i += 1;
      }
    } else {
      while (i < data.length) {
        const c = data[i];
        i += 1;
        if (c >= "@" && c <= "~") {
          break;
        }
      }
    }
  } else if (i < data.length) {
    i += 1;
  }
  return { next: i, editsText: data.slice(start, i) === "\x1b[3~" };
}

export function stepHintIndex(
  current: number,
  count: number,
  direction: 1 | -1,
): number {
  if (count <= 0) {
    return HINT_INDEX_NONE;
  }
  if (current < 0) {
    return direction > 0 ? 0 : count - 1;
  }
  return (current + direction + count) % count;
}

export interface ScreenLine {
  isWrapped: boolean;
  translateToString(
    trimRight?: boolean,
    startColumn?: number,
    endColumn?: number,
  ): string;
}

/** Viewport slice needed to recover the shell's current input line. */
export interface ScreenBuffer {
  type: "normal" | "alternate";
  cursorX: number;
  cursorY: number;
  baseY: number;
  length: number;
  cols: number;
  getLine(y: number): ScreenLine | undefined;
}

function applyShellInput(
  buf: string,
  data: string,
): {
  buf: string;
  submitted?: string;
  needsScreenSync: boolean;
  reveal: boolean;
} {
  let result = buf;
  let submitted: string | undefined;
  let needsScreenSync = false;
  let reveal = false;

  for (let i = 0; i < data.length; ) {
    const ch = data[i];

    if (ch === "\x1b") {
      // Arrows, Home/End, Delete: the shell redraws the line.
      needsScreenSync = true;
      const esc = consumeEscape(data, i);
      if (esc.editsText) {
        reveal = true;
      }
      i = esc.next;
      continue;
    }

    if (ch === "\r" || ch === "\n") {
      const line = result.trim();
      result = "";
      if (line) {
        submitted = line;
      }
      i += 1;
      if (ch === "\r" && data[i] === "\n") {
        i += 1;
      }
      continue;
    }

    if (ch === "\x7f" || ch === "\b") {
      const chars = [...result];
      chars.pop();
      result = chars.join("");
      reveal = true;
      i += 1;
      continue;
    }

    if (ch === "\x15" || ch === "\x03") {
      result = "";
      i += 1;
      continue;
    }

    if (ch === "\t" || ch < " ") {
      // Tab and cursor controls rewrite the line without opening hints.
      // Ctrl-W / Ctrl-K / Ctrl-D change the text the user is editing.
      needsScreenSync = true;
      if (ch === "\x17" || ch === "\x0b" || ch === "\x04") {
        reveal = true;
      }
      i += 1;
      continue;
    }

    const cp = data.codePointAt(i);
    if (cp === undefined) {
      break;
    }
    result += String.fromCodePoint(cp);
    reveal = true;
    i += cp > 0xffff ? 2 : 1;
  }

  return { buf: result, submitted, needsScreenSync, reveal };
}

/** Display columns of a command, treating non-Latin-1 code points as width 2. */
function cellWidth(text: string): number {
  let width = 0;
  for (const ch of text) {
    const cp = ch.codePointAt(0) ?? 0;
    width += cp > 0xff ? 2 : 1;
  }
  return width;
}

/**
 * Prompt width while the cursor is still on the pre-key line.
 * Only valid when the command has not wrapped and the cursor is at its end.
 */
export function inferPromptCols(
  screen: ScreenBuffer,
  lineBuf: string,
): number | null {
  if (screen.type !== "normal" || hasContentAfterCursor(screen)) {
    return null;
  }
  const y = screen.baseY + screen.cursorY;
  if (screen.getLine(y)?.isWrapped) {
    return null;
  }
  const cols = screen.cursorX - cellWidth(lineBuf);
  return cols >= 0 ? cols : null;
}

/** True when the cursor is left of non-space cells on the current input line. */
export function hasContentAfterCursor(screen: ScreenBuffer): boolean {
  if (screen.type !== "normal") {
    return false;
  }
  const y = screen.baseY + screen.cursorY;
  const line = screen.getLine(y);
  if (!line) {
    return false;
  }
  if (line.translateToString(true, screen.cursorX).length > 0) {
    return true;
  }
  for (let row = y + 1; row < screen.length; row++) {
    const wrapped = screen.getLine(row);
    if (!wrapped?.isWrapped) {
      break;
    }
    if (wrapped.translateToString(true).length > 0) {
      return true;
    }
  }
  return false;
}

/**
 * Command text on the cursor's logical line, excluding the prompt columns.
 * Walks wrapped rows so a recalled history line longer than the width is whole.
 */
export function readScreenCommand(
  screen: ScreenBuffer,
  promptCols: number,
): string {
  if (screen.type !== "normal" || promptCols < 0) {
    return "";
  }
  let start = screen.baseY + screen.cursorY;
  while (start > 0 && screen.getLine(start)?.isWrapped) {
    start -= 1;
  }
  let end = screen.baseY + screen.cursorY;
  while (end + 1 < screen.length && screen.getLine(end + 1)?.isWrapped) {
    end += 1;
  }
  let text = "";
  for (let y = start; y <= end; y++) {
    const line = screen.getLine(y);
    if (!line) {
      break;
    }
    const from = y === start ? Math.min(promptCols, screen.cols) : 0;
    text += line.translateToString(false, from);
  }
  return text.replace(/\s+$/u, "");
}

export function matchCommandHints(
  history: string[],
  prefix: string,
  limit = 8,
): string[] {
  if (!prefix) {
    return [];
  }
  const seen = new Set<string>();
  const exact: string[] = [];
  const longer: string[] = [];
  for (const cmd of history) {
    if (!cmd.startsWith(prefix) || seen.has(cmd)) {
      continue;
    }
    seen.add(cmd);
    if (cmd === prefix) {
      exact.push(cmd);
    } else {
      longer.push(cmd);
    }
  }
  // Only show the list when there is something to complete beyond what is typed.
  if (longer.length === 0) {
    return [];
  }
  // Exact match first (dismiss on Enter); longer items paste the suffix.
  return [...exact, ...longer].slice(0, limit);
}
