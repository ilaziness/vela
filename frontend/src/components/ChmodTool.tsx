import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Paper,
  Stack,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Alert,
  Chip,
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { ChmodDirection, ChmodResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function ChmodTool() {
  const [input, setInput] = useState("755");
  const [direction, setDirection] = useState<ChmodDirection>(ChmodDirection.ToSymbolic);
  const [result, setResult] = useState<ChmodResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage("请输入权限");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertChmod(input.trim(), direction);
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
        权限计算
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        chmod 八进制与 rwx 符号互相转换
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>方向</InputLabel>
            <Select
              value={direction}
              label="方向"
              onChange={(e) => setDirection(e.target.value as ChmodDirection)}
            >
              <MenuItem value={ChmodDirection.ToSymbolic}>八进制 → 符号</MenuItem>
              <MenuItem value={ChmodDirection.ToOctal}>符号 → 八进制</MenuItem>
            </Select>
          </FormControl>
          <TextField
            label="输入"
            placeholder={
              direction === ChmodDirection.ToSymbolic ? "例如: 755 或 4755" : "例如: rwxr-xr-x"
            }
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleConvert} disabled={isLoading} fullWidth>
            {isLoading ? "转换中..." : "转换"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={2}>
              <Chip
                label={`八进制: ${result.octal}`}
                variant="outlined"
                onClick={() => handleCopy(result.octal)}
                sx={{ fontFamily: "monospace", justifyContent: "flex-start" }}
              />
              <Chip
                label={`符号: ${result.symbolic}`}
                variant="outlined"
                onClick={() => handleCopy(result.symbolic)}
                sx={{ fontFamily: "monospace", justifyContent: "flex-start" }}
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
