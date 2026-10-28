import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Paper,
  TextField,
  Button,
  Stack,
  FormControlLabel,
  Switch,
} from "@mui/material";
import FormRow from "../FormRow";
import { useMessageStore } from "../../stores/message";
import {
  Config,
  ConfigService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { translateServiceError } from "../../func/service";
import { t, tf } from "../../i18n";

interface TerminalSettingsProps {
  config: Config["Terminal"];
  onSaved?: (config: Config["Terminal"]) => void;
}

const MIN_SCROLLBACK = 100;
const MAX_SCROLLBACK = 100000;
const MIN_FONT_SIZE = 1;
const MAX_FONT_SIZE = 72;

const TerminalSettings: React.FC<TerminalSettingsProps> = ({
  config,
  onSaved,
}) => {
  const [localConfig, setLocalConfig] = useState(config);
  const [saving, setSaving] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  useEffect(() => {
    setLocalConfig(config);
  }, [config]);

  const handleChange = (field: keyof Config["Terminal"], value: unknown) => {
    setLocalConfig((prev) => ({ ...prev, [field]: value }));
  };

  const validateConfig = (): string[] => {
    const errors: string[] = [];

    if (!localConfig.fontFamily || localConfig.fontFamily.trim() === "") {
      errors.push(t("settings.terminal.font_required"));
    }
    if (
      !localConfig.fontSize ||
      localConfig.fontSize < MIN_FONT_SIZE ||
      localConfig.fontSize > MAX_FONT_SIZE
    ) {
      errors.push(
        tf("settings.terminal.font_size_range", MIN_FONT_SIZE, MAX_FONT_SIZE),
      );
    }
    if (!localConfig.lineHeight || localConfig.lineHeight <= 0) {
      errors.push(t("settings.terminal.line_height_positive"));
    }
    const sb = localConfig.scrollback;
    if (typeof sb !== "number" || sb < MIN_SCROLLBACK || sb > MAX_SCROLLBACK) {
      errors.push(
        tf("settings.terminal.scrollback_range", MIN_SCROLLBACK, MAX_SCROLLBACK),
      );
    }

    return errors;
  };

  const handleSave = async () => {
    const errors = validateConfig();
    if (errors.length > 0) {
      errorMessage(errors.join("\n"));
      return;
    }

    setSaving(true);
    try {
      await ConfigService.SaveTerminalConfig(localConfig);
      onSaved?.(localConfig);
      successMessage(t("settings.terminal.save_success"));
    } catch (error) {
      console.error("Failed to save terminal config:", error);
      errorMessage(
        translateServiceError(error) || t("settings.terminal.save_failed"),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        {t("settings.terminal.title")}
      </Typography>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Stack spacing={1.5}>
          <FormRow label={t("settings.terminal.font")}>
            <TextField
              required
              fullWidth
              size="small"
              value={localConfig.fontFamily || ""}
              onChange={(e) => handleChange("fontFamily", e.target.value)}
              placeholder={t("settings.terminal.font_placeholder")}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.font_size")}>
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: {
                  min: MIN_FONT_SIZE,
                  max: MAX_FONT_SIZE,
                  step: 1,
                },
              }}
              value={localConfig.fontSize || ""}
              onChange={(e) => {
                const value =
                  e.target.value === "" ? 0 : Number.parseInt(e.target.value, 10);
                handleChange("fontSize", isNaN(value) ? 0 : value);
              }}
              placeholder={t("settings.general.font_size_placeholder")}
              helperText={`${MIN_FONT_SIZE}–${MAX_FONT_SIZE}`}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.line_height")}>
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: {
                  min: 0.1,
                  step: 0.1,
                },
              }}
              value={localConfig.lineHeight || ""}
              onChange={(e) => {
                const value =
                  e.target.value === ""
                    ? 0
                    : Number.parseFloat(e.target.value);
                handleChange("lineHeight", isNaN(value) ? 0 : value);
              }}
              placeholder={t("settings.terminal.line_height_placeholder")}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.scrollback")}>
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: {
                  min: MIN_SCROLLBACK,
                  max: MAX_SCROLLBACK,
                  step: 1,
                },
              }}
              value={localConfig.scrollback ?? ""}
              onChange={(e) => {
                const value =
                  e.target.value === "" ? 0 : Number.parseInt(e.target.value, 10);
                handleChange("scrollback", isNaN(value) ? 0 : value);
              }}
              placeholder={t("settings.terminal.scrollback_placeholder")}
              helperText={tf(
                "settings.terminal.scrollback_helper",
                MIN_SCROLLBACK,
                MAX_SCROLLBACK,
              )}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.bell")}>
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.bell}
                  onChange={(e) => handleChange("bell", e.target.checked)}
                />
              }
              label={t("settings.terminal.bell_label")}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.copy_on_select")}>
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.copyOnSelect}
                  onChange={(e) =>
                    handleChange("copyOnSelect", e.target.checked)
                  }
                />
              }
              label={t("settings.terminal.copy_on_select_label")}
            />
          </FormRow>
          <FormRow label={t("settings.terminal.right_click_paste")}>
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.rightClickPaste}
                  onChange={(e) =>
                    handleChange("rightClickPaste", e.target.checked)
                  }
                />
              }
              label={t("settings.terminal.right_click_paste_label")}
            />
          </FormRow>
        </Stack>
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 3 }}>
          <Button variant="contained" onClick={handleSave} loading={saving}>
            {t("common.save")}
          </Button>
        </Box>
      </Paper>
    </Box>
  );
};

export default TerminalSettings;
