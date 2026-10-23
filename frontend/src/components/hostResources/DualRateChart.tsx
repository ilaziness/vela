import { Box, Typography } from "@mui/material";
import { LineChart } from "@mui/x-charts/LineChart";
import { HOST_METRIC_HISTORY_MAX } from "../../hooks/useHostMetricHistory";
import { CHART_HEIGHT, chartFrameSx, chartPlotSx } from "./chartShared";
import { formatRate } from "./format";

interface DualRateChartProps {
  title: string;
  seriesId: string;
  aBps: number;
  bBps: number;
  aHistory: number[];
  bHistory: number[];
  aLabel: string;
  bLabel: string;
  aColor: string;
  bColor: string;
}

const DualRateChart: React.FC<DualRateChartProps> = ({
  title,
  seriesId,
  aBps,
  bBps,
  aHistory,
  bHistory,
  aLabel,
  bLabel,
  aColor,
  bColor,
}) => {
  const len = Math.max(aHistory.length, bHistory.length);
  const xData = Array.from({ length: len }, (_, i) => i);
  const yMax = Math.max(1, ...aHistory, ...bHistory);

  return (
    <Box sx={{ minWidth: 0, width: "100%" }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "baseline",
          mb: 0.5,
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          {title}
        </Typography>
        <Typography variant="body2" sx={{ fontWeight: 500 }}>
          {aLabel} {formatRate(aBps)} · {bLabel} {formatRate(bBps)}
        </Typography>
      </Box>
      <Box sx={{ ...chartFrameSx, height: CHART_HEIGHT }}>
        {len > 0 ? (
          <LineChart
            xAxis={[
              {
                data: xData,
                min: 0,
                max: Math.max(HOST_METRIC_HISTORY_MAX - 1, len - 1, 1),
                hideTooltip: true,
              },
            ]}
            yAxis={[{ min: 0, max: yMax, width: 0 }]}
            series={[
              {
                id: `${seriesId}-a`,
                data: aHistory,
                area: true,
                showMark: false,
                color: aColor,
                label: aLabel,
                valueFormatter: (v) => (v == null ? "" : formatRate(v)),
              },
              {
                id: `${seriesId}-b`,
                data: bHistory,
                area: true,
                showMark: false,
                color: bColor,
                label: bLabel,
                valueFormatter: (v) => (v == null ? "" : formatRate(v)),
              },
            ]}
            height={CHART_HEIGHT}
            margin={{ top: 8, right: 8, bottom: 8, left: 8 }}
            skipAnimation
            hideLegend
            grid={{ horizontal: true }}
            sx={chartPlotSx}
          />
        ) : null}
      </Box>
    </Box>
  );
};

export default DualRateChart;
