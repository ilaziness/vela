import { Box, Stack, useTheme } from "@mui/material";
import { HostResources } from "../../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { formatFileSize } from "../../func/service";
import type { MetricHistory } from "../../hooks/useHostMetricHistory";
import CompactStat from "./CompactStat";
import DiskUsageRow from "./DiskUsageRow";
import DualRateChart from "./DualRateChart";
import MetricChart from "./MetricChart";
import { pct } from "./format";
import { t, tf } from "../../i18n";

interface HostResourcesContentProps {
  resources: HostResources;
  history: MetricHistory;
}

const twoColGridSx = {
  display: "grid",
  gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
  gap: 2,
} as const;

const HostResourcesContent: React.FC<HostResourcesContentProps> = ({
  resources,
  history,
}) => {
  const theme = useTheme();
  const netIfaces = (resources.net_ifaces ?? []).filter((iface) => !!iface?.name);
  const diskIOList = (resources.disk_io ?? []).filter((disk) => !!disk?.name);

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
          progress={pct(resources.disk_used ?? 0, resources.disk_total ?? 0)}
        />
      )}

      {(resources.has_cpu || resources.has_mem) && (
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns:
              resources.has_cpu && resources.has_mem
                ? "repeat(2, minmax(0, 1fr))"
                : "minmax(0, 1fr)",
            gap: 2,
          }}
        >
          {resources.has_cpu && (
            <MetricChart
              label="CPU"
              value={`${Math.round(resources.cpu_percent ?? 0)}%`}
              data={history.cpu}
              color={theme.palette.primary.main}
              seriesId="cpu"
            />
          )}
          {resources.has_mem && (
            <MetricChart
              label={t("host.memory")}
              value={`${formatFileSize(resources.mem_used ?? 0)} / ${formatFileSize(resources.mem_total ?? 0)}`}
              data={history.mem}
              color={theme.palette.secondary.main}
              seriesId="mem"
            />
          )}
        </Box>
      )}

      {(netIfaces.length > 0 || diskIOList.length > 0) && (
        <Box sx={twoColGridSx}>
          {netIfaces.map((iface) => {
            const name = iface!.name!;
            const netHist = history.net[name];
            return (
              <DualRateChart
                key={`net-${name}`}
                title={tf("host.network", name)}
                seriesId={`net-${name}`}
                aBps={iface!.rx_bps ?? 0}
                bBps={iface!.tx_bps ?? 0}
                aHistory={netHist?.a ?? []}
                bHistory={netHist?.b ?? []}
                aLabel="↓"
                bLabel="↑"
                aColor={theme.palette.primary.main}
                bColor={theme.palette.secondary.main}
              />
            );
          })}
          {diskIOList.map((disk) => {
            const name = disk!.name!;
            const hist = history.diskIO[name];
            return (
              <DualRateChart
                key={`disk-${name}`}
                title={tf("host.disk_io", name)}
                seriesId={`disk-${name}`}
                aBps={disk!.read_bps ?? 0}
                bBps={disk!.write_bps ?? 0}
                aHistory={hist?.a ?? []}
                bHistory={hist?.b ?? []}
                aLabel={t("host.read")}
                bLabel={t("host.write")}
                aColor={theme.palette.primary.main}
                bColor={theme.palette.secondary.main}
              />
            );
          })}
        </Box>
      )}
    </Stack>
  );
};

export default HostResourcesContent;
