import { useState } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Paper,
  Stack,
  Alert,
  List,
  ListItem,
  ListItemText,
  Chip,
} from "@mui/material";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { CronResult } from "../types/tool";
import { useMessageStore } from "../stores/message";
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

export default function CronTool() {
  const [expr, setExpr] = useState("*/5 * * * *");
  const [count, setCount] = useState(5);
  const [result, setResult] = useState<CronResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleParse = async () => {
    if (!expr.trim()) {
      errorMessage(t("tools.shared.input_required"));
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ParseCron(expr.trim(), count);
      setResult(res);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        {t("tools.cron.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.cron.desc")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.cron.expression")}
            placeholder={t("tools.cron.expression_placeholder")}
            value={expr}
            onChange={(e) => setExpr(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <TextField
            label={t("tools.cron.next_count")}
            type="number"
            value={count}
            onChange={(e) => setCount(Number(e.target.value) || 5)}
            slotProps={{ htmlInput: { min: 1, max: 20 } }}
            fullWidth
          />
          <Button variant="contained" onClick={handleParse} disabled={isLoading} fullWidth>
            {isLoading ? t("tools.shared.parsing") : t("tools.shared.parse")}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <>
              <Stack direction="row" spacing={1} sx={{ mb: 2, alignItems: "center" }}>
                <Typography variant="subtitle1">{t("tools.cron.next_times")}</Typography>
                <Chip
                  label={tf("tools.cron.times_n", result.nextTimes?.length ?? 0)}
                  size="small"
                  color="success"
                />
              </Stack>
              <List dense>
                {(result.nextTimes ?? []).map((timeStr, index) => (
                  <ListItem key={`${index}-${timeStr}`} disablePadding>
                    <ListItemText
                      primary={timeStr}
                      slotProps={{ primary: { sx: { fontFamily: "monospace" } } }}
                    />
                  </ListItem>
                ))}
              </List>
            </>
          ) : (
            <Alert severity="error">{result.error}</Alert>
          )}
        </Paper>
      )}
    </Box>
  );
}
