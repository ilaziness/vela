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
import { parseCallServiceError } from "../func/service";

export default function PortCheckTool() {
  const [host, setHost] = useState("");
  const [port, setPort] = useState("");
  const [result, setResult] = useState<PortCheckResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleCheck = async () => {
    const portNum = Number.parseInt(port, 10);
    if (!host.trim()) {
      errorMessage("请输入主机地址");
      return;
    }
    if (Number.isNaN(portNum) || portNum < 1 || portNum > 65535) {
      errorMessage("端口号必须在 1-65535 之间");
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CheckPort(host.trim(), portNum);
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
        端口连通性检测
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        检测目标主机的 TCP 端口是否开放
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="主机地址"
            placeholder="例如: 192.168.1.1 或 example.com"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            fullWidth
          />
          <TextField
            label="端口号"
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
            {isLoading ? "检测中..." : "开始检测"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            检测结果
          </Typography>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              端口连接成功
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              端口连接失败: {result.error}
            </Alert>
          )}
          <Stack direction="row" spacing={2} sx={{ flexWrap: "wrap" }}>
            <Chip label={`主机: ${result.host}`} variant="outlined" />
            <Chip label={`端口: ${result.port}`} variant="outlined" />
            {result.success && (
              <Chip
                label={`响应时间: ${result.responseTime}ms`}
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
