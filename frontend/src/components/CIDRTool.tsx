import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Paper,
  Stack,
  Alert,
  Chip,
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { CIDRResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function CIDRTool() {
  const [cidr, setCidr] = useState("192.168.1.0/24");
  const [checkIP, setCheckIP] = useState("");
  const [result, setResult] = useState<CIDRResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleCalc = async () => {
    if (!cidr.trim()) {
      errorMessage(t("tools.cidr.cidr_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CalculateCIDR(cidr.trim(), checkIP.trim());
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
        {t("tools.cidr.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.cidr.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="CIDR"
            placeholder={t("tools.cidr.cidr_placeholder")}
            value={cidr}
            onChange={(e) => setCidr(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <TextField
            label={t("tools.cidr.check_ip")}
            placeholder={t("tools.cidr.check_ip_placeholder")}
            value={checkIP}
            onChange={(e) => setCheckIP(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleCalc} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.calculating") : t("tools.shared.calculate")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={1.5}>
              <Chip label={tf("tools.cidr.network", result.network)} variant="outlined" />
              <Chip
                label={tf("tools.cidr.netmask", result.netmask, result.prefixLen)}
                variant="outlined"
              />
              <Chip label={tf("tools.cidr.broadcast", result.broadcast)} variant="outlined" />
              <Chip label={tf("tools.cidr.first_host", result.firstHost)} variant="outlined" />
              <Chip label={tf("tools.cidr.last_host", result.lastHost)} variant="outlined" />
              <Chip label={tf("tools.cidr.host_count", result.hostCount)} variant="outlined" />
              <Chip label={tf("tools.cidr.total_addrs", result.totalAddrs)} variant="outlined" />
              {result.error && (
                <Alert severity="warning">{result.error}</Alert>
              )}
              {result.contains != null && result.containsIP && (
                <Alert severity={result.contains ? "success" : "warning"}>
                  {result.contains
                    ? tf("tools.cidr.contains_yes", result.containsIP)
                    : tf("tools.cidr.contains_no", result.containsIP)}
                </Alert>
              )}
            </Stack>
          ) : (
            <Alert severity="error">{result.error}</Alert>
          )}
        </Paper>
      )}
    </Box>
  );
}
