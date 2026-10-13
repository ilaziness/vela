import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Paper,
  TextField,
  Button,
  Stack,
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
}

const TerminalSettings: React.FC<TerminalSettingsProps> = ({ config }) => {
  const [localConfig, setLocalConfig] = useState(config);
  const [saving, setSaving] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  // 当父组件传入新 config 时同步更新本地状态
  useEffect(() => {
    setLocalConfig(config);
  }, [config]);

  const handleChange = (field: keyof Config["Terminal"], value: any) => {
    setLocalConfig((prev) => ({ ...prev, [field]: value }));
  };

  const validateConfig = (): string[] => {
    const errors: string[] = [];

    if (!localConfig.fontFamily || localConfig.fontFamily.trim() === "") {
      errors.push("字体不能为空");
    }
    if (!localConfig.fontSize || localConfig.fontSize < 1) {
      errors.push("字体大小必须大于等于1");
    }
    if (!localConfig.lineHeight || localConfig.lineHeight <= 0) {
      errors.push("行高必须大于0");
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
                  min: 1,
                  step: 1,
                },
              }}
              value={localConfig.fontSize || ""}
              onChange={(e) => {
                const value = e.target.value === "" ? 0 : Number.parseInt(e.target.value, 10);
                handleChange("fontSize", isNaN(value) ? 0 : value);
              }}
              placeholder="例如: 14"
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
                const value = e.target.value === "" ? 0 : Number.parseFloat(e.target.value);
                handleChange("lineHeight", isNaN(value) ? 0 : value);
              }}
              placeholder="例如: 1.2"
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
