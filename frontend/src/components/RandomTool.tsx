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
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

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
        {t("tools.random.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.random.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <InputLabel>{t("tools.random.kind")}</InputLabel>
            <Select
              value={kind}
              label={t("tools.random.kind")}
              onChange={(e) => setKind(e.target.value as RandomKind)}
            >
              <MenuItem value={RandomKind.UuidV4}>UUID v4</MenuItem>
              <MenuItem value={RandomKind.UuidV7}>UUID v7</MenuItem>
              <MenuItem value={RandomKind.String}>{t("tools.random.kind_string")}</MenuItem>
            </Select>
          </FormControl>

          {kind === RandomKind.String && (
            <>
              <TextField
                label={t("tools.random.length")}
                type="number"
                value={length}
                onChange={(e) => setLength(Number(e.target.value) || 16)}
                slotProps={{ htmlInput: { min: 1, max: 1024 } }}
                fullWidth
              />
              <FormControl fullWidth>
                <InputLabel>{t("tools.random.charset")}</InputLabel>
                <Select
                  value={charset}
                  label={t("tools.random.charset")}
                  onChange={(e) => setCharset(e.target.value as RandomCharset)}
                >
                  <MenuItem value={RandomCharset.Alphanum}>
                    {t("tools.random.charset_alphanum")}
                  </MenuItem>
                  <MenuItem value={RandomCharset.Alpha}>
                    {t("tools.random.charset_alpha")}
                  </MenuItem>
                  <MenuItem value={RandomCharset.Numeric}>
                    {t("tools.random.charset_numeric")}
                  </MenuItem>
                  <MenuItem value={RandomCharset.Hex}>
                    {t("tools.random.charset_hex")}
                  </MenuItem>
                  <MenuItem value={RandomCharset.Base64}>
                    {t("tools.random.charset_base64")}
                  </MenuItem>
                  <MenuItem value={RandomCharset.Custom}>
                    {t("tools.random.charset_custom")}
                  </MenuItem>
                </Select>
              </FormControl>
              {charset === RandomCharset.Custom && (
                <TextField
                  label={t("tools.random.custom_charset")}
                  value={customCharset}
                  onChange={(e) => setCustomCharset(e.target.value)}
                  fullWidth
                />
              )}
            </>
          )}

          <Button variant="contained" onClick={handleGenerate} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.generating") : t("tools.shared.generate")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <>
              <TextField
                label={t("tools.shared.result")}
                value={result.result}
                fullWidth
                multiline
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" onClick={handleCopy} sx={{ mt: 2 }}>
                {t("tools.shared.copy_result")}
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
