import type { Theme } from "@mui/material/styles";

export const chartFrameSx = {
  width: "100%",
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
  bgcolor: "action.hover",
  overflow: "hidden",
} as const;

export const hostChartColors = (theme: Theme) => {
  const { primary, secondary, warning, error } = theme.palette;
  return {
    cpu: primary.main,
    mem: warning.main,
    netRx: primary.main,
    netTx: warning.main,
    diskRead: secondary.main,
    diskWrite: error.main,
  };
};
