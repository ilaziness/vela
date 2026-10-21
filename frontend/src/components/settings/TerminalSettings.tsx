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
import { parseCallServiceError } from "../../func/service";

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
      errors.push("字体不能为空");
    }
    if (
      !localConfig.fontSize ||
      localConfig.fontSize < MIN_FONT_SIZE ||
      localConfig.fontSize > MAX_FONT_SIZE
    ) {
      errors.push(`字体大小须在 ${MIN_FONT_SIZE}–${MAX_FONT_SIZE} 之间`);
    }
    if (!localConfig.lineHeight || localConfig.lineHeight <= 0) {
      errors.push("行高必须大于0");
    }
    const sb = localConfig.scrollback;
    if (typeof sb !== "number" || sb < MIN_SCROLLBACK || sb > MAX_SCROLLBACK) {
      errors.push(`滚动缓冲行数须在 ${MIN_SCROLLBACK}–${MAX_SCROLLBACK} 之间`);
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
      successMessage("终端配置保存成功");
    } catch (error) {
      console.error("Failed to save terminal config:", error);
      errorMessage(parseCallServiceError(error) || "终端配置保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        终端设置
      </Typography>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Stack spacing={1.5}>
          <FormRow label="字体">
            <TextField
              required
              fullWidth
              size="small"
              value={localConfig.fontFamily || ""}
              onChange={(e) => handleChange("fontFamily", e.target.value)}
              placeholder="例如: Consolas, Monaco"
            />
          </FormRow>
          <FormRow label="字体大小">
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
              placeholder="例如: 14"
              helperText={`${MIN_FONT_SIZE}–${MAX_FONT_SIZE}`}
            />
          </FormRow>
          <FormRow label="行高">
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
              placeholder="例如: 1.2"
            />
          </FormRow>
          <FormRow label="滚动缓冲行数">
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
              placeholder="例如: 10000"
              helperText={`${MIN_SCROLLBACK}–${MAX_SCROLLBACK}，向上可回看的行数`}
            />
          </FormRow>
          <FormRow label="响铃">
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.bell}
                  onChange={(e) => handleChange("bell", e.target.checked)}
                />
              }
              label="服务器发出 BEL 时视觉提示"
            />
          </FormRow>
          <FormRow label="选中即复制">
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.copyOnSelect}
                  onChange={(e) =>
                    handleChange("copyOnSelect", e.target.checked)
                  }
                />
              }
              label="选中文字后立即复制到剪贴板"
            />
          </FormRow>
          <FormRow label="右键粘贴">
            <FormControlLabel
              control={
                <Switch
                  checked={!!localConfig.rightClickPaste}
                  onChange={(e) =>
                    handleChange("rightClickPaste", e.target.checked)
                  }
                />
              }
              label="右键直接粘贴（Shift+右键仍打开菜单）"
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

export default TerminalSettings;
