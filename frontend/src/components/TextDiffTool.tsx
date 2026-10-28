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
import { DiffLineType, DiffResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

export default function TextDiffTool() {
  const [left, setLeft] = useState("");
  const [right, setRight] = useState("");
  const [result, setResult] = useState<DiffResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleDiff = async () => {
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.DiffText(left, right);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  const lineBg = (type: string) => {
    switch (type) {
      case DiffLineType.Add:
        return "rgba(46, 125, 50, 0.15)";
      case DiffLineType.Remove:
        return "rgba(211, 47, 47, 0.15)";
      default:
        return "transparent";
    }
  };

  const linePrefix = (type: string) => {
    switch (type) {
      case DiffLineType.Add:
        return "+";
      case DiffLineType.Remove:
        return "-";
      default:
        return " ";
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.text_diff.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.text_diff.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <Stack direction={{ xs: "column", md: "row" }} spacing={2}>
            <TextField
              label={t("tools.text_diff.original")}
              value={left}
              onChange={(e) => setLeft(e.target.value)}
              multiline
              rows={12}
              fullWidth
              slotProps={{ input: { style: { fontFamily: "monospace" } } }}
            />
            <TextField
              label={t("tools.text_diff.revised")}
              value={right}
              onChange={(e) => setRight(e.target.value)}
              multiline
              rows={12}
              fullWidth
              slotProps={{ input: { style: { fontFamily: "monospace" } } }}
            />
          </Stack>
          <Button variant="contained" onClick={handleDiff} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.comparing") : t("tools.shared.compare")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <Box
              sx={{
                fontFamily: "monospace",
                fontSize: 13,
                whiteSpace: "pre-wrap",
                wordBreak: "break-all",
              }}
            >
              {(result.lines ?? []).length === 0 ? (
                <Alert severity="info">{t("tools.text_diff.both_empty")}</Alert>
              ) : (
                (result.lines ?? []).map((line, index) => (
                  <Box
                    key={`${line.type}-${line.oldLine ?? 0}-${line.newLine ?? 0}-${index}`}
                    sx={{
                      bgcolor: lineBg(line.type),
                      px: 1,
                      py: 0.25,
                      opacity: line.type === DiffLineType.Equal ? 0.85 : 1,
                    }}
                  >
                    {linePrefix(line.type)} {line.content}
                  </Box>
                ))
              )}
            </Box>
          ) : (
            <Alert severity="error">{result.error}</Alert>
          )}
        </Paper>
      )}
    </Box>
  );
}
