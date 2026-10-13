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
  Switch,
  FormGroup,
  FormControlLabel,
} from "@mui/material";
import CodeIcon from "@mui/icons-material/Code";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { TimestampResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function TimestampTool() {
  const [input, setInput] = useState("");
  const [toTimestamp, setToTimestamp] = useState(true);
  const [result, setResult] = useState<TimestampResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage("请输入要转换的内容");
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertTimestamp(input, toTimestamp);
      setResult(res);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    const textToCopy = toTimestamp ? result?.timestamp?.toString() : result?.datetime;
    if (!textToCopy) return;
    try {
      await navigator.clipboard.writeText(textToCopy);
      successMessage("已复制到剪贴板");
    } catch {
      errorMessage("复制失败");
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        时间戳转换工具
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        Unix时间戳和日期时间相互转换
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="输入内容"
            placeholder={toTimestamp ? "例如: 2026-04-01 15:04:05" : "例如: 1743507845"}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            type={toTimestamp ? "datetime-local" : "text"}
            fullWidth
            slotProps={toTimestamp ? { inputLabel: { shrink: true } } : undefined}
          />
          <FormGroup row>
            <FormControlLabel
              control={<Switch checked={toTimestamp} onChange={(e) => setToTimestamp(e.target.checked)} />}
              label="转换为时间戳"
            />
            <FormControlLabel
              control={<Switch checked={!toTimestamp} onChange={(e) => setToTimestamp(!e.target.checked)} />}
              label="转换为日期时间"
            />
          </FormGroup>
          <Button
            variant="contained"
            startIcon={<CodeIcon />}
            onClick={handleConvert}
            disabled={isLoading}
            fullWidth
          >
            {isLoading ? "转换中..." : "开始转换"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            转换结果
          </Typography>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              转换成功
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              转换失败: {result.error}
            </Alert>
          )}
          <Stack direction="row" spacing={2} sx={{ flexWrap: "wrap" }}>
            {toTimestamp && result.timestamp !== undefined && (
              <Chip label={`时间戳: ${result.timestamp}`} variant="outlined" />
            )}
            {!toTimestamp && result.datetime && (
              <Chip label={`日期时间: ${result.datetime}`} variant="outlined" />
            )}
          </Stack>
          <Button
            variant="outlined"
            onClick={handleCopy}
            disabled={isLoading}
            sx={{ mt: 2 }}
          >
            复制结果
          </Button>
        </Paper>
      )}
    </Box>
  );
}