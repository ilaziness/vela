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
import { JWTResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

export default function JWTTool() {
  const [token, setToken] = useState("");
  const [result, setResult] = useState<JWTResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  const handleDecode = async () => {
    if (!token.trim()) {
      errorMessage(t("tools.jwt.jwt_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.DecodeJWT(token.trim());
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
        {t("tools.jwt.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.jwt.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="JWT"
            placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
            value={token}
            onChange={(e) => setToken(e.target.value)}
            multiline
            rows={4}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <Button variant="contained" onClick={handleDecode} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.processing") : t("tools.shared.decode")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Stack spacing={2}>
              <TextField
                label="Header"
                value={result.header ?? ""}
                fullWidth
                multiline
                rows={4}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" size="small" onClick={() => handleCopy(result.header)}>
                {t("tools.jwt.copy_header")}
              </Button>
              <TextField
                label="Payload"
                value={result.payload ?? ""}
                fullWidth
                multiline
                rows={6}
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
              />
              <Button variant="outlined" size="small" onClick={() => handleCopy(result.payload)}>
                {t("tools.jwt.copy_payload")}
              </Button>
              <TextField
                label={t("tools.jwt.signature")}
                value={result.signature ?? ""}
                fullWidth
                multiline
                slotProps={{ input: { readOnly: true, style: { fontFamily: "monospace" } } }}
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
