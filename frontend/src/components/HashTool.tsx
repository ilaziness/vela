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
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function HashTool() {
  const [input, setInput] = useState("");
  const [algorithm, setAlgorithm] = useState<HashAlgorithm>(HashAlgorithm.MD5);
  const [result, setResult] = useState<HashResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleCalculate = async () => {
    if (!input.trim()) {
      errorMessage(t("tools.shared.input_required"));
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.CalculateHash(input, algorithm);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!result?.result) return;
    try {
      await navigator.clipboard.writeText(result.result);
      successMessage(t("common.copied"));
    } catch {
      errorMessage(t("common.copy_failed"));
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.hash.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.hash.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.timestamp.input_label")}
            placeholder={t("tools.hash.input_placeholder")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            multiline
            rows={4}
            fullWidth
          />
          <FormControl fullWidth>
            <InputLabel>{t("tools.hash.algorithm")}</InputLabel>
            <Select
              value={algorithm}
              label={t("tools.hash.algorithm")}
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
            {isLoading ? t("tools.shared.calculating") : t("tools.shared.calculate")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            {t("tools.shared.result")}
          </Typography>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              {t("tools.hash.hash_success")}
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              {tf("tools.hash.hash_failed", result.error ?? "")}
            </Alert>
          )}
          <Stack spacing={2}>
            <Chip label={tf("tools.hash.chip_input", result.input)} variant="outlined" />
            <Chip label={tf("tools.hash.chip_algorithm", result.algorithm)} variant="outlined" />
            <TextField
              label={t("tools.hash.chip_result")}
              value={result.result}
              fullWidth
              multiline
              slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
            />
          </Stack>
          <Button
            variant="outlined"
            onClick={handleCopy}
            disabled={isLoading}
            sx={{ mt: 2 }}
          >
            {t("tools.shared.copy_result")}
          </Button>
        </Paper>
      )}
    </Box>
  );
}
