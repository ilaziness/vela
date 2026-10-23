import {
  Box,
  IconButton,
  Slide,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { Close } from "@mui/icons-material";
import { HostResources } from "../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { useHostMetricHistory } from "../hooks/useHostMetricHistory";
import CompactStat from "./hostResources/CompactStat";
import HostResourcesContent from "./hostResources/HostResourcesContent";

interface HostResourcesPanelProps {
  open: boolean;
  statusBarHeight: string;
  sessionID: string;
  resources: HostResources | null;
  pollStopped: boolean;
  onClose: () => void;
}

const HostResourcesPanel: React.FC<HostResourcesPanelProps> = ({
  open,
  statusBarHeight,
  sessionID,
  resources,
  pollStopped,
  onClose,
}) => {
  const history = useHostMetricHistory(sessionID, resources);

  const title = resources?.hostname
    ? `主机资源 · ${resources.hostname}`
    : "主机资源";

  let body: React.ReactNode;
  if (resources?.supported) {
    body = (
      <HostResourcesContent resources={resources} history={history} />
    );
  } else if (resources && !resources.supported) {
    body = (
      <>
        <Stack direction="row" spacing={3} sx={{ mb: 1.5 }}>
          <CompactStat
            label="延迟 (SSH)"
            value={`${resources.latency_ms ?? 0} ms`}
          />
        </Stack>
        <Typography variant="body2" color="text.secondary">
          当前远程系统不支持资源采集（需 Linux /proc）
        </Typography>
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

          <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>{body}</Box>
        </Box>
      </Slide>
    </Box>
  );
};

export default HostResourcesPanel;
