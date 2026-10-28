import { t } from "../i18n";

export type ServiceErrorPayload = {
  code?: string;
  params?: string[];
  message?: string;
};

/** Extract Wails / RPC error text (may be JSON usererr payload). */
export const parseCallServiceError = (err: unknown): string => {
  if (err && typeof err === "object") {
    const obj = err as Record<string, unknown>;
    if ("message" in obj && typeof obj.message === "string") {
      return obj.message;
    }
    if ("error" in obj && typeof obj.error === "string") {
      return obj.error;
    }
  }
  return String(err ?? "");
};

/** Parse RPC user-error JSON `{code,params,message}` from a string. */
export const parseServiceErrorPayload = (
  raw: string,
): ServiceErrorPayload | null => {
  const s = raw.trim();
  if (!s.startsWith("{")) return null;
  try {
    const parsed = JSON.parse(s) as ServiceErrorPayload;
    if (
      parsed &&
      typeof parsed === "object" &&
      (typeof parsed.code === "string" || typeof parsed.message === "string")
    ) {
      return parsed;
    }
  } catch {
    // not JSON
  }
  return null;
};

/**
 * Translate a service error via catalog when `code` is present; else show message / raw.
 * Catalog is filled by GetI18nBootstrap (fe-runtime); until then falls back to English message.
 */
export const translateServiceError = (err: unknown): string => {
  const raw = parseCallServiceError(err).trim();
  const payload = parseServiceErrorPayload(raw);
  if (payload?.code) {
    const params = Array.isArray(payload.params) ? payload.params : [];
    const translated = t(payload.code, ...params);
    if (translated !== payload.code) {
      return translated;
    }
    if (payload.message) {
      return payload.message;
    }
    return payload.code;
  }
  if (payload?.message) {
    return payload.message;
  }
  return raw;
};

/** SSH connect failures: prefer coded usererr; no locale substring heuristics. */
export const formatSSHConnectError = (err: unknown): string => {
  const raw = parseCallServiceError(err).trim();
  const payload = parseServiceErrorPayload(raw);
  const code = payload?.code;
  if (
    code === "ssh.auth_cancelled" ||
    code === "ssh.auth_timeout" ||
    code === "ssh.auth_failed" ||
    code === "ssh.connect_failed"
  ) {
    return translateServiceError(err);
  }
  const msg = translateServiceError(err).trim();
  return msg || t("ssh.connect_failed");
};

// genTabIndex generate tab index ID
export const genTabIndex = (): string => {
  return `${Date.now()}`;
};

export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return "0 B";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
  if (bytes < 1024 * 1024 * 1024)
    return (bytes / (1024 * 1024)).toFixed(2) + " MB";
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + " GB";
};

export const sleep = (ms: number): Promise<void> => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};
