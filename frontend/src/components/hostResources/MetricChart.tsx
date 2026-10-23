import { Box, Typography } from "@mui/material";
import { LineChart } from "@mui/x-charts/LineChart";
import { HOST_METRIC_HISTORY_MAX } from "../../hooks/useHostMetricHistory";
import { CHART_HEIGHT, chartFrameSx, chartPlotSx } from "./chartShared";

interface MetricChartProps {
  label: string;
  value: string;
  data: number[];
  color: string;
  seriesId: string;
}

const MetricChart: React.FC<MetricChartProps> = ({
  label,
  value,
  data,
  color,
  seriesId,
}) => {
  const xData = data.map((_, i) => i);

  return (
    <Box sx={{ minWidth: 0, width: "100%" }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "baseline",
          mb: 0.5,
          gap: 1,
        }}
      >
        <Typography variant="body2" color="text.secondary">
          {label}
        </Typography>
        <Typography variant="body2" sx={{ fontWeight: 500 }}>
          {value}
        </Typography>
      </Box>
      <Box sx={{ ...chartFrameSx, height: CHART_HEIGHT }}>
        {data.length > 0 ? (
          <LineChart
            xAxis={[
              {
                data: xData,
                min: 0,
                max: Math.max(HOST_METRIC_HISTORY_MAX - 1, data.length - 1, 1),
                hideTooltip: true,
              },
            ]}
            yAxis={[{ min: 0, max: 100, width: 0 }]}
            series={[
              {
                id: seriesId,
                data,
                area: true,
                showMark: false,
                color,
                valueFormatter: (v) =>
                  v == null ? "" : `${Math.round(v)}%`,
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

export default MetricChart;
