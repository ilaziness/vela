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
import { parseCallServiceError } from "../func/service";

export default function CronTool() {
  const [expr, setExpr] = useState("*/5 * * * *");
  const [count, setCount] = useState(5);
  const [result, setResult] = useState<CronResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { errorMessage } = useMessageStore();

  const handleParse = async () => {
    if (!expr.trim()) {
      errorMessage("请输入 Cron 表达式");
      return;
    }
    setIsLoading(true);
    setResult(null);
    try {
      const res = await ToolService.ParseCron(expr.trim(), count);
      setResult(res);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        Cron 表达式
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        支持 5 段或带秒的 6 段，以及 @hourly / @daily 等描述符
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="Cron 表达式"
            placeholder="例如: */5 * * * * 或 0 */1 * * * *"
            value={expr}
            onChange={(e) => setExpr(e.target.value)}
            fullWidth
            slotProps={{ input: { style: { fontFamily: "monospace" } } }}
          />
          <TextField
            label="接下来次数"
            type="number"
            value={count}
            onChange={(e) => setCount(Number(e.target.value) || 5)}
            slotProps={{ htmlInput: { min: 1, max: 20 } }}
            fullWidth
          />
          <Button variant="contained" onClick={handleParse} disabled={isLoading} fullWidth>
            {isLoading ? "解析中..." : "解析"}
          </Button>
        </Stack>
      </Paper>

      {result && (
        <Paper sx={{ p: 3 }}>
          {result.success ? (
            <>
              <Stack direction="row" spacing={1} sx={{ mb: 2, alignItems: "center" }}>
                <Typography variant="subtitle1">接下来执行时间</Typography>
                <Chip label={`${result.nextTimes?.length ?? 0} 次`} size="small" color="success" />
              </Stack>
              <List dense>
                {(result.nextTimes ?? []).map((t, index) => (
                  <ListItem key={`${index}-${t}`} disablePadding>
                    <ListItemText
                      primary={t}
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
