import { Box, Typography } from "@mui/material";
import { chartsGridClasses } from "@mui/x-charts/ChartsGrid";
import { LineChart, lineClasses } from "@mui/x-charts/LineChart";
import type { XAxis, YAxis } from "@mui/x-charts/models";
import { HOST_METRIC_WINDOW_MS } from "../../hooks/useHostMetricHistory";
import { chartFrameSx } from "./chartShared";

const CHART_HEIGHT = 96;

interface HostLineSeries {
  id: string;
  data: number[];
  color: string;
  label?: string;
  valueFormatter: (v: number | null) => string | null;
}

interface HostLineChartProps {
  title: string;
  value: string;
  times: number[];
  yMax: number;
  series: HostLineSeries[];
}

const pad2 = (n: number) => String(n).padStart(2, "0");
const MINUTE_MS = 60_000;

const minuteTicks = (startMs: number, endMs: number): Date[] => {
  const ticks: Date[] = [];
  let t = Math.ceil(startMs / MINUTE_MS) * MINUTE_MS;
  for (; t <= endMs; t += MINUTE_MS) {
    ticks.push(new Date(t));
  }
  return ticks;
};

const timeXAxis = (timesMs: number[]): XAxis<"time"> => {
  const t0 = timesMs[0];
  const t1 = t0 + HOST_METRIC_WINDOW_MS;
  return {
    data: timesMs.map((t) => new Date(t)),
    scaleType: "time",
    min: new Date(t0),
    max: new Date(t1),
    height: "auto",
    tickInterval: minuteTicks(t0, t1),
    tickSize: 4,
    hideTooltip: true,
    valueFormatter: (value) =>
      `${pad2(value.getHours())}:${pad2(value.getMinutes())}:${pad2(value.getSeconds())}`,
    tickLabelStyle: { fontSize: 10 },
  };
};

const hiddenYAxis = (max: number): YAxis => ({
  position: "none",
  min: 0,
  max,
  tickInterval: [max / 2],
});

const HostLineChart: React.FC<HostLineChartProps> = ({
  title,
  value,
  times,
  yMax,
  series,
}) => {
  const len = series[0].data.length;

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
          {value}
        </Typography>
      </Box>
      <Box sx={{ ...chartFrameSx, height: CHART_HEIGHT }}>
        {len > 0 ? (
          <LineChart
            height={CHART_HEIGHT}
            margin={{ top: 2, right: 0, bottom: 2, left: 0 }}
            skipAnimation
            hideLegend
            grid={{ horizontal: true }}
            sx={{
              [`& .${chartsGridClasses.line}`]: { strokeDasharray: "2 4" },
              [`& .${lineClasses.area}`]: { opacity: 0.28 },
            }}
            xAxis={[timeXAxis(times.slice(-len))]}
            yAxis={[hiddenYAxis(yMax)]}
            series={series.map((s) => ({
              ...s,
              area: true,
              showMark: false,
            }))}
          />
        ) : null}
      </Box>
    </Box>
  );
};

export default HostLineChart;
