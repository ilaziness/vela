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
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

export default function BaseConvertTool() {
  const [input, setInput] = useState("");
  const [fromBase, setFromBase] = useState(10);
  const [toBase, setToBase] = useState(16);
  const [result, setResult] = useState<BaseConvertResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage(t("tools.base_convert.input_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertBase(input.trim(), fromBase, toBase);
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
        {t("tools.base_convert.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.base_convert.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.base_convert.input")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
            <TextField
              label={t("tools.base_convert.from_base")}
              type="number"
              value={fromBase}
              onChange={(e) => setFromBase(Number(e.target.value) || 10)}
              slotProps={{ htmlInput: { min: 2, max: 36 } }}
              fullWidth
            />
            <TextField
              label={t("tools.base_convert.to_base")}
              type="number"
              value={toBase}
              onChange={(e) => setToBase(Number(e.target.value) || 16)}
              slotProps={{ htmlInput: { min: 2, max: 36 } }}
              fullWidth
            />
          </Stack>
          <Stack direction="row" spacing={2}>
            <Button variant="outlined" onClick={handleSwap} fullWidth>
              {t("tools.base_convert.swap")}
            </Button>
            <Button variant="contained" onClick={handleConvert} disabled={isLoading} fullWidth>
              {isLoading
                ? t("tools.base_convert.converting")
                : t("tools.base_convert.convert")}
            </Button>
          </Stack>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <>
              <TextField
                label={t("tools.base_convert.result")}
                value={result.result}
                fullWidth
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" onClick={handleCopy} sx={{ mt: 2 }}>
                {t("tools.base_convert.copy_result")}
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
