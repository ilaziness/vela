import React from "react";
import {
  Box,
  IconButton,
  LinearProgress,
  Slide,
  Tooltip,
  Typography,
} from "@mui/material";
import { Close } from "@mui/icons-material";
import { HostResources } from "../../bindings/github.com/ilaziness/vexo/internal/ssh/models";
import { formatFileSize } from "../func/service";

interface HostResourcesPanelProps {
  open: boolean;
  statusBarHeight: string;
  resources: HostResources | null;
  pollStopped: boolean;
  onClose: () => void;
}

const pct = (used: number, total: number): number => {
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, (used / total) * 100));
};

const MetricRow: React.FC<{
  label: string;
  value: string;
  progress?: number;
}> = ({ label, value, progress }) => (
  <Box sx={{ mb: 1.5 }}>
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "baseline",
        mb: 0.5,
      }}
    >
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" sx={{ fontWeight: 500 }}>
        {value}
      </Typography>
    </Box>
    {progress !== undefined && (
      <LinearProgress
        variant="determinate"
        value={progress}
        sx={{ height: 6, borderRadius: 1 }}
      />
    )}
  </Box>
);

const HostResourcesPanel: React.FC<HostResourcesPanelProps> = ({
  open,
  statusBarHeight,
  resources,
  pollStopped,
  onClose,
}) => {
  const title = resources?.hostname
    ? `主机资源 · ${resources.hostname}`
    : "主机资源";

  let body: React.ReactNode;
  if (resources?.supported) {
    body = (
      <>
        {resources.has_cpu && (
          <MetricRow
            label="CPU"
            value={`${Math.round(resources.cpu_percent ?? 0)}%`}
            progress={resources.cpu_percent ?? 0}
          />
        )}
        {resources.has_mem && (
          <MetricRow
            label="内存"
            value={`${formatFileSize(resources.mem_used ?? 0)} / ${formatFileSize(resources.mem_total ?? 0)}`}
            progress={pct(resources.mem_used ?? 0, resources.mem_total ?? 0)}
          />
        )}
        {resources.has_disk && (
          <MetricRow
            label="磁盘 (/)"
            value={`${formatFileSize(resources.disk_used ?? 0)} / ${formatFileSize(resources.disk_total ?? 0)}`}
            progress={pct(resources.disk_used ?? 0, resources.disk_total ?? 0)}
          />
        )}
        {resources.has_load && (
          <MetricRow
            label="Load (1m)"
            value={(resources.load1 ?? 0).toFixed(2)}
          />
        )}
        <MetricRow
          label="延迟 (SSH)"
          value={`${resources.latency_ms ?? 0} ms`}
        />
      </>
    );
  } else if (resources && !resources.supported) {
    body = (
      <>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          当前远程系统不支持 CPU/内存/磁盘采集（需 Linux /proc）
        </Typography>
        <MetricRow
          label="延迟 (SSH)"
          value={`${resources.latency_ms ?? 0} ms`}
        />
      </>
    );
  } else if (pollStopped) {
    body = (
      <Typography variant="body2" color="text.secondary">
        无法获取主机资源
      </Typography>
    );
  } else {
    body = (
      <Typography variant="body2" color="text.secondary">
        正在采集主机资源…
      </Typography>
    );
  }

  return (
    <Box
      sx={{
        position: "absolute",
        bottom: statusBarHeight,
        left: 0,
        right: 0,
        height: "50%",
        zIndex: 10,
        overflow: "hidden",
        pointerEvents: open ? "auto" : "none",
      }}
    >
      <Slide in={open} direction="up" mountOnEnter unmountOnExit>
        <Box
          sx={{
            height: "100%",
            display: "flex",
            flexDirection: "column",
            bgcolor: "background.default",
            borderTop: 1,
            borderColor: "divider",
          }}
        >
          <Box
            sx={{
              p: 1,
              px: 2,
              borderBottom: 1,
              borderColor: "divider",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              height: "40px",
            }}
          >
            <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
              {title}
            </Typography>
            <Tooltip title="关闭">
              <IconButton onClick={onClose} size="small">
                <Close fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>

          <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>
            {body}
          </Box>
        </Box>
      </Slide>
    </Box>
  );
};

export default HostResourcesPanel;
