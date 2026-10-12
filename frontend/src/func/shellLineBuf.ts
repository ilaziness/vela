/** Local line buffer for terminal history hints (mirrors backend LineCapture lightly). */

export function applyShellInput(
  buf: string,
  data: string,
): { buf: string; submitted?: string } {
  let result = buf;
  let submitted: string | undefined;

  for (let i = 0; i < data.length; ) {
    const ch = data[i];

    if (ch === "\x1b") {
      i += 1;
      if (i < data.length && data[i] === "[") {
        i += 1;
        while (i < data.length) {
          const c = data[i];
          i += 1;
          if (c >= "@" && c <= "~") {
            break;
          }
        }
      } else if (i < data.length) {
        i += 1;
      }
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
      i += 1;
      continue;
    }

    if (ch === "\x15" || ch === "\x03") {
      result = "";
      i += 1;
      continue;
    }

    if (ch < " " && ch !== "\t") {
      i += 1;
      continue;
    }

    const cp = data.codePointAt(i);
    if (cp === undefined) {
      break;
    }
    result += String.fromCodePoint(cp);
    i += cp > 0xffff ? 2 : 1;
  }

  return { buf: result, submitted };
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
