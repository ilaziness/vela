import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Paper,
  Stack,
  Alert,
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { JWTResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function JWTTool() {
  const [token, setToken] = useState("");
  const [result, setResult] = useState<JWTResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleDecode = async () => {
    if (!token.trim()) {
      errorMessage("请输入 JWT");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.DecodeJWT(token.trim());
      setResult(res);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async (text?: string) => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      successMessage("已复制到剪贴板");
    } catch {
      errorMessage("复制失败");
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        JWT 解码
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        本地拆分 Header / Payload，不校验、不签发
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="JWT"
            placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
            value={token}
            onChange={(e) => setToken(e.target.value)}
            multiline
            rows={4}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleDecode} disabled={isLoading} fullWidth>
            {isLoading ? "解码中..." : "解码"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={2}>
              <TextField
                label="Header"
                value={result.header ?? ""}
                fullWidth
                multiline
                rows={4}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" size="small" onClick={() => handleCopy(result.header)}>
                复制 Header
              </Button>
              <TextField
                label="Payload"
                value={result.payload ?? ""}
                fullWidth
                multiline
                rows={6}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" size="small" onClick={() => handleCopy(result.payload)}>
                复制 Payload
              </Button>
              <TextField
                label="Signature（原文）"
                value={result.signature ?? ""}
                fullWidth
                multiline
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
            </Stack>
          ) : (
            <Alert severity="error">{result.error}</Alert>
          )}
        </Paper>
      )}
    </Box>
  );
}
