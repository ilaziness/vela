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
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

export default function JSONYAMLTool() {
  const [input, setInput] = useState("");
  const [action, setAction] = useState<JSONYAMLAction>(JSONYAMLAction.Format);
  const [result, setResult] = useState<FormatResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleRun = async () => {
    if (!input.trim()) {
      errorMessage(t("tools.shared.input_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.FormatJSONYAML(action, input);
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
        {t("tools.json_yaml.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.json_yaml.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>{t("tools.json_yaml.action")}</InputLabel>
            <Select
              value={action}
              label={t("tools.json_yaml.action")}
              onChange={(e) => setAction(e.target.value as JSONYAMLAction)}
            >
              <MenuItem value={JSONYAMLAction.Format}>
                {t("tools.json_yaml.format")}
              </MenuItem>
              <MenuItem value={JSONYAMLAction.Minify}>
                {t("tools.json_yaml.minify")}
              </MenuItem>
              <MenuItem value={JSONYAMLAction.Validate}>
                {t("tools.json_yaml.validate")}
              </MenuItem>
              <MenuItem value={JSONYAMLAction.Json2Yaml}>
                {t("tools.json_yaml.to_yaml")}
              </MenuItem>
              <MenuItem value={JSONYAMLAction.Yaml2Json}>
                {t("tools.json_yaml.to_json")}
              </MenuItem>
            </Select>
          </FormControl>
          <TextField
            label={t("tools.timestamp.input_label")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            multiline
            rows={10}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleRun} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.processing") : t("tools.shared.run")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Alert severity="success" sx={{ mb: 2 }}>
              {t("tools.shared.success")}
            </Alert>
          ) : (
            <Alert severity="error" sx={{ mb: 2 }}>
              {result.error}
            </Alert>
          )}
          {result.success && (
            <>
              <TextField
                label={t("tools.shared.result")}
                value={result.result}
                fullWidth
                multiline
                rows={10}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" onClick={handleCopy} sx={{ mt: 2 }}>
                {t("tools.shared.copy_result")}
              </Button>
            </>
          )}
        </Paper>
      )}
    </Box>
  );
}
