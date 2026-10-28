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
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function TimestampTool() {
  const [input, setInput] = useState("");
  const [toTimestamp, setToTimestamp] = useState(true);
  const [result, setResult] = useState<TimestampResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleConvert = async () => {
    if (!input.trim()) {
      errorMessage(t("tools.shared.input_required"));
      return;
    }

    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ConvertTimestamp(input, toTimestamp);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCopy = async () => {
    const textToCopy = toTimestamp ? result?.timestamp?.toString() : result?.datetime;
    if (!textToCopy) return;
    try {
      await navigator.clipboard.writeText(textToCopy);
      successMessage(t("common.copied"));
    } catch {
      errorMessage(t("common.copy_failed"));
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.timestamp.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.timestamp.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.timestamp.input_label")}
            placeholder={
              toTimestamp
                ? t("tools.timestamp.placeholder_to_ts")
                : t("tools.timestamp.placeholder_to_dt")
            }
            value={input}
            onChange={(e) => setInput(e.target.value)}
            type={toTimestamp ? "datetime-local" : "text"}
            fullWidth
            slotProps={toTimestamp ? { inputLabel: { shrink: true } } : undefined}
          />
          <FormGroup row>
            <FormControlLabel
              control={<Switch checked={toTimestamp} onChange={(e) => setToTimestamp(e.target.checked)} />}
              label={t("tools.timestamp.to_timestamp")}
            />
            <FormControlLabel
              control={<Switch checked={!toTimestamp} onChange={(e) => setToTimestamp(!e.target.checked)} />}
              label={t("tools.timestamp.to_datetime")}
            />
          </FormGroup>
          <Button
            variant="contained"
            startIcon={<CodeIcon />}
            onClick={handleConvert}
            disabled={isLoading}
            fullWidth
          >
            {isLoading ? t("tools.shared.converting") : t("tools.shared.convert")}
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
              {t("tools.shared.success")}
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              {tf("tools.shared.failed", result.error ?? "")}
            </Alert>
          )}
          <Stack direction="row" spacing={2} sx={{ flexWrap: "wrap" }}>
            {toTimestamp && result.timestamp !== undefined && (
              <Chip label={tf("tools.timestamp.result_ts", result.timestamp)} variant="outlined" />
            )}
            {!toTimestamp && result.datetime && (
              <Chip label={tf("tools.timestamp.result_dt", result.datetime)} variant="outlined" />
            )}
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
