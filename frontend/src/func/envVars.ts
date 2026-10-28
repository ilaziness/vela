import { tf } from "../i18n";

/** Parse KEY=value lines into a record. TERM lines are ignored. */
export function parseEnvLines(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }
    const eq = line.indexOf("=");
    if (eq <= 0) {
      throw new Error(tf("validation.env_invalid_line", line));
    }
    const key = line.slice(0, eq).trim();
    if (!key || key.toUpperCase() === "TERM") {
      continue;
    }
    out[key] = line.slice(eq + 1);
  }
  return out;
}

/** Format env record as KEY=value lines for the form. */
export function formatEnvLines(env: Record<string, string> | null | undefined): string {
  if (!env) {
    return "";
  }
  return Object.entries(env)
    .filter(([k]) => k && k.toUpperCase() !== "TERM")
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
}

/** Encode env map to JSON string for bookmark.env_vars. */
export function envToJSON(env: Record<string, string>): string {
  const keys = Object.keys(env);
  if (keys.length === 0) {
    return "";
  }
  return JSON.stringify(env);
}

/** Decode bookmark.env_vars JSON to a map. */
export function envFromJSON(raw: string | null | undefined): Record<string, string> {
  const s = (raw || "").trim();
  if (!s) {
    return {};
  }
  try {
    const parsed = JSON.parse(s) as Record<string, string>;
    if (!parsed || typeof parsed !== "object") {
      return {};
    }
    const out: Record<string, string> = {};
    for (const [k, v] of Object.entries(parsed)) {
      if (k && k.toUpperCase() !== "TERM" && typeof v === "string") {
        out[k] = v;
      }
    }
    return out;
  } catch {
    return {};
  }
}
