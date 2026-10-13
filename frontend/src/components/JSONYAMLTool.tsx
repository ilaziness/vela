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
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { FormatResult, JSONYAMLAction } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function JSONYAMLTool() {
  const [input, setInput] = useState("");
  const [action, setAction] = useState<JSONYAMLAction>(JSONYAMLAction.Format);
  const [result, setResult] = useState<FormatResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleRun = async () => {
    if (!input.trim()) {
      errorMessage("请输入内容");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.FormatJSONYAML(action, input);
      setResult(res);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!result?.result) return;
    try {
      await navigator.clipboard.writeText(result.result);
      successMessage("已复制到剪贴板");
    } catch {
      errorMessage("复制失败");
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        JSON / YAML
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        格式化、压缩、校验，以及 JSON 与 YAML 互转
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>操作</InputLabel>
            <Select
              value={action}
              label="操作"
              onChange={(e) => setAction(e.target.value as JSONYAMLAction)}
            >
              <MenuItem value={JSONYAMLAction.Format}>JSON 格式化</MenuItem>
              <MenuItem value={JSONYAMLAction.Minify}>JSON 压缩</MenuItem>
              <MenuItem value={JSONYAMLAction.Validate}>JSON 校验</MenuItem>
              <MenuItem value={JSONYAMLAction.Json2Yaml}>JSON → YAML</MenuItem>
              <MenuItem value={JSONYAMLAction.Yaml2Json}>YAML → JSON</MenuItem>
            </Select>
          </FormControl>
          <TextField
            label="输入内容"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            multiline
            rows={10}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleRun} disabled={isLoading} fullWidth>
            {isLoading ? "处理中..." : "执行"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              处理成功
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              {result.error}
            </Alert>
          )}
          {result.success && (
            <>
              <TextField
                label="结果"
                value={result.result}
                fullWidth
                multiline
                rows={10}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" onClick={handleCopy} sx={{ mt: 2 }}>
                复制结果
              </Button>
            </>
          )}
        </Paper>
      )}
    </Box>
  );
}
