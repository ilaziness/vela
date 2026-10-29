import { useSyncExternalStore } from "react";
import { enUS, zhCN, zhTW } from "@mui/material/locale";
import type { Localization } from "@mui/material/locale";
import type { ChatToolInvocationState } from "@mui/x-chat/headless";

export enum AppLanguage {
  En = "en",
  ZhCN = "zh-CN",
  ZhTW = "zh-TW",
}

/** Stable persist ID for the default bookmark group (display via t). */
export const DEFAULT_BOOKMARK_GROUP_ID = "__default__";

/** Stable persist ID / empty title for a new AI session (display via t). */
export const NEW_SESSION_TITLE_ID = "__new_session__";

/** Stable tab name for a blank SSH tab (display via displaySSHTabName). */
export const NEW_CONNECTION_TAB_ID = "__new_connection__";

/** Legacy Chinese defaults still present in older DBs before Go migration. */
const LEGACY_DEFAULT_GROUP = "\u9ed8\u8ba4\u4e66\u7b7e";
const LEGACY_NEW_SESSION = "\u65b0\u4f1a\u8bdd";

let language: AppLanguage = AppLanguage.En;
let catalog: Record<string, string> = {};
let bootstrapPromise: Promise<void> | null = null;
const languageListeners = new Set<() => void>();

type TranslateFn = (key: string, ...args: unknown[]) => string;

function emitLanguageChange(): void {
  languageListeners.forEach((l) => l());
}

/** Subscribe to language/catalog updates (for useSyncExternalStore). */
export function subscribeLanguage(onStoreChange: () => void): () => void {
  languageListeners.add(onStoreChange);
  return () => {
    languageListeners.delete(onStoreChange);
  };
}

/** Hook so memoized components re-render after SetLanguage. */
export function useLanguage(): AppLanguage {
  return useSyncExternalStore(subscribeLanguage, getLanguage, getLanguage);
}

export function getLanguage(): AppLanguage {
  return language;
}

export function normalizeAppLanguage(lang: string): AppLanguage {
  const s = (lang || "").trim().replace(/_/g, "-");
  const lower = s.toLowerCase();
  if (
    lower === "zh-tw" ||
    lower === "zh-hk" ||
    lower === "zh-mo" ||
    lower.startsWith("zh-hant") ||
    lower.startsWith("zh-tw") ||
    lower.startsWith("zh-hk") ||
    lower.startsWith("zh-mo")
  ) {
    return AppLanguage.ZhTW;
  }
  if (
    lower === "zh" ||
    lower === "zh-cn" ||
    lower === "zh-sg" ||
    lower.startsWith("zh-hans") ||
    lower.startsWith("zh-cn") ||
    lower.startsWith("zh-sg") ||
    lower.startsWith("zh-")
  ) {
    return AppLanguage.ZhCN;
  }
  return AppLanguage.En;
}

/** Apply bootstrap / eventLanguage payload and sync document.lang. */
export function applyI18nBootstrap(boot: {
  language?: string | null;
  /** Wails map models use optional values (`{ [k: string]?: string }`). */
  catalog?: { [key: string]: string | undefined } | null;
}): void {
  const lang = normalizeAppLanguage(boot.language ?? "");
  language = lang;
  const next: Record<string, string> = {};
  for (const [k, v] of Object.entries(boot.catalog ?? {})) {
    if (typeof v === "string") {
      next[k] = v;
    }
  }
  catalog = next;
  if (typeof document !== "undefined") {
    document.documentElement.lang = lang;
  }
  emitLanguageChange();
}

/** Lightweight sprintf for %s / %d / %% (fmt-style, shared with Go Tf). */
export function sprintf(fmt: string, ...args: unknown[]): string {
  let i = 0;
  return fmt.replace(/%(%)|%([sd])/g, (_m, pct, verb) => {
    if (pct) return "%";
    if (i >= args.length) return "";
    const v = args[i++];
    if (verb === "d") return String(Number(v));
    return String(v ?? "");
  });
}

export function t(key: string, ...args: unknown[]): string {
  const msg = catalog[key] ?? key;
  if (args.length === 0) return msg;
  return sprintf(msg, ...args);
}

export function tf(key: string, ...args: unknown[]): string {
  return t(key, ...args);
}

export function displayBookmarkGroupName(name: string | null | undefined): string {
  const n = (name || "").trim();
  if (!n || n === DEFAULT_BOOKMARK_GROUP_ID || n === LEGACY_DEFAULT_GROUP) {
    return t("bookmark.group.default");
  }
  return n;
}

export function isDefaultBookmarkGroup(name: string | null | undefined): boolean {
  const n = (name || "").trim();
  return n === DEFAULT_BOOKMARK_GROUP_ID || n === LEGACY_DEFAULT_GROUP || n === "";
}

export function displaySessionTitle(title: string | null | undefined): string {
  const n = (title || "").trim();
  if (!n || n === NEW_SESSION_TITLE_ID || n === LEGACY_NEW_SESSION) {
    return t("ai.session.new");
  }
  return n;
}

/** Blank / numbered new-connection tab labels stored as stable ids. */
export function isNewConnectionTabName(name: string | null | undefined): boolean {
  const n = (name || "").trim();
  return !n || n === NEW_CONNECTION_TAB_ID || n.startsWith(`${NEW_CONNECTION_TAB_ID}:`);
}

export function displaySSHTabName(name: string | null | undefined): string {
  const n = (name || "").trim();
  if (!n || n === NEW_CONNECTION_TAB_ID) {
    return t("ssh.new_connection");
  }
  const prefix = `${NEW_CONNECTION_TAB_ID}:`;
  if (n.startsWith(prefix)) {
    return t("ssh.new_connection_n", n.slice(prefix.length));
  }
  return n;
}

export function newConnectionTabName(number?: number): string {
  if (number == null || number <= 1) {
    return NEW_CONNECTION_TAB_ID;
  }
  return `${NEW_CONNECTION_TAB_ID}:${number}`;
}

export function muiLocaleFor(lang: AppLanguage = language): Localization {
  switch (lang) {
    case AppLanguage.ZhCN:
      return zhCN;
    case AppLanguage.ZhTW:
      return zhTW;
    default:
      return enUS;
  }
}

export function formatDateTime(
  input: Date | number | string,
  lang: AppLanguage = language,
): string {
  const date =
    input instanceof Date
      ? input
      : typeof input === "number"
        ? new Date(input)
        : new Date(input);
  if (Number.isNaN(date.getTime())) {
    return String(input);
  }
  return date.toLocaleString(lang);
}

/** MUI X Chat localeText built from catalog (recompute after language change). */
export function buildChatLocaleText(translate: TranslateFn = t) {
  return {
    messageToolApproveButtonLabel: translate("ai.chat.approve"),
    messageToolDenyButtonLabel: translate("ai.chat.deny"),
    messageToolInputLabel: translate("ai.chat.tool_input"),
    messageToolOutputLabel: translate("ai.chat.tool_output"),
    messageCopyButtonLabel: translate("common.copy"),
    messageAuthorUserLabel: translate("ai.chat.user_you"),
    messageAuthorAssistantLabel: translate("ai.chat.user_assistant"),
    conversationHeaderMenuLabel: translate("ai.session.history"),
    conversationHeaderCloseLabel: translate("common.close"),
    conversationHeaderNewChatLabel: translate("ai.session.new"),
    conversationListLandmarkLabel: translate("ai.session.history"),
    composerInputPlaceholder: translate("ai.chat.placeholder"),
    toolStateLabel: (state: ChatToolInvocationState) => {
      switch (state) {
        case "input-streaming":
          return translate("ai.chat.state.preparing");
        case "input-available":
          return translate("ai.chat.state.pending");
        case "approval-requested":
          return translate("ai.chat.state.awaiting_approval");
        case "approval-responded":
          return translate("ai.chat.state.running");
        case "output-available":
          return translate("ai.chat.state.done");
        case "output-error":
          return translate("ai.chat.state.failed");
        case "output-denied":
          return translate("ai.chat.state.denied");
        default:
          return state;
      }
    },
  };
}

type BootstrapLoader = () => PromiseLike<{
  language?: string | null;
  catalog?: { [key: string]: string | undefined } | null;
}>;

let bootstrapLoader: BootstrapLoader | null = null;

/** Register GetI18nBootstrap (avoids circular import at module init). */
export function setI18nBootstrapLoader(loader: BootstrapLoader): void {
  bootstrapLoader = loader;
}

async function loadBootstrapOnce(force: boolean): Promise<void> {
  if (!force && bootstrapPromise) {
    return bootstrapPromise;
  }
  const loader = bootstrapLoader;
  if (!loader) {
    bootstrapPromise = Promise.resolve();
    return bootstrapPromise;
  }
  bootstrapPromise = Promise.resolve(loader())
    .then((boot) => {
      applyI18nBootstrap(boot);
    })
    .catch((err) => {
      console.error("GetI18nBootstrap failed:", err);
      bootstrapPromise = null;
      throw err;
    });
  return bootstrapPromise;
}

/** Idempotent catalog load for StrictMode / multi-call. */
export function ensureI18nBootstrap(): Promise<void> {
  return loadBootstrapOnce(false);
}

/** Reload after eventLanguage. */
export function reloadI18nBootstrap(): Promise<void> {
  return loadBootstrapOnce(true);
}
