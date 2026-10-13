import React, { useEffect, useState } from "react";
import {
  Box,
  Typography,
  Paper,
  TextField,
  Button,
  Stack,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Switch,
  FormControlLabel,
  Slider,
  Divider,
  Alert,
  CircularProgress,
} from "@mui/material";
import { Provider } from "../../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { useAIConfigStore } from "../../stores/aiConfig";
import { useMessageStore } from "../../stores/message";
import { parseCallServiceError } from "../../func/service";
import FormRow from "../FormRow";

const AISettings: React.FC = () => {
  const {
    config,
    isLoading,
    error,
    providers,
    isLoadingProviders,
    loadConfig,
    saveConfig,
    resetConfig,
    loadProviders,
    updatePartialConfig,
    clearError,
  } = useAIConfigStore();
  const { successMessage, errorMessage } = useMessageStore();

  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);

  useEffect(() => {
    loadConfig();
    loadProviders();
  }, [loadConfig, loadProviders]);

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await saveConfig(config);
      successMessage("AI 配置保存成功");
    } catch (err) {
      errorMessage(parseCallServiceError(err) || "AI 配置保存失败");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    setResetting(true);
    try {
      await resetConfig();
      successMessage("已重置为默认配置");
    } catch (err) {
      errorMessage(parseCallServiceError(err) || "重置配置失败");
    } finally {
      setResetting(false);
    }
  };

  const currentProvider = providers.find((p) => p.name === config?.provider);

  if (isLoading || !config) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        AI 助手设置
      </Typography>

      <Paper sx={{ p: 3 }} elevation={1}>
        {error && (
          <Alert severity="error" sx={{ mb: 2 }} onClose={clearError}>
            {error}
          </Alert>
        )}

        <Stack spacing={2}>
          <FormRow label="启用 AI 助手">
            <FormControlLabel
              control={
                <Switch
                  checked={config.enabled}
                  onChange={(e) =>
                    updatePartialConfig({ enabled: e.target.checked })
                  }
                />
              }
              label={config.enabled ? "已启用" : "已禁用"}
            />
          </FormRow>

          <Divider sx={{ my: 1 }} />

          <FormRow label="提供商">
            <FormControl fullWidth size="small">
              <InputLabel>选择提供商</InputLabel>
              <Select
                value={config.provider}
                label="选择提供商"
                onChange={(e) =>
                  updatePartialConfig({ provider: e.target.value as Provider })
                }
                disabled={isLoadingProviders}
              >
                {providers.map((provider) => (
                  <MenuItem key={provider.name} value={provider.name}>
                    {provider.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </FormRow>

          <FormRow label="模型">
            <TextField
              fullWidth
              size="small"
              value={config.model}
              onChange={(e) => updatePartialConfig({ model: e.target.value })}
              placeholder="例如: llama3.2, gpt-4"
            />
          </FormRow>

          {currentProvider?.needs_endpoint && (
            <FormRow label="API 端点">
              <TextField
                fullWidth
                size="small"
                value={config.endpoint}
                onChange={(e) =>
                  updatePartialConfig({ endpoint: e.target.value })
                }
                placeholder="例如: http://localhost:11434"
              />
            </FormRow>
          )}

          {currentProvider?.needs_api_key && (
            <FormRow label="API Key">
              <TextField
                fullWidth
                size="small"
                type="password"
                value={config.api_key}
                onChange={(e) =>
                  updatePartialConfig({ api_key: e.target.value })
                }
                placeholder="输入您的 API Key"
              />
            </FormRow>
          )}

          <FormRow label="温度 (Temperature)">
            <Box sx={{ width: "100%", px: 1 }}>
              <Slider
                value={config.temperature}
                onChange={(_, value) =>
                  updatePartialConfig({ temperature: value as number })
                }
                min={0}
                max={2}
                step={0.1}
                marks={[
                  { value: 0, label: "0" },
                  { value: 1, label: "1" },
                  { value: 2, label: "2" },
                ]}
                valueLabelDisplay="auto"
              />
            </Box>
          </FormRow>

          <FormRow label="最大 Token 数">
            <TextField
              fullWidth
              size="small"
              type="number"
              value={config.max_tokens}
              onChange={(e) => {
                const value = parseInt(e.target.value, 10);
                updatePartialConfig({
                  max_tokens: isNaN(value) ? 0 : value,
                });
              }}
              slotProps={{ htmlInput: { min: 100, max: 8192 } }}
              helperText="限制单次回复的最大生成长度；数值越大可输出越长，但响应更慢、占用更多资源。"
            />
          </FormRow>

          <Divider />

          <FormRow label="命令回显到终端">
            <FormControlLabel
              control={
                <Switch
                  checked={Boolean(config.echo_ssh_commands)}
                  onChange={(e) =>
                    updatePartialConfig({ echo_ssh_commands: e.target.checked })
                  }
                />
              }
              label="将 AI 批准执行的命令注解显示到当前 SSH 终端（默认关闭；不会重复执行）"
            />
          </FormRow>

          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 2, mt: 3 }}>
            <Button variant="outlined" onClick={handleReset} loading={resetting}>
              重置为默认
            </Button>
            <Button
              variant="contained"
              onClick={handleSave}
              loading={saving}
              disabled={!config}
            >
              保存
            </Button>
          </Box>
        </Stack>
      </Paper>
    </Box>
  );
};

export default AISettings;
