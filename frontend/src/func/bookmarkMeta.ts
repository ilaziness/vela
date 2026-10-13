import { BookmarkColor, BOOKMARK_COLOR_OPTIONS } from "../types/bookmark";

const MAX_TAG_RUNES = 32;
const MAX_TAG_COUNT = 16;

const COLOR_HEX: Record<string, string> = {
  [BookmarkColor.Red]: "#e53935",
  [BookmarkColor.Orange]: "#fb8c00",
  [BookmarkColor.Yellow]: "#fdd835",
  [BookmarkColor.Green]: "#43a047",
  [BookmarkColor.Cyan]: "#00acc1",
  [BookmarkColor.Blue]: "#1e88e5",
  [BookmarkColor.Purple]: "#8e24aa",
  [BookmarkColor.Pink]: "#d81b60",
};

export type BookmarkMetaFields = {
  id?: string | null;
  title?: string | null;
  host?: string | null;
  user?: string | null;
  tags?: string[] | null;
  color?: string | null;
};

export type BookmarkGroupLike<T extends BookmarkMetaFields = BookmarkMetaFields> = {
  name: string;
  icon?: string | null;
  bookmarks?: (T | null)[] | null;
};

export function bookmarkColorHex(color: string | null | undefined): string | null {
  const token = normalizeBookmarkColor(color);
  if (!token) {
    return null;
  }
  return COLOR_HEX[token] ?? null;
}

export function normalizeBookmarkColor(color: string | null | undefined): string {
  const c = (color || "").trim().toLowerCase();
  if (!c) {
    return BookmarkColor.None;
  }
  return BOOKMARK_COLOR_OPTIONS.includes(c as BookmarkColor) ? c : BookmarkColor.None;
}

export function normalizeBookmarkTags(tags: string[] | null | undefined): string[] {
  if (!tags || tags.length === 0) {
    return [];
  }
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of tags) {
    const t = (raw || "").trim();
    if (!t || /[\n\r\u0000-\u001f\u007f]/.test(t)) {
      continue;
    }
    const clipped =
      Array.from(t).length > MAX_TAG_RUNES
        ? Array.from(t).slice(0, MAX_TAG_RUNES).join("")
        : t;
    const key = clipped.toLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(clipped);
    if (out.length >= MAX_TAG_COUNT) {
      break;
    }
  }
  return out;
}

export function collectTagSuggestions<T extends BookmarkMetaFields>(
  groups: BookmarkGroupLike<T>[],
): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const g of groups) {
    for (const b of g.bookmarks || []) {
      if (!b) {
        continue;
      }
      for (const tag of b.tags || []) {
        const t = (tag || "").trim();
        if (!t) {
          continue;
        }
        const key = t.toLowerCase();
        if (seen.has(key)) {
          continue;
        }
        seen.add(key);
        out.push(t);
      }
    }
  }
  out.sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
  return out;
}

function matchesQuery(b: BookmarkMetaFields, query: string): boolean {
  if (!query) {
    return true;
  }
  const q = query.toLowerCase();
  const fields = [b.title, b.host, b.user, ...(b.tags || [])];
  return fields.some((f) => (f || "").toLowerCase().includes(q));
}

function matchesTag(b: BookmarkMetaFields, tag: string): boolean {
  if (!tag) {
    return true;
  }
  const key = tag.toLowerCase();
  return (b.tags || []).some((t) => (t || "").toLowerCase() === key);
}

export function filterBookmarkGroups<G extends BookmarkGroupLike>(
  groups: G[],
  query: string,
  selectedTag: string,
): G[] {
  const q = query.trim();
  const tag = selectedTag.trim();
  const filtering = Boolean(q || tag);
  return groups
    .map((g) => {
      const bookmarks = (g.bookmarks || []).filter(
        (b) => b != null && matchesQuery(b, q) && matchesTag(b, tag),
      );
      return { ...g, bookmarks };
    })
    .filter((g) => !filtering || (g.bookmarks?.length ?? 0) > 0);
}

export function isBookmarkInFilteredGroups<T extends BookmarkMetaFields>(
  groups: BookmarkGroupLike<T>[],
  bookmarkId: string | null | undefined,
): boolean {
  if (!bookmarkId) {
    return false;
  }
  return groups.some((g) =>
    (g.bookmarks || []).some((b) => b?.id === bookmarkId),
  );
}
