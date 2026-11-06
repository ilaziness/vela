import { Box, Stack, useTheme } from "@mui/material";
import { HostResources } from "../../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { metricPct } from "../../func/hostMetrics";
import { formatFileSize } from "../../func/service";
import type { MetricHistory } from "../../hooks/useHostMetricHistory";
import CompactStat from "./CompactStat";
import DiskUsageRow from "./DiskUsageRow";
import HostLineChart from "./HostLineChart";
import { hostChartColors } from "./chartShared";
import { formatPctTick, formatRate, formatRateTick } from "./format";
import { t, tf } from "../../i18n";

interface HostResourcesContentProps {
  resources: HostResources;
  history: MetricHistory;
}

const chartsGridSx = (twoCols: boolean) => ({
  display: "grid",
  gridTemplateColumns: twoCols ? "repeat(2, minmax(0, 1fr))" : "minmax(0, 1fr)",
  gap: 2,
});

const HostResourcesContent: React.FC<HostResourcesContentProps> = ({
  resources,
  history,
}) => {
  const theme = useTheme();
  const colors = hostChartColors(theme);
  const netIfaces = (resources.net_ifaces ?? []).filter((iface) => !!iface?.name);
  const diskIOList = (resources.disk_io ?? []).filter((disk) => !!disk?.name);
  const readLabel = t("host.read");
  const writeLabel = t("host.write");

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={3} useFlexGap sx={{ flexWrap: "wrap" }}>
        {resources.has_load && (
          <CompactStat
            label="Load (1m)"
            value={(resources.load1 ?? 0).toFixed(2)}
          />
        )}
        <CompactStat
          label={t("host.latency_ssh")}
          value={`${resources.latency_ms ?? 0} ms`}
        />
      </Stack>

      {resources.has_disk && (
        <DiskUsageRow
          label={t("host.disk_root")}
          value={`${formatFileSize(resources.disk_used ?? 0)} / ${formatFileSize(resources.disk_total ?? 0)}`}
          progress={metricPct(resources.disk_used ?? 0, resources.disk_total ?? 0)}
        />
      )}

      {(resources.has_cpu || resources.has_mem) && (
        <Box sx={chartsGridSx(!!(resources.has_cpu && resources.has_mem))}>
          {resources.has_cpu && (
            <HostLineChart
              title="CPU"
              value={`${Math.round(resources.cpu_percent ?? 0)}%`}
              times={history.times}
              yMax={100}
              series={[
                {
                  id: "cpu",
                  data: history.cpu,
                  color: colors.cpu,
                  valueFormatter: formatPctTick,
                },
              ]}
            />
          )}
          {resources.has_mem && (
            <HostLineChart
              title={t("host.memory")}
              value={`${formatFileSize(resources.mem_used ?? 0)} / ${formatFileSize(resources.mem_total ?? 0)}`}
              times={history.times}
              yMax={100}
              series={[
                {
                  id: "mem",
                  data: history.mem,
                  color: colors.mem,
                  valueFormatter: formatPctTick,
                },
              ]}
            />
          )}
        </Box>
      )}

      {(netIfaces.length > 0 || diskIOList.length > 0) && (
        <Box sx={chartsGridSx(netIfaces.length + diskIOList.length > 1)}>
          {netIfaces.map((iface) => {
            const name = iface!.name!;
            const a = history.net[name]?.a ?? [];
            const b = history.net[name]?.b ?? [];
            return (
              <HostLineChart
                key={`net-${name}`}
                title={tf("host.network", name)}
                value={`↓ ${formatRate(iface!.rx_bps ?? 0)} · ↑ ${formatRate(iface!.tx_bps ?? 0)}`}
                times={history.times}
                yMax={Math.max(1, ...a, ...b)}
                series={[
                  {
                    id: `net-${name}-a`,
                    data: a,
                    color: colors.netRx,
                    label: "↓",
                    valueFormatter: formatRateTick,
                  },
                  {
                    id: `net-${name}-b`,
                    data: b,
                    color: colors.netTx,
                    label: "↑",
                    valueFormatter: formatRateTick,
                  },
                ]}
              />
            );
          })}
          {diskIOList.map((disk) => {
            const name = disk!.name!;
            const a = history.diskIO[name]?.a ?? [];
            const b = history.diskIO[name]?.b ?? [];
            return (
              <HostLineChart
                key={`disk-${name}`}
                title={tf("host.disk_io", name)}
                value={`${readLabel} ${formatRate(disk!.read_bps ?? 0)} · ${writeLabel} ${formatRate(disk!.write_bps ?? 0)}`}
                times={history.times}
                yMax={Math.max(1, ...a, ...b)}
                series={[
                  {
                    id: `disk-${name}-a`,
                    data: a,
                    color: colors.diskRead,
                    label: readLabel,
                    valueFormatter: formatRateTick,
                  },
                  {
                    id: `disk-${name}-b`,
                    data: b,
                    color: colors.diskWrite,
                    label: writeLabel,
                    valueFormatter: formatRateTick,
                  },
                ]}
              />
            );
          })}
        </Box>
      )}
    </Stack>
  );
};

export default HostResourcesContent;
