import { formatFileSize } from "../../func/service";

export const formatRate = (bps: number): string =>
  `${formatFileSize(Math.max(0, bps))}/s`;

export const formatPctTick = (v: number | null) =>
  v == null ? null : `${Math.round(v)}%`;

export const formatRateTick = (v: number | null) =>
  v == null ? null : formatRate(v);
