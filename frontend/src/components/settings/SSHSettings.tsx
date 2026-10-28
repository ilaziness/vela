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
import { translateServiceError } from "../../func/service";
import { ProxyType } from "../../types/proxy";
import { t } from "../../i18n";

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
      errors.push(t("settings.ssh.keepalive_negative"));
    }
    if (!localConfig.dialTimeoutSec || localConfig.dialTimeoutSec < 1) {
      errors.push(t("settings.ssh.dial_timeout_min"));
    }
    if (proxyEnabled) {
      if (!localConfig.proxyHost?.trim()) {
        errors.push(t("settings.ssh.proxy_host_required"));
      }
      const port = localConfig.proxyPort ?? 0;
      if (port < 1 || port > 65535) {
        errors.push(t("settings.ssh.proxy_port_range"));
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
      successMessage(t("settings.ssh.save_success"));
    } catch (error) {
      console.error("Failed to save SSH config:", error);
      errorMessage(
        translateServiceError(error) || t("settings.ssh.save_failed"),
      );
    } finally {
      setSaving(false);
    }
  };

  const renderProxyType = (selected: unknown) => {
    const v = String(selected ?? "");
    if (v === ProxyType.Http) return "HTTP";
    if (v === ProxyType.Socks5) return "SOCKS5";
    return t("settings.ssh.proxy_off");
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        {t("settings.ssh.title")}
      </Typography>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Stack spacing={1.5}>
          <FormRow label={t("settings.ssh.keepalive_sec")}>
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
              helperText={t("settings.ssh.keepalive_helper")}
            />
          </FormRow>
          <FormRow label={t("settings.ssh.dial_timeout_sec")}>
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
          <FormRow label={t("settings.ssh.auto_reconnect")}>
            <FormControlLabel
              control={
                <Switch
                  checked={Boolean(localConfig.autoReconnect)}
                  onChange={(e) =>
                    handleChange("autoReconnect", e.target.checked)
                  }
                />
              }
              label={
                localConfig.autoReconnect
                  ? t("common.enabled")
                  : t("common.disabled")
              }
            />
          </FormRow>
          <Typography variant="subtitle2" sx={{ pt: 1, fontWeight: 600 }}>
            {t("settings.ssh.dial_proxy")}
          </Typography>
          <FormRow label={t("settings.ssh.proxy_type")}>
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
                  renderValue: renderProxyType,
                },
              }}
              helperText={t("settings.ssh.proxy_helper")}
            >
              <MenuItem value={ProxyType.None}>
                {t("settings.ssh.proxy_off")}
              </MenuItem>
              <MenuItem value={ProxyType.Http}>HTTP</MenuItem>
              <MenuItem value={ProxyType.Socks5}>SOCKS5</MenuItem>
            </TextField>
          </FormRow>
          {proxyEnabled && (
            <>
              <FormRow label={t("settings.ssh.proxy_host")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.proxyHost ?? ""}
                  onChange={(e) => handleChange("proxyHost", e.target.value)}
                  placeholder={t("settings.ssh.proxy_host_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.ssh.proxy_port")}>
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
              <FormRow label={t("settings.ssh.proxy_username")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.proxyUser ?? ""}
                  onChange={(e) => handleChange("proxyUser", e.target.value)}
                  placeholder={t("common.optional")}
                />
              </FormRow>
              <FormRow label={t("settings.ssh.proxy_password")}>
                <TextField
                  fullWidth
                  size="small"
                  type="password"
                  value={localConfig.proxyPassword ?? ""}
                  onChange={(e) =>
                    handleChange("proxyPassword", e.target.value)
                  }
                  placeholder={t("common.optional")}
                />
              </FormRow>
            </>
          )}
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

export default SSHSettings;
