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
import { BaseConvertResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function BaseConvertTool() {
  const [input, setInput] = useState("");
  const [fromBase, setFromBase] = useState(10);
  const [toBase, setToBase] = useState(16);
  const [result, setResult] = useState<BaseConvertResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage("请输入数字");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertBase(input.trim(), fromBase, toBase);
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

  const handleSwap = () => {
    setFromBase(toBase);
    setToBase(fromBase);
    if (result?.success && result.result) {
      setInput(result.result);
      setResult(null);
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        进制转换
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        支持 2–36 进制，可处理较长数字
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="输入"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
            <TextField
              label="源进制"
              type="number"
              value={fromBase}
              onChange={(e) => setFromBase(Number(e.target.value) || 10)}
              slotProps={{ htmlInput: { min: 2, max: 36 } }}
              fullWidth
            />
            <TextField
              label="目标进制"
              type="number"
              value={toBase}
              onChange={(e) => setToBase(Number(e.target.value) || 16)}
              slotProps={{ htmlInput: { min: 2, max: 36 } }}
              fullWidth
            />
          </Stack>
          <Stack direction="row" spacing={2}>
            <Button variant="outlined" onClick={handleSwap} fullWidth>
              交换进制
            </Button>
            <Button variant="contained" onClick={handleConvert} disabled={isLoading} fullWidth>
              {isLoading ? "转换中..." : "转换"}
            </Button>
          </Stack>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <>
              <TextField
                label="结果"
                value={result.result}
                fullWidth
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" onClick={handleCopy} sx={{ mt: 2 }}>
                复制结果
              </Button>
            </>
          ) : (
            <Alert severity="error">{result.error}</Alert>
          )}
        </Paper>
      )}
    </Box>
  );
}
