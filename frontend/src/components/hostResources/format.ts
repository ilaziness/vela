import { metricPct } from "../../func/hostMetrics";
import { formatFileSize } from "../../func/service";

export { metricPct as pct };

export const formatRate = (bps: number): string =>
  `${formatFileSize(Math.max(0, bps))}/s`;
