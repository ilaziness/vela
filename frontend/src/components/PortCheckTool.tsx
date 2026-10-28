import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Alert,
  Paper,
  Stack,
  Chip,
} from "@mui/material";
import NetworkCheckIcon from "@mui/icons-material/NetworkCheck";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { PortCheckResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function PortCheckTool() {
  const [host, setHost] = useState("");
  const [port, setPort] = useState("");
  const [result, setResult] = useState<PortCheckResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleCheck = async () => {
    const portNum = Number.parseInt(port, 10);
    if (!host.trim()) {
      errorMessage(t("tools.port_check.host_required"));
      return;
    }
    if (Number.isNaN(portNum) || portNum < 1 || portNum > 65535) {
      errorMessage(t("tools.port_check.port_range"));
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CheckPort(host.trim(), portNum);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.port_check.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.port_check.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.port_check.host")}
            placeholder={t("tools.port_check.host_placeholder")}
            value={host}
            onChange={(e) => setHost(e.target.value)}
            fullWidth
          />
          <TextField
            label={t("tools.port_check.port")}
            placeholder="1-65535"
            type="number"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            fullWidth
            slotProps={{ htmlInput: { min: 1, max: 65535 } }}
          />
          <Button
            variant="contained"
            startIcon={<NetworkCheckIcon />}
            onClick={handleCheck}
            disabled={isLoading}
            fullWidth
          >
            {isLoading ? t("tools.shared.checking") : t("tools.shared.check")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            {t("tools.shared.result")}
          </Typography>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              {t("tools.port_check.success")}
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              {tf("tools.port_check.failed", result.error ?? "")}
            </Alert>
          )}
          <Stack direction="row" spacing={2} sx={{ flexWrap: "wrap" }}>
            <Chip label={tf("tools.port_check.chip_host", result.host)} variant="outlined" />
            <Chip label={tf("tools.port_check.chip_port", result.port)} variant="outlined" />
            {result.success && (
              <Chip
                label={tf("tools.port_check.chip_latency", result.responseTime)}
                color="success"
                variant="outlined"
              />
            )}
          </Stack>
        </Paper>
      )}
    </Box>
  );
}
