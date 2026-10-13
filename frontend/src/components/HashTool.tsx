import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Alert,
  Paper,
  Stack,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { HashAlgorithm, HashResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function HashTool() {
  const [input, setInput] = useState("");
  const [algorithm, setAlgorithm] = useState<HashAlgorithm>(HashAlgorithm.MD5);
  const [result, setResult] = useState<HashResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleCalculate = async () => {
    if (!input.trim()) {
      errorMessage("请输入要计算哈希的内容");
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CalculateHash(input, algorithm);
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
        哈希计算工具
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        计算字符串的哈希值（支持 MD5、SHA1、SHA256、SHA512）
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="输入内容"
            placeholder="请输入要计算哈希的内容"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            multiline
            rows={4}
            fullWidth
          />
          <FormControl fullWidth>
            <InputLabel>哈希算法</InputLabel>
            <Select
              value={algorithm}
              label="哈希算法"
              onChange={(e) => setAlgorithm(e.target.value as HashAlgorithm)}
            >
              <MenuItem value={HashAlgorithm.MD5}>MD5</MenuItem>
              <MenuItem value={HashAlgorithm.SHA1}>SHA1</MenuItem>
              <MenuItem value={HashAlgorithm.SHA256}>SHA256</MenuItem>
              <MenuItem value={HashAlgorithm.SHA512}>SHA512</MenuItem>
            </Select>
          </FormControl>
          <Button
            variant="contained"
            onClick={handleCalculate}
            disabled={isLoading}
            fullWidth
          >
            {isLoading ? "计算中..." : "开始计算"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            计算结果
          </Typography>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              哈希计算成功
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              哈希计算失败: {result.error}
            </Alert>
          )}
          <Stack spacing={2}>
            <Chip label={`输入: ${result.input}`} variant="outlined" />
            <Chip label={`算法: ${result.algorithm}`} variant="outlined" />
            <TextField
              label="哈希结果"
              value={result.result}
              fullWidth
              multiline
              slotProps={{ input: { readOnly: true, style: { fontFamily: 'monospace' } } }}
            />
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