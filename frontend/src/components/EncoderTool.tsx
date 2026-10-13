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
  IconButton,
  Tooltip,
} from "@mui/material";
import CodeIcon from "@mui/icons-material/Code";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { EncodingType } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";

export default function EncoderTool() {
  const [input, setInput] = useState("");
  const [output, setOutput] = useState("");
  const [encodingType, setEncodingType] = useState<EncodingType>(EncodingType.Base64);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { successMessage, errorMessage } = useMessageStore();

  const handleEncode = async () => {
    if (!input.trim()) {
      setError("请输入要编码的内容");
      return;
    }
    setError(null);
    setIsLoading(true);
    try {
      const res = await ToolService.Encode(encodingType, input);
      if (res.error) {
        setError(res.error);
        setOutput("");
      } else {
        setOutput(res.result);
      }
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleDecode = async () => {
    if (!input.trim()) {
      setError("请输入要解码的内容");
      return;
    }
    setError(null);
    setIsLoading(true);
    try {
      const res = await ToolService.Decode(encodingType, input);
      if (res.error) {
        setError(res.error);
        setOutput("");
      } else {
        setOutput(res.result);
      }
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!output) return;
    try {
      await navigator.clipboard.writeText(output);
      successMessage("已复制到剪贴板");
    } catch {
      errorMessage("复制失败");
    }
  };

  const encodingLabels: Record<EncodingType, string> = {
    [EncodingType.Base64]: "Base64",
    [EncodingType.URL]: "URL",
    [EncodingType.HTML]: "HTML",
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        编码解码工具
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        支持 Base64、URL、HTML 编码和解码
      </Typography>

      <Paper sx={{ p: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>编码类型</InputLabel>
            <Select
              value={encodingType}
              label="编码类型"
              onChange={(e) => setEncodingType(e.target.value as EncodingType)}
            >
              <MenuItem value={EncodingType.Base64}>Base64</MenuItem>
              <MenuItem value={EncodingType.URL}>URL</MenuItem>
              <MenuItem value={EncodingType.HTML}>HTML</MenuItem>
            </Select>
          </FormControl>

          <TextField
            label="输入内容"
            placeholder={`请输入要${encodingLabels[encodingType]}编码或解码的内容`}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            multiline
            rows={4}
            fullWidth
          />

          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<CodeIcon />}
              onClick={handleEncode}
              disabled={isLoading}
            >
              编码
            </Button>
            <Button
              variant="outlined"
              startIcon={<CodeIcon />}
              onClick={handleDecode}
              disabled={isLoading}
            >
              解码
            </Button>
          </Stack>

          {error && <Alert severity="error">{error}</Alert>}

          {output && (
            <Box>
              <Stack
                direction="row"
                sx={{ mb: 1, alignItems: "center", justifyContent: "space-between" }}
              >
                <Typography variant="subtitle2">结果</Typography>
                <Tooltip title="复制结果">
                  <IconButton onClick={handleCopy} size="small">
                    <ContentCopyIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </Stack>
              <TextField
                value={output}
                multiline
                rows={4}
                fullWidth
                slotProps={{ input: { readOnly: true } }}
              />
            </Box>
          )}
        </Stack>
      </Paper>
    </Box>
  );
}
