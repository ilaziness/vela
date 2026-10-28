import { t } from "../i18n";
import type { Tool } from "../types/tool";

const TOOL_KEY_BY_ID: Record<string, string> = {
  "json-yaml": "json_yaml",
  timestamp: "timestamp",
  encoder: "encoder",
  hash: "hash",
  regex: "regex",
  cron: "cron",
  cidr: "cidr",
  "port-check": "port_check",
  jwt: "jwt",
  random: "random",
  "base-convert": "base_convert",
  "text-diff": "text_diff",
  chmod: "chmod",
};

function toolKey(id: string): string {
  return TOOL_KEY_BY_ID[id] ?? id.replace(/-/g, "_");
}

function translateToolField(
  id: string,
  field: "title" | "desc",
  fallback: string,
): string {
  const key = `tools.${toolKey(id)}.${field}`;
  const msg = t(key);
  return msg === key ? fallback : msg;
}

export function displayToolName(tool: Tool): string {
  return translateToolField(tool.id, "title", tool.name);
}

export function displayToolDescription(tool: Tool): string {
  return translateToolField(tool.id, "desc", tool.description);
}

/** Category is a stable id from Go (`dev`, `encoding`, …). */
export function displayToolCategory(category: string): string {
  const key = `tools.category.${category}`;
  const msg = t(key);
  return msg === key ? category : msg;
}
