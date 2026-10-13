import { useState, useEffect, useCallback } from "react";
import {
  Box,
  TextField,
  Typography,
  Paper,
  Stack,
  Chip,
  Alert,
  FormGroup,
  FormControlLabel,
  Checkbox,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";
import { RegexMatchResult } from "../types/tool";

export default function RegexTool() {
  const [pattern, setPattern] = useState("");
  const [text, setText] = useState("");
  const [flags, setFlags] = useState({ i: false, m: false, g: true });
  const [result, setResult] = useState<RegexMatchResult | null>(null);

  const handleMatch = useCallback(async () => {
    if (!pattern.trim() || !text.trim()) {
      setResult(null);
      return;
    }

    try {
      const flagStr = [
        flags.i ? "i" : "",
        flags.m ? "m" : "",
        flags.g ? "g" : "",
      ].join("");
      const res = await ToolService.RegexMatch(pattern, text, flagStr);
      setResult(res);
    } catch {
      setResult({ matches: [], count: 0, error: "匹配失败" });
    }
  }, [pattern, text, flags]);

  useEffect(() => {
    const timeout = setTimeout(() => {
      handleMatch();
    }, 300);
    return () => clearTimeout(timeout);
  }, [handleMatch]);

  const handleFlagChange = (flag: keyof typeof flags) => {
    setFlags((prev) => ({ ...prev, [flag]: !prev[flag] }));
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom>
        正则表达式测试
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        实时测试正则表达式匹配结果
      </Typography>

      <Paper sx={{ p: 3 }}>
        <Stack spacing={3}>
          <TextField
            label="正则表达式"
            placeholder="输入正则表达式，例如: \d+"
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            fullWidth
          />

          <FormGroup row>
            <FormControlLabel
              control={
                <Checkbox
                  checked={flags.i}
                  onChange={() => handleFlagChange("i")}
                />
              }
              label="忽略大小写 (i)"
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={flags.m}
                  onChange={() => handleFlagChange("m")}
                />
              }
              label="多行模式 (m)"
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={flags.g}
                  onChange={() => handleFlagChange("g")}
                />
              }
              label="全局匹配 (g)"
            />
          </FormGroup>

          <TextField
            label="测试文本"
            placeholder="输入要测试的文本内容"
            value={text}
            onChange={(e) => setText(e.target.value)}
            multiline
            rows={6}
            fullWidth
          />

          {result?.error && <Alert severity="error">{result.error}</Alert>}

          {result && !result.error && (
            <Box>
              <Stack direction="row" spacing={1} sx={{ mb: 2, alignItems: "center" }}>
                <Typography variant="subtitle1">匹配结果</Typography>
                <Chip
                  label={`${result.count} 个匹配`}
                  color={result.count > 0 ? "success" : "default"}
                  size="small"
                />
              </Stack>

              {result.matches.length === 0 ? (
                <Alert severity="info">未找到匹配项</Alert>
              ) : (
                <Stack spacing={1}>
                  {result.matches.map((match, index) => (
                    <Accordion key={`${match.text}-${match.index}`} defaultExpanded={index === 0}>
                      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                        <Typography variant="body2">
                          匹配 #{index + 1}: "{match.text}" (位置: {match.index})
                        </Typography>
                      </AccordionSummary>
                      {match.groups.length > 0 && (
                        <AccordionDetails>
                          <Typography variant="subtitle2" gutterBottom>
                            捕获组:
                          </Typography>
                          <Stack spacing={0.5}>
                            {match.groups.map((group, groupIndex) => (
                              <Typography
                                key={`${group}-${groupIndex}`}
                                variant="body2"
                                color="text.secondary"
                              >
                                {`$${groupIndex + 1}: "${group}"`}
                              </Typography>
                            ))}
                          </Stack>
                        </AccordionDetails>
                      )}
                    </Accordion>
                  ))}
                </Stack>
              )}
            </Box>
          )}
        </Stack>
      </Paper>
    </Box>
  );
}
