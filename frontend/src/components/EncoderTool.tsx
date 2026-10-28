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
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

export default function EncoderTool() {
  const [input, setInput] = useState("");
  const [output, setOutput] = useState("");
  const [encodingType, setEncodingType] = useState<EncodingType>(EncodingType.Base64);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { successMessage, errorMessage } = useMessageStore();

  const handleEncode = async () => {
    if (!input.trim()) {
      setError(t("tools.encoder.encode_required"));
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
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleDecode = async () => {
    if (!input.trim()) {
      setError(t("tools.encoder.decode_required"));
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
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!output) return;
    try {
      await navigator.clipboard.writeText(output);
      successMessage(t("common.copied"));
    } catch {
      errorMessage(t("common.copy_failed"));
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.encoder.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.encoder.desc")}
      </Typography>

      <Paper sx={{ p: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>{t("tools.encoder.encoding_type")}</InputLabel>
            <Select
              value={encodingType}
              label={t("tools.encoder.encoding_type")}
              onChange={(e) => setEncodingType(e.target.value as EncodingType)}
            >
              <MenuItem value={EncodingType.Base64}>Base64</MenuItem>
              <MenuItem value={EncodingType.URL}>URL</MenuItem>
              <MenuItem value={EncodingType.HTML}>HTML</MenuItem>
            </Select>
          </FormControl>

          <TextField
            label={t("tools.timestamp.input_label")}
            placeholder={t("tools.encoder.input_placeholder")}
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
              {t("tools.shared.encode")}
            </Button>
            <Button
              variant="outlined"
              startIcon={<CodeIcon />}
              onClick={handleDecode}
              disabled={isLoading}
            >
              {t("tools.shared.decode")}
            </Button>
          </Stack>

          {error && <Alert severity="error">{error}</Alert>}

          {output && (
            <Box>
              <Stack
                direction="row"
                sx={{ mb: 1, alignItems: "center", justifyContent: "space-between" }}
              >
                <Typography variant="subtitle2">{t("tools.shared.result")}</Typography>
                <Tooltip title={t("tools.shared.copy_result")}>
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
