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
import { t, tf } from "../i18n";

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
      setResult({ matches: [], count: 0, error: t("tools.regex.match_failed") });
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
        {t("tools.regex.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("tools.regex.desc")}
      </Typography>

      <Paper sx={{ p: 3 }}>
        <Stack spacing={3}>
          <TextField
            label={t("tools.regex.pattern")}
            placeholder={t("tools.regex.pattern_placeholder")}
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
              label={t("tools.regex.flag_i")}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={flags.m}
                  onChange={() => handleFlagChange("m")}
                />
              }
              label={t("tools.regex.flag_m")}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={flags.g}
                  onChange={() => handleFlagChange("g")}
                />
              }
              label={t("tools.regex.flag_g")}
            />
          </FormGroup>

          <TextField
            label={t("tools.regex.test_text")}
            placeholder={t("tools.regex.test_placeholder")}
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
                <Typography variant="subtitle1">{t("tools.regex.results")}</Typography>
                <Chip
                  label={tf("tools.regex.match_count", result.count)}
                  color={result.count > 0 ? "success" : "default"}
                  size="small"
                />
              </Stack>

              {result.matches.length === 0 ? (
                <Alert severity="info">{t("tools.regex.no_matches")}</Alert>
              ) : (
                <Stack spacing={1}>
                  {result.matches.map((match, index) => (
                    <Accordion key={`${match.text}-${match.index}`} defaultExpanded={index === 0}>
                      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                        <Typography variant="body2">
                          {tf(
                            "tools.regex.match_n",
                            index + 1,
                            match.text,
                            match.index,
                          )}
                        </Typography>
                      </AccordionSummary>
                      {match.groups.length > 0 && (
                        <AccordionDetails>
                          <Typography variant="subtitle2" gutterBottom>
                            {t("tools.regex.capture_groups")}
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
