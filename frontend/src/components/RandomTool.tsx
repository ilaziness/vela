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
import { RandomCharset, RandomKind, RandomResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function RandomTool() {
  const [kind, setKind] = useState<RandomKind>(RandomKind.UuidV4);
  const [length, setLength] = useState(16);
  const [charset, setCharset] = useState<RandomCharset>(RandomCharset.Alphanum);
  const [customCharset, setCustomCharset] = useState("");
  const [result, setResult] = useState<RandomResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleGenerate = async () => {
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.GenerateRandom(
        kind,
        length,
        charset,
        customCharset,
      );
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
        随机生成
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        UUID v4 / v7，或指定长度与字符集的随机串
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>类型</InputLabel>
            <Select
              value={kind}
              label="类型"
              onChange={(e) => setKind(e.target.value as RandomKind)}
            >
              <MenuItem value={RandomKind.UuidV4}>UUID v4</MenuItem>
              <MenuItem value={RandomKind.UuidV7}>UUID v7</MenuItem>
              <MenuItem value={RandomKind.String}>随机串</MenuItem>
            </Select>
          </FormControl>

          {kind === RandomKind.String && (
            <>
              <TextField
                label="长度"
                type="number"
                value={length}
                onChange={(e) => setLength(Number(e.target.value) || 16)}
                slotProps={{ htmlInput: { min: 1, max: 1024 } }}
                fullWidth
              />
              <FormControl fullWidth>
                <InputLabel>字符集</InputLabel>
                <Select
                  value={charset}
                  label="字符集"
                  onChange={(e) => setCharset(e.target.value as RandomCharset)}
                >
                  <MenuItem value={RandomCharset.Alphanum}>字母+数字</MenuItem>
                  <MenuItem value={RandomCharset.Alpha}>仅字母</MenuItem>
                  <MenuItem value={RandomCharset.Numeric}>仅数字</MenuItem>
                  <MenuItem value={RandomCharset.Hex}>十六进制</MenuItem>
                  <MenuItem value={RandomCharset.Base64}>Base64 字符</MenuItem>
                  <MenuItem value={RandomCharset.Custom}>自定义</MenuItem>
                </Select>
              </FormControl>
              {charset === RandomCharset.Custom && (
                <TextField
                  label="自定义字符集"
                  value={customCharset}
                  onChange={(e) => setCustomCharset(e.target.value)}
                  fullWidth
                />
              )}
            </>
          )}

          <Button variant="contained" onClick={handleGenerate} disabled={isLoading} fullWidth>
            {isLoading ? "生成中..." : "生成"}
          </Button>
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
                multiline
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
