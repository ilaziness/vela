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
} from "@mui/material";
import { MoreHoriz } from "@mui/icons-material";
import FormRow from "../FormRow";
import { useMessageStore } from "../../stores/message";
import {
  Config,
  ConfigService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { SelectDirectory } from "../../../bindings/github.com/ilaziness/vela/services/appservice";
import { parseCallServiceError } from "../../func/service";
import { sanitizeAppFont } from "../../theme";

interface GeneralSettingsProps {
  config: Config["General"];
}

const GeneralSettings: React.FC<GeneralSettingsProps> = ({ config }) => {
  const [localConfig, setLocalConfig] = useState(config);
  const [saving, setSaving] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  // 当父组件传入新 config 时同步更新本地状态
  useEffect(() => {
    setLocalConfig(config);
  }, [config]);

  const handleChange = (field: keyof Config["General"], value: any) => {
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
      errorMessage("选择目录失败");
    }
  };

  const handleSave = async () => {
    const userDataDir = localConfig.UserDataDir?.trim();
    if (!userDataDir) {
      errorMessage("用户数据目录不能为空");
      return;
    }

    const fontSize = localConfig.FontSize ?? 0;
    if (fontSize < 8 || fontSize > 72) {
      errorMessage("字体大小必须在 8–72 之间");
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
      successMessage("通用配置保存成功");
    } catch (error) {
      console.error("Failed to save general config:", error);
      errorMessage(parseCallServiceError(error) || "通用配置保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        通用设置
      </Typography>
      <Paper sx={{ p: 3 }} elevation={1}>
        <Stack spacing={2}>
        <FormRow label="用户数据目录">
          <Box sx={{ display: "flex", gap: 1 }}>
            <TextField
              fullWidth
              size="small"
              value={localConfig.UserDataDir || ""}
              onChange={(e) => handleChange("UserDataDir", e.target.value)}
              placeholder="请选择用户数据目录路径"
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
        <FormRow label="无边框">
          <FormControlLabel
            control={
              <Switch
                checked={Boolean(localConfig.Frameless)}
                onChange={(e) => handleChange("Frameless", e.target.checked)}
              />
            }
            label={localConfig.Frameless ? "已开启" : "已关闭"}
          />
        </FormRow>
        <FormRow label="字体">
          <TextField
            fullWidth
            size="small"
            value={localConfig.Font || ""}
            onChange={(e) => handleChange("Font", e.target.value)}
            placeholder="留空使用系统字体"
          />
        </FormRow>
        <FormRow label="字体大小">
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
            placeholder="例如: 14"
          />
        </FormRow>
        </Stack>
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 3 }}>
          <Button variant="contained" onClick={handleSave} loading={saving}>
            保存
          </Button>
        </Box>
      </Paper>
    </Box>
  );
};

export default GeneralSettings;
