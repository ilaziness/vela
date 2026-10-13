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
  MenuItem,
} from "@mui/material";
import FormRow from "../FormRow";
import { useMessageStore } from "../../stores/message";
import {
  Config,
  ConfigService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { parseCallServiceError } from "../../func/service";
import { ProxyType } from "../../types/proxy";

interface SSHSettingsProps {
  config: Config["SSH"];
}

const defaultSSHConfig: Config["SSH"] = {
  serverAliveInterval: 30,
  dialTimeoutSec: 30,
  autoReconnect: true,
  proxyType: ProxyType.None,
  proxyHost: "",
  proxyPort: 0,
  proxyUser: "",
  proxyPassword: "",
};

const SSHSettings: React.FC<SSHSettingsProps> = ({ config }) => {
  const [localConfig, setLocalConfig] = useState({
    ...defaultSSHConfig,
    ...config,
  });
  const [saving, setSaving] = useState(false);
  const { errorMessage, successMessage } = useMessageStore();

  useEffect(() => {
    setLocalConfig({ ...defaultSSHConfig, ...config });
  }, [config]);

  const handleChange = <K extends keyof Config["SSH"]>(
    field: K,
    value: Config["SSH"][K],
  ) => {
    setLocalConfig((prev) => ({ ...prev, [field]: value }));
  };

  const proxyEnabled = Boolean(localConfig.proxyType);

  const validateConfig = (): string[] => {
    const errors: string[] = [];
    if (localConfig.serverAliveInterval < 0) {
      errors.push("心跳间隔不能为负数（0 表示关闭）");
    }
    if (!localConfig.dialTimeoutSec || localConfig.dialTimeoutSec < 1) {
      errors.push("拨号超时必须大于等于 1 秒");
    }
    if (proxyEnabled) {
      if (!localConfig.proxyHost?.trim()) {
        errors.push("代理主机不能为空");
      }
      const port = localConfig.proxyPort ?? 0;
      if (port < 1 || port > 65535) {
        errors.push("代理端口必须在 1–65535 之间");
      }
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
      const toSave = { ...localConfig };
      if (!toSave.proxyType) {
        toSave.proxyHost = "";
        toSave.proxyPort = 0;
        toSave.proxyUser = "";
        toSave.proxyPassword = "";
      }
      await ConfigService.SaveSSHConfig(toSave);
      successMessage("连接配置保存成功");
    } catch (error) {
      console.error("Failed to save SSH config:", error);
      errorMessage(parseCallServiceError(error) || "连接配置保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        连接设置
      </Typography>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Stack spacing={1.5}>
          <FormRow label="心跳间隔（秒）">
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: { min: 0, step: 1 },
              }}
              value={localConfig.serverAliveInterval ?? 0}
              onChange={(e) => {
                const value =
                  e.target.value === ""
                    ? 0
                    : Number.parseInt(e.target.value, 10);
                handleChange(
                  "serverAliveInterval",
                  Number.isNaN(value) ? 0 : value,
                );
              }}
              helperText="对应 ServerAliveInterval，0 表示关闭保活"
            />
          </FormRow>
          <FormRow label="拨号超时（秒）">
            <TextField
              fullWidth
              size="small"
              type="number"
              slotProps={{
                htmlInput: { min: 1, step: 1 },
              }}
              value={localConfig.dialTimeoutSec ?? 30}
              onChange={(e) => {
                const value =
                  e.target.value === ""
                    ? 0
                    : Number.parseInt(e.target.value, 10);
                handleChange(
                  "dialTimeoutSec",
                  Number.isNaN(value) ? 0 : value,
                );
              }}
            />
          </FormRow>
          <FormRow label="自动重连">
            <FormControlLabel
              control={
                <Switch
                  checked={Boolean(localConfig.autoReconnect)}
                  onChange={(e) =>
                    handleChange("autoReconnect", e.target.checked)
                  }
                />
              }
              label={localConfig.autoReconnect ? "已开启" : "已关闭"}
            />
          </FormRow>
          <Typography variant="subtitle2" sx={{ pt: 1, fontWeight: 600 }}>
            拨号代理
          </Typography>
          <FormRow label="代理类型">
            <TextField
              select
              fullWidth
              size="small"
              value={localConfig.proxyType || ProxyType.None}
              onChange={(e) =>
                handleChange("proxyType", e.target.value as string)
              }
              slotProps={{
                select: {
                  displayEmpty: true,
                  renderValue: (selected) => {
                    const v = String(selected ?? "");
                    if (v === ProxyType.Http) return "HTTP";
                    if (v === ProxyType.Socks5) return "SOCKS5";
                    return "关闭";
                  },
                },
              }}
              helperText="仅用于 SSH 拨号，与系统代理无关"
            >
              <MenuItem value={ProxyType.None}>关闭</MenuItem>
              <MenuItem value={ProxyType.Http}>HTTP</MenuItem>
              <MenuItem value={ProxyType.Socks5}>SOCKS5</MenuItem>
            </TextField>
          </FormRow>
          {proxyEnabled && (
            <>
              <FormRow label="代理主机">
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.proxyHost ?? ""}
                  onChange={(e) => handleChange("proxyHost", e.target.value)}
                  placeholder="例如: 127.0.0.1 或 proxy.example.com"
                />
              </FormRow>
              <FormRow label="代理端口">
                <TextField
                  fullWidth
                  size="small"
                  type="number"
                  slotProps={{ htmlInput: { min: 1, max: 65535, step: 1 } }}
                  value={localConfig.proxyPort || ""}
                  onChange={(e) => {
                    const value =
                      e.target.value === ""
                        ? 0
                        : Number.parseInt(e.target.value, 10);
                    handleChange(
                      "proxyPort",
                      Number.isNaN(value) ? 0 : value,
                    );
                  }}
                />
              </FormRow>
              <FormRow label="用户名">
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.proxyUser ?? ""}
                  onChange={(e) => handleChange("proxyUser", e.target.value)}
                  placeholder="可选"
                />
              </FormRow>
              <FormRow label="密码">
                <TextField
                  fullWidth
                  size="small"
                  type="password"
                  value={localConfig.proxyPassword ?? ""}
                  onChange={(e) =>
                    handleChange("proxyPassword", e.target.value)
                  }
                  placeholder="可选"
                />
              </FormRow>
            </>
          )}
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

export default SSHSettings;
