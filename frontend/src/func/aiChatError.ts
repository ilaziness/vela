import { t } from "../i18n";

/** English / legacy raw errors that should map to catalog keys. */
const NOT_ENABLED_PATTERNS = [
  /ai engine not initialized/i,
  /ai assistant is not enabled/i,
  /ai \u52a9\u624b\u5c1a\u672a\u542f\u7528/i,
  /ai \u52a9\u624b\u521d\u59cb\u5316\u5931\u8d25/i,
  /ai \u914d\u7f6e\u52a0\u8f7d\u5931\u8d25/i,
];

const TOOL_ARGS_PATTERNS = [
  /could not parse tool args/i,
  /unmarshal failed to parse json/i,
  /\u5de5\u5177\u53c2\u6570 JSON \u65e0\u6548/i,
];

/**
 * Polish AI chat errors for display.
 * Prefer already-translated usererr strings from translateServiceError;
 * only remap known technical/legacy English (or short legacy CN) fragments.
 */
export const formatAIChatError = (raw: string): string => {
  const message = raw.trim().replace(/^AI chat failed:\s*/i, "").trim();
  if (NOT_ENABLED_PATTERNS.some((pattern) => pattern.test(message))) {
    return t("ai.not_enabled");
  }
  if (TOOL_ARGS_PATTERNS.some((pattern) => pattern.test(message))) {
    return t("ai.error.tool_args_invalid");
  }
  if (message.length > 400) {
    return `${message.slice(0, 400)}…`;
  }
  return message;
};
