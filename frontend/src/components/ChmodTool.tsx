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
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function ChmodTool() {
  const [input, setInput] = useState("755");
  const [direction, setDirection] = useState<ChmodDirection>(ChmodDirection.ToSymbolic);
  const [result, setResult] = useState<ChmodResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage(t("tools.chmod.permission_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertChmod(input.trim(), direction);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async (text?: string) => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      successMessage(t("common.copied"));
    } catch {
      errorMessage(t("common.copy_failed"));
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.chmod.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.chmod.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>{t("tools.chmod.direction")}</InputLabel>
            <Select
              value={direction}
              label={t("tools.chmod.direction")}
              onChange={(e) => setDirection(e.target.value as ChmodDirection)}
            >
              <MenuItem value={ChmodDirection.ToSymbolic}>
                {t("tools.chmod.to_symbolic")}
              </MenuItem>
              <MenuItem value={ChmodDirection.ToOctal}>
                {t("tools.chmod.to_octal")}
              </MenuItem>
            </Select>
          </FormControl>
          <TextField
            label={t("tools.shared.input")}
            placeholder={
              direction === ChmodDirection.ToSymbolic
                ? t("tools.chmod.input_placeholder_octal")
                : t("tools.chmod.input_placeholder_symbolic")
            }
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleConvert} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.converting") : t("tools.shared.convert")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={2}>
              <Chip
                label={tf("tools.chmod.octal", result.octal)}
                variant="outlined"
                onClick={() => handleCopy(result.octal)}
                sx={{ fontFamily: "monospace", justifyContent: "flex-start" }}
              />
              <Chip
                label={tf("tools.chmod.symbolic", result.symbolic)}
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
