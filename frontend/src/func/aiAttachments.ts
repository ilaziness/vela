/** Shared limits / MIME allowlists for AI chat attachments. */

export const AI_ATTACH_MIME_TYPES = [
  "image/png",
  "image/jpeg",
  "image/webp",
  "image/gif",
  "application/pdf",
  "text/plain",
  "text/markdown",
  "text/csv",
] as const;

export const AI_PASTE_MIME_TYPES = [
  "image/png",
  "image/jpeg",
  "image/webp",
  "image/gif",
] as const;

export const AI_ATTACH_MAX_COUNT = 5;
export const AI_ATTACH_MAX_BYTES = 5 * 1024 * 1024;

function normalizeMime(mime: string): string {
  const base = (mime || "").split(";")[0].trim().toLowerCase();
  return base === "image/jpg" ? "image/jpeg" : base;
}

function matchesAttachMime(mime: string, allow: readonly string[]): boolean {
  const m = normalizeMime(mime);
  return m !== "" && (allow as readonly string[]).includes(m);
}

export type AttachmentRejectReason = "mime-type" | "file-size" | "file-count";

export interface AttachmentRejection {
  file: File;
  reason: AttachmentRejectReason;
}

/** Validate and return files that pass; collect rejections. */
export function filterAttachFiles(
  files: File[],
  opts: {
    allow?: readonly string[];
    existingCount?: number;
    maxCount?: number;
    maxBytes?: number;
  } = {},
): { accepted: File[]; rejections: AttachmentRejection[] } {
  const allow = opts.allow ?? AI_ATTACH_MIME_TYPES;
  const maxCount = opts.maxCount ?? AI_ATTACH_MAX_COUNT;
  const maxBytes = opts.maxBytes ?? AI_ATTACH_MAX_BYTES;
  let remaining = Math.max(0, maxCount - (opts.existingCount ?? 0));
  const accepted: File[] = [];
  const rejections: AttachmentRejection[] = [];

  for (const file of files) {
    if (!matchesAttachMime(file.type, allow)) {
      rejections.push({ file, reason: "mime-type" });
      continue;
    }
    if (file.size > maxBytes) {
      rejections.push({ file, reason: "file-size" });
      continue;
    }
    if (remaining <= 0) {
      rejections.push({ file, reason: "file-count" });
      continue;
    }
    accepted.push(file);
    remaining -= 1;
  }
  return { accepted, rejections };
}

export function formatAttachmentRejectMessage(
  rejections: Array<{ reason: AttachmentRejectReason }>,
): string {
  if (rejections.length === 0) return "";
  const reasons = new Set(rejections.map((r) => r.reason));
  if (reasons.has("file-count")) {
    return `最多只能添加 ${AI_ATTACH_MAX_COUNT} 个附件`;
  }
  if (reasons.has("file-size")) {
    return `单个附件不能超过 ${Math.round(AI_ATTACH_MAX_BYTES / (1024 * 1024))} MB`;
  }
  return "不支持的文件类型";
}

/** Collect image Files from a paste event. Prefer DataTransferItem.type (more reliable than File.type). */
export function imagesFromClipboard(data: DataTransfer | null): File[] {
  if (!data) return [];
  const out: File[] = [];
  for (const item of Array.from(data.items || [])) {
    if (item.kind !== "file" || !matchesAttachMime(item.type, AI_PASTE_MIME_TYPES)) continue;
    const file = item.getAsFile();
    if (file) out.push(file);
  }
  if (out.length > 0) return out;
  for (const file of Array.from(data.files || [])) {
    if (matchesAttachMime(file.type, AI_PASTE_MIME_TYPES)) out.push(file);
  }
  return out;
}

const localPathByFile = new WeakMap<File, string>();

export function rememberLocalFilePath(file: File, path: string) {
  localPathByFile.set(file, path);
}

export function localFilePath(file: File): string {
  return localPathByFile.get(file) || (file as File & { path?: string }).path || "";
}

export async function dataURLToFile(
  dataURL: string,
  filename: string,
  mime: string,
): Promise<File> {
  const resp = await fetch(dataURL);
  const blob = await resp.blob();
  return new File([blob], filename, { type: mime || blob.type || "application/octet-stream" });
}

export function fileToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") resolve(reader.result);
      else reject(new Error("failed to read file"));
    };
    reader.onerror = () => reject(reader.error ?? new Error("failed to read file"));
    reader.readAsDataURL(blob);
  });
}
