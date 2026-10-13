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
import { parseCallServiceError } from "../func/service";

export default function CIDRTool() {
  const [cidr, setCidr] = useState("192.168.1.0/24");
  const [checkIP, setCheckIP] = useState("");
  const [result, setResult] = useState<CIDRResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleCalc = async () => {
    if (!cidr.trim()) {
      errorMessage("请输入 CIDR");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CalculateCIDR(cidr.trim(), checkIP.trim());
      setResult(res);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        子网计算
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        计算网络地址、掩码、主机范围，并可判断 IP 是否在网段内
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="CIDR"
            placeholder="例如: 10.0.0.0/8 或 2001:db8::/64"
            value={cidr}
            onChange={(e) => setCidr(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <TextField
            label="检查 IP（可选）"
            placeholder="例如: 192.168.1.100"
            value={checkIP}
            onChange={(e) => setCheckIP(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleCalc} disabled={isLoading} fullWidth>
            {isLoading ? "计算中..." : "计算"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={1.5}>
              <Chip label={`网络: ${result.network}`} variant="outlined" />
              <Chip label={`掩码: ${result.netmask} (/${result.prefixLen})`} variant="outlined" />
              <Chip label={`广播/末地址: ${result.broadcast}`} variant="outlined" />
              <Chip label={`首主机: ${result.firstHost}`} variant="outlined" />
              <Chip label={`末主机: ${result.lastHost}`} variant="outlined" />
              <Chip label={`可用主机数: ${result.hostCount}`} variant="outlined" />
              <Chip label={`总地址数: ${result.totalAddrs}`} variant="outlined" />
              {result.error && (
                <Alert severity="warning">{result.error}</Alert>
              )}
              {result.contains != null && (
                <Alert severity={result.contains ? "success" : "warning"}>
                  {result.containsIP} {result.contains ? "在" : "不在"} 该网段内
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
