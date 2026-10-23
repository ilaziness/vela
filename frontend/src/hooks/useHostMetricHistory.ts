import { useState } from "react";
import { HostResources } from "../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { metricPct } from "../func/hostMetrics";

export const HOST_METRIC_HISTORY_MAX = 60;

export interface DualRateHistory {
  a: number[];
  b: number[];
}

export interface MetricHistory {
  cpu: number[];
  mem: number[];
  net: Record<string, DualRateHistory>;
  diskIO: Record<string, DualRateHistory>;
}

const emptyHistory = (): MetricHistory => ({
  cpu: [],
  mem: [],
  net: {},
  diskIO: {},
});

const pushCapped = (series: number[], value: number): number[] => {
  const next = [...series, value];
  if (next.length > HOST_METRIC_HISTORY_MAX) next.shift();
  return next;
};

export const appendHostMetricHistory = (
  prev: MetricHistory,
  resources: HostResources,
): MetricHistory => {
  let cpu = prev.cpu;
  let mem = prev.mem;
  let net = prev.net;
  let diskIO = prev.diskIO;

  if (resources.has_cpu) {
    cpu = pushCapped(
      cpu,
      Math.min(100, Math.max(0, resources.cpu_percent ?? 0)),
    );
  }
  if (resources.has_mem) {
    mem = pushCapped(
      mem,
      metricPct(resources.mem_used ?? 0, resources.mem_total ?? 0),
    );
  }
  if (resources.has_net && (resources.net_ifaces?.length ?? 0) > 0) {
    net = {};
    for (const iface of resources.net_ifaces ?? []) {
      if (!iface?.name) continue;
      const prevIface = prev.net[iface.name] ?? { a: [], b: [] };
      net[iface.name] = {
        a: pushCapped(prevIface.a, Math.max(0, iface.rx_bps ?? 0)),
        b: pushCapped(prevIface.b, Math.max(0, iface.tx_bps ?? 0)),
      };
    }
  } else if (!resources.has_net) {
    net = {};
  }
  if (resources.has_disk_io && (resources.disk_io?.length ?? 0) > 0) {
    diskIO = {};
    for (const disk of resources.disk_io ?? []) {
      if (!disk?.name) continue;
      const prevDisk = prev.diskIO[disk.name] ?? { a: [], b: [] };
      diskIO[disk.name] = {
        a: pushCapped(prevDisk.a, Math.max(0, disk.read_bps ?? 0)),
        b: pushCapped(prevDisk.b, Math.max(0, disk.write_bps ?? 0)),
      };
    }
  } else if (!resources.has_disk_io) {
    diskIO = {};
  }

  return { cpu, mem, net, diskIO };
};

/**
 * Keeps a rolling metric history keyed by session.
 * State is adjusted during render so the first paint after a snapshot already has data.
 */
export function useHostMetricHistory(
  sessionID: string,
  resources: HostResources | null,
): MetricHistory {
  const [history, setHistory] = useState<MetricHistory>(emptyHistory);
  const [trackedSessionID, setTrackedSessionID] = useState(sessionID);
  const [trackedResources, setTrackedResources] =
    useState<HostResources | null>(null);

  if (trackedSessionID !== sessionID) {
    setTrackedSessionID(sessionID);
    // Mark current props as seen without appending — may be a stale prior-session snapshot.
    setTrackedResources(resources);
    setHistory(emptyHistory());
  } else if (resources !== trackedResources) {
    setTrackedResources(resources);
    if (resources?.supported) {
      setHistory((prev) => appendHostMetricHistory(prev, resources));
    }
  }

  return history;
}
