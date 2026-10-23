/** Clamp used/total to a 0–100 percentage. */
export const metricPct = (used: number, total: number): number => {
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, (used / total) * 100));
};
