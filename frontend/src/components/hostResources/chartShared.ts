export const chartFrameSx = {
  width: "100%",
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
  bgcolor: "action.hover",
  overflow: "hidden",
} as const;

export const chartPlotSx = {
  "& .MuiChartsAxis-root": { display: "none" },
  "& .MuiChartsGrid-line": { strokeDasharray: "2 4" },
} as const;

export const CHART_HEIGHT = 88;
