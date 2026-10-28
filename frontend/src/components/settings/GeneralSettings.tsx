import React, { useState, useEffect } from "react";
import {
  Box,
  Stack,
  Typography,
  Paper,
  TextField,
  Button,
  IconButton,
  FormControlLabel,
  Switch,
  MenuItem,
  Select,
  FormControl,
} from "@mui/material";
import { MoreHoriz } from "@mui/icons-material";
import FormRow from "../FormRow";
import { useMessageStore } from "../../stores/message";
import {
  Config,
  ConfigService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { SelectDirectory } from "../../../bindings/github.com/ilaziness/vela/services/appservice";
import { SetLanguage } from "../../../bindings/github.com/ilaziness/vela/services/configservice";
import { translateServiceError } from "../../func/service";
import { sanitizeAppFont } from "../../theme";
import {
  AppLanguage,
  getLanguage,
  normalizeAppLanguage,
  t,
} from "../../i18n";

interface GeneralSettingsProps {
  config: Config["General"];
}

const LANGUAGE_OPTIONS: AppLanguage[] = [
  AppLanguage.En,
  AppLanguage.ZhCN,
  AppLanguage.ZhTW,
];

const GeneralSettings: React.FC<GeneralSettingsProps> = ({ config }) => {
  const [localConfig, setLocalConfig] = useState(config);
  const [saving, setSaving] = useState(false);
  const [langSaving, setLangSaving] = useState(false);
  const [language, setLanguage] = useState<AppLanguage>(() =>
    normalizeAppLanguage(config?.Language || getLanguage()),
  );
  const { errorMessage, successMessage } = useMessageStore();

  useEffect(() => {
    setLocalConfig(config);
    if (config?.Language) {
      setLanguage(normalizeAppLanguage(config.Language));
    }
  }, [config]);

  const handleChange = (field: keyof Config["General"], value: unknown) => {
    setLocalConfig((prev) => ({ ...prev, [field]: value }));
  };

  const handleSelectDirectory = async () => {
    try {
      const selectedDir = await SelectDirectory();
      if (selectedDir) {
        handleChange("UserDataDir", selectedDir);
      }
    } catch (error) {
      console.error("Failed to select directory:", error);
      errorMessage(t("settings.general.select_dir_failed"));
    }
  };

  const handleLanguageChange = async (next: AppLanguage) => {
    if (next === language) {
      return;
    }
    const prev = language;
    setLanguage(next);
    setLangSaving(true);
    try {
      await SetLanguage(next);
      successMessage(t("settings.general.language_saved"));
    } catch (error) {
      console.error("Failed to set language:", error);
      setLanguage(prev);
      errorMessage(
        translateServiceError(error) || t("settings.general.language_save_failed"),
      );
    } finally {
      setLangSaving(false);
    }
  };

  const handleSave = async () => {
    const userDataDir = localConfig.UserDataDir?.trim();
    if (!userDataDir) {
      errorMessage(t("settings.general.user_data_dir_required"));
      return;
    }

    const fontSize = localConfig.FontSize ?? 0;
    if (fontSize < 8 || fontSize > 72) {
      errorMessage(t("settings.general.font_size_range"));
      return;
    }

    setSaving(true);
    try {
      const font = sanitizeAppFont(localConfig.Font ?? "");
      await ConfigService.SaveGeneralConfig({
        ...localConfig,
        UserDataDir: userDataDir,
        Font: font,
        FontSize: fontSize,
      });
      setLocalConfig((prev) => ({ ...prev, Font: font, FontSize: fontSize }));
      successMessage(t("settings.general.save_success"));
    } catch (error) {
      console.error("Failed to save general config:", error);
      errorMessage(
        translateServiceError(error) || t("settings.general.save_failed"),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        {t("settings.general.title")}
      </Typography>
      <Paper sx={{ p: 3 }} elevation={1}>
        <Stack spacing={2}>
          <FormRow label={t("settings.language")}>
            <FormControl fullWidth size="small" disabled={langSaving}>
              <Select
                value={language}
                onChange={(e) =>
                  void handleLanguageChange(e.target.value as AppLanguage)
                }
              >
                {LANGUAGE_OPTIONS.map((opt) => (
                  <MenuItem key={opt} value={opt}>
                    {t(`settings.language.${opt}`)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </FormRow>
          <FormRow label={t("settings.general.user_data_dir")}>
            <Box sx={{ display: "flex", gap: 1 }}>
              <TextField
                fullWidth
                size="small"
                value={localConfig.UserDataDir || ""}
                onChange={(e) => handleChange("UserDataDir", e.target.value)}
                placeholder={t("settings.general.user_data_dir_placeholder")}
                slotProps={{
                  input: {
                    readOnly: true,
                    endAdornment: (
                      <IconButton size="small" onClick={handleSelectDirectory}>
                        <MoreHoriz />
                      </IconButton>
                    ),
                  },
                }}
              />
            </Box>
          </FormRow>
          <FormRow label={t("settings.general.frameless")}>
            <FormControlLabel
              control={
                <Switch
                  checked={Boolean(localConfig.Frameless)}
                  onChange={(e) => handleChange("Frameless", e.target.checked)}
                />
              }
              label={
                localConfig.Frameless
                  ? t("common.enabled")
                  : t("common.disabled")
              }
            />
          </FormRow>
          <FormRow label={t("settings.general.font")}>
            <TextField
              fullWidth
              size="small"
              value={localConfig.Font || ""}
              onChange={(e) => handleChange("Font", e.target.value)}
              placeholder={t("settings.general.font_placeholder")}
            />
          </FormRow>
          <FormRow label={t("settings.general.font_size")}>
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: {
                  min: 8,
                  max: 72,
                  step: 1,
                },
              }}
              value={localConfig.FontSize || ""}
              onChange={(e) => {
                const value =
                  e.target.value === ""
                    ? 0
                    : Number.parseInt(e.target.value, 10);
                handleChange("FontSize", Number.isNaN(value) ? 0 : value);
              }}
              placeholder={t("settings.general.font_size_placeholder")}
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

export default GeneralSettings;
