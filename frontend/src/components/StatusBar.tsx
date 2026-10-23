import React from "react";
import { Box, Stack, Tooltip, Typography } from "@mui/material";
import MobiledataOffIcon from "@mui/icons-material/MobiledataOff";
import AltRouteIcon from "@mui/icons-material/AltRoute";
import FiberManualRecordIcon from "@mui/icons-material/FiberManualRecord";
import TransferList from "./TransferList";
import SSHTunnel from "./SSHTunnel";
import HostResourcesPanel from "./HostResourcesPanel";
import { SSHService } from "../../bindings/github.com/ilaziness/vela/services";
import { HostResources } from "../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { parseCallServiceError } from "../func/service";

const HOST_RESOURCES_POLL_MS = 5000;
/** Wait after terminal WS ready so PTY welcome/MOTD is not stolen by metrics Exec. */
const HOST_RESOURCES_INITIAL_DELAY_MS = 1500;

interface StatusBarProps {
  sessionID: string;
  height: string;
  isActive: boolean;
  termReady: boolean;
  logging?: boolean;
  sftpReady?: boolean;
}

const formatCompactBytes = (n: number): string => {
  if (!n || n <= 0) return "0";
  const gib = 1024 ** 3;
  const mib = 1024 ** 2;
  if (n >= gib) return `${(n / gib).toFixed(1)}G`;
  if (n >= mib) return `${Math.round(n / mib)}M`;
  return `${Math.round(n / 1024)}K`;
};

const formatMetricsLabel = (
  r: HostResources | null,
  pollStopped: boolean,
): string => {
  if (pollStopped) return "无法获取";
  if (!r) return "CPU -- · Mem -- · Disk -- · --ms";
  const parts: string[] = [];
  if (r.has_cpu) {
    parts.push(`CPU ${Math.round(r.cpu_percent ?? 0)}%`);
  } else {
    parts.push("CPU --");
  }
  if (r.has_mem && (r.mem_total ?? 0) > 0) {
    parts.push(
      `Mem ${formatCompactBytes(r.mem_used ?? 0)}/${formatCompactBytes(r.mem_total ?? 0)}`,
    );
  } else {
    parts.push("Mem --");
  }
  if (r.has_disk && (r.disk_total ?? 0) > 0) {
    const pct = Math.round(((r.disk_used ?? 0) / (r.disk_total ?? 1)) * 100);
    parts.push(`Disk ${pct}%`);
  } else {
    parts.push("Disk --");
  }
  parts.push(`${r.latency_ms ?? 0}ms`);
  return parts.join(" · ");
};

const isSessionGoneError = (err: unknown): boolean => {
  const msg = parseCallServiceError(err).toLowerCase();
  return msg.includes("ssh connection") && msg.includes("not found");
};

const StatusBar: React.FC<StatusBarProps> = ({
  sessionID,
  height,
  isActive,
  termReady,
  logging = false,
  sftpReady = false,
}) => {
  const [open, setOpen] = React.useState(false);
  const [sshTunnelOpen, setSshTunnelOpen] = React.useState(false);
  const [resourcesOpen, setResourcesOpen] = React.useState(false);
  const [resources, setResources] = React.useState<HostResources | null>(null);
  const [pollStopped, setPollStopped] = React.useState(false);
  const [trackedSessionID, setTrackedSessionID] = React.useState(sessionID);
  const needsInitialDelayRef = React.useRef(true);
  /** Latest session; in-flight polls compare against this after await (cleanup may not have run yet). */
  const pollSessionRef = React.useRef(sessionID);
  pollSessionRef.current = sessionID;

  // Reset synchronously on session change so children never see (newSession, oldResources).
  if (trackedSessionID !== sessionID) {
    setTrackedSessionID(sessionID);
    setResources(null);
    setPollStopped(false);
    setResourcesOpen(false);
    needsInitialDelayRef.current = true;
  }

  const toggleOpen = () => {
    setOpen((prev) => !prev);
  };
  const handleClose = () => {
    setOpen(false);
  };

  const toggleSshTunnelOpen = () => {
    setSshTunnelOpen((prev) => !prev);
  };
  const handleSshTunnelClose = () => {
    setSshTunnelOpen(false);
  };

  const toggleResourcesOpen = () => {
    setResourcesOpen((prev) => !prev);
  };
  const handleResourcesClose = () => {
    setResourcesOpen(false);
  };

  React.useEffect(() => {
    if (!isActive || !sessionID || !termReady || pollStopped) {
      return;
    }

    let cancelled = false;
    let stopped = false;
    let timer: number | undefined;

    const scheduleNext = (delayMs: number) => {
      if (cancelled || stopped) return;
      timer = window.setTimeout(() => {
        void fetchOnce();
      }, delayMs);
    };

    const forSession = sessionID;

    const fetchOnce = async () => {
      if (cancelled || stopped) return;
      needsInitialDelayRef.current = false;
      try {
        const res = await SSHService.GetHostResources(forSession);
        if (cancelled || stopped) return;
        // Guard: await may resolve before effect cleanup flips `cancelled` on session switch.
        if (pollSessionRef.current !== forSession) return;
        setResources(res);
      } catch (err) {
        if (cancelled || stopped) return;
        if (pollSessionRef.current !== forSession) return;
        if (isSessionGoneError(err)) {
          stopped = true;
          setPollStopped(true);
          setResources(null);
          return;
        }
        // Keep last good snapshot on transient errors.
      } finally {
        scheduleNext(HOST_RESOURCES_POLL_MS);
      }
    };

    // Only delay the first probe per session (after PTY welcome); tab switches resume immediately.
    scheduleNext(
      needsInitialDelayRef.current ? HOST_RESOURCES_INITIAL_DELAY_MS : 0,
    );

    return () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [isActive, sessionID, termReady, pollStopped]);

  const metricsLabel = formatMetricsLabel(resources, pollStopped);

  return (
    <Box
      sx={{
        height: height,
        px: 2,
        fontSize: 8,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        bgcolor: "background.paper",
        borderTop: 1,
        borderColor: "divider",
        gap: 2,
      }}
    >
      <Tooltip title="主机资源">
        <Typography
          component="span"
          onClick={toggleResourcesOpen}
          sx={{
            fontSize: 11,
            color: "text.secondary",
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
            cursor: "pointer",
            userSelect: "none",
            "&:hover": {
              color: "primary.main",
            },
          }}
        >
          {metricsLabel}
        </Typography>
      </Tooltip>

      <Stack direction="row" spacing={2} sx={{ flexShrink: 0 }}>
        {logging && (
          <Tooltip title="会话日志录制中">
            <FiberManualRecordIcon
              fontSize="small"
              sx={{ color: "error.main" }}
            />
          </Tooltip>
        )}
        <Tooltip title="隧道">
          <AltRouteIcon
            fontSize="small"
            onClick={toggleSshTunnelOpen}
            sx={{
              color: "text.secondary",
              "&:hover": {
                color: "primary.main",
                cursor: "pointer",
              },
            }}
          />
        </Tooltip>
        <Tooltip title="传输列表">
          <MobiledataOffIcon
            fontSize="small"
            onClick={toggleOpen}
            sx={{
              color: "text.secondary",
              "&:hover": {
                color: "primary.main",
                cursor: "pointer",
              },
            }}
          />
        </Tooltip>
      </Stack>

      <HostResourcesPanel
        open={resourcesOpen}
        statusBarHeight={height}
        sessionID={sessionID}
        resources={resources}
        pollStopped={pollStopped}
        onClose={handleResourcesClose}
      />

      <SSHTunnel
        sessionID={sessionID}
        open={sshTunnelOpen}
        statusBarHeight={height}
        onClose={handleSshTunnelClose}
      />

      <TransferList
        sessionID={sessionID}
        open={open}
        statusBarHeight={height}
        onClose={handleClose}
        sftpReady={sftpReady}
      />
    </Box>
  );
};

export default StatusBar;
