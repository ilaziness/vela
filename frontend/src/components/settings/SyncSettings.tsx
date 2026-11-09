import React, { useState, useEffect, useEffectEvent } from "react";
import {
  Box,
  Typography,
  Paper,
  Stack,
  TextField,
  Button,
  List,
  ListItem,
  ListItemText,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  CircularProgress,
  IconButton,
  MenuItem,
  Select,
  FormControl,
} from "@mui/material";
import { Refresh, Save, Delete as DeleteIcon } from "@mui/icons-material";
import FormRow from "../common/FormRow";
import { useMessageStore } from "../../stores/message";
import {
  ListBackups,
  DeleteBackup,
  HealthCheck,
} from "../../../bindings/github.com/ilaziness/vela/services/syncservice";
import { Config, LogService } from "../../../bindings/github.com/ilaziness/vela/services";
import { formatDateTime, t, tf } from "../../i18n";
import { formatFileSize, translateServiceError } from "../../func/service";
import { isSyncConfigured } from "../../func/sync";
import {
  BackupInfo,
  Provider,
} from "../../../bindings/github.com/ilaziness/vela/internal/sync/models";

type SyncConfig = Config["Sync"];

interface SyncSettingsProps {
  syncConfig: SyncConfig;
  onChange: (config: SyncConfig) => void | Promise<void>;
}

const SECRET_MASK = "••••••••";

function SecretKeyField({
  value,
  encrypted,
  placeholder,
  onChange,
}: {
  value?: string;
  encrypted?: string;
  placeholder: string;
  onChange: (next: string) => void;
}) {
  const hasSaved = Boolean(encrypted);
  const display = value || (hasSaved ? SECRET_MASK : "");
  return (
    <TextField
      fullWidth
      size="small"
      type="password"
      autoComplete="new-password"
      value={display}
      placeholder={hasSaved ? SECRET_MASK : placeholder}
      onFocus={(e) => {
        if (!value && hasSaved) {
          e.target.select();
        }
      }}
      onChange={(e) => {
        const next = e.target.value;
        if (next === SECRET_MASK) {
          return;
        }
        onChange(next);
      }}
    />
  );
}

function emptySyncConfig(
  provider: Provider = Provider.ProviderSelfHosted,
): SyncConfig {
  return {
    provider,
    selfhosted: {
      serverUrl: "",
      syncId: "",
      userKey: "",
      userKeyEncrypted: "",
    },
    webdav: {
      url: "",
      username: "",
      remotePath: "/Vela",
      password: "",
      passwordEncrypted: "",
    },
    s3: {
      endpoint: "",
      region: "",
      bucket: "",
      prefix: "",
      accessKey: "",
      secretKey: "",
      secretKeyEncrypted: "",
    },
  };
}

function normalizeLoadedConfig(cfg: SyncConfig | null | undefined): SyncConfig {
  const provider = cfg?.provider || Provider.ProviderSelfHosted;
  const base = emptySyncConfig(provider);
  if (!cfg) {
    return base;
  }
  return {
    provider,
    selfhosted: {
      serverUrl: cfg.selfhosted?.serverUrl || "",
      syncId: cfg.selfhosted?.syncId || "",
      userKey: "",
      userKeyEncrypted: cfg.selfhosted?.userKeyEncrypted || "",
    },
    webdav: {
      url: cfg.webdav?.url || "",
      username: cfg.webdav?.username || "",
      remotePath: cfg.webdav?.remotePath || "/Vela",
      password: "",
      passwordEncrypted: cfg.webdav?.passwordEncrypted || "",
    },
    s3: {
      endpoint: cfg.s3?.endpoint || "",
      region: cfg.s3?.region || "",
      bucket: cfg.s3?.bucket || "",
      prefix: cfg.s3?.prefix || "",
      accessKey: cfg.s3?.accessKey || "",
      secretKey: "",
      secretKeyEncrypted: cfg.s3?.secretKeyEncrypted || "",
    },
  };
}

const SyncSettings: React.FC<SyncSettingsProps> = ({
  syncConfig,
  onChange,
}) => {
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [healthStatus, setHealthStatus] = useState<string>("");
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [backupToDelete, setBackupToDelete] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [saving, setSaving] = useState(false);
  const [refreshingHistory, setRefreshingHistory] = useState(false);
  const { successMessage, errorMessage } = useMessageStore();

  const [localConfig, setLocalConfig] = useState<SyncConfig>(() =>
    normalizeLoadedConfig(syncConfig),
  );

  useEffect(() => {
    setLocalConfig(normalizeLoadedConfig(syncConfig));
  }, [syncConfig]);

  const isConfigured = isSyncConfigured(syncConfig);

  const validateLocalConfig = (): boolean => {
    if (!localConfig.provider) {
      errorMessage(t("settings.sync.provider_required"));
      return false;
    }
    switch (localConfig.provider) {
      case Provider.ProviderSelfHosted: {
        const sh = localConfig.selfhosted;
        if (!sh?.serverUrl?.trim()) {
          errorMessage(t("settings.sync.server_url_required"));
          return false;
        }
        if (!sh?.syncId?.trim()) {
          errorMessage(t("settings.sync.sync_id_required"));
          return false;
        }
        if (!sh?.userKey?.trim() && !sh?.userKeyEncrypted) {
          errorMessage(t("settings.sync.user_key_required"));
          return false;
        }
        return true;
      }
      case Provider.ProviderWebDAV: {
        const w = localConfig.webdav;
        if (!w?.url?.trim()) {
          errorMessage(t("settings.sync.webdav_url_required"));
          return false;
        }
        if (!w?.username?.trim()) {
          errorMessage(t("settings.sync.webdav_username_required"));
          return false;
        }
        if (!w?.remotePath?.trim()) {
          errorMessage(t("settings.sync.webdav_remote_path_required"));
          return false;
        }
        if (!w?.password?.trim() && !w?.passwordEncrypted) {
          errorMessage(t("settings.sync.webdav_password_required"));
          return false;
        }
        return true;
      }
      case Provider.ProviderS3: {
        const s = localConfig.s3;
        if (!s?.endpoint?.trim()) {
          errorMessage(t("settings.sync.s3_endpoint_required"));
          return false;
        }
        if (!s?.bucket?.trim()) {
          errorMessage(t("settings.sync.s3_bucket_required"));
          return false;
        }
        if (!s?.accessKey?.trim()) {
          errorMessage(t("settings.sync.s3_access_key_required"));
          return false;
        }
        if (!s?.secretKey?.trim() && !s?.secretKeyEncrypted) {
          errorMessage(t("settings.sync.s3_secret_key_required"));
          return false;
        }
        return true;
      }
      default:
        errorMessage(t("settings.sync.provider_required"));
        return false;
    }
  };

  const checkHealth = async () => {
    if (!isConfigured) return;
    setCheckingHealth(true);
    try {
      await HealthCheck();
      setHealthStatus("connected");
    } catch (error) {
      LogService.Error("Health check failed: " + String(error));
      setHealthStatus("disconnected");
    } finally {
      setCheckingHealth(false);
    }
  };

  const loadBackups = async () => {
    if (!isConfigured) return;
    setRefreshingHistory(true);
    try {
      const result = await ListBackups(10, 0);
      setBackups(result || []);
    } catch (error) {
      LogService.Error("Failed to load backups: " + String(error));
      errorMessage(
        t("settings.sync.load_history_failed", translateServiceError(error)),
      );
    } finally {
      setRefreshingHistory(false);
    }
  };

  const onConfigured = useEffectEvent(() => {
    void checkHealth();
    void loadBackups();
  });

  useEffect(() => {
    if (isConfigured) {
      onConfigured();
      return;
    }
    setHealthStatus("");
    setBackups([]);
  }, [isConfigured]);

  const handleSaveConfig = async () => {
    if (!validateLocalConfig()) return;
    setSaving(true);
    try {
      await onChange({ ...localConfig });
      // Health/history use saved config; parent updates syncConfig after save.
    } catch {
      // Parent already surfaces the error toast.
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!backupToDelete) return;
    setDeleting(true);
    try {
      await DeleteBackup(backupToDelete);
      successMessage(t("settings.sync.delete_backup_success"));
      await loadBackups();
    } catch (error) {
      LogService.Error("Delete backup failed: " + String(error));
      errorMessage(
        t("settings.sync.delete_backup_failed", translateServiceError(error)),
      );
    } finally {
      setDeleting(false);
      setDeleteDialogOpen(false);
      setBackupToDelete(null);
    }
  };

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false);
    setBackupToDelete(null);
  };

  const providerHintKey =
    localConfig.provider === Provider.ProviderWebDAV
      ? "settings.sync.provider_webdav_hint"
      : localConfig.provider === Provider.ProviderS3
        ? "settings.sync.provider_s3_hint"
        : "settings.sync.provider_selfhosted_hint";

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        {t("settings.sync.title")}
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }} elevation={1}>
        <Typography variant="h6" gutterBottom>
          {t("settings.sync.config_section")}
        </Typography>
        <Stack spacing={2}>
          <FormRow label={t("settings.sync.provider")}>
            <FormControl fullWidth size="small">
              <Select
                value={localConfig.provider || Provider.ProviderSelfHosted}
                onChange={(e) =>
                  setLocalConfig({
                    ...localConfig,
                    provider: e.target.value as Provider,
                  })
                }
              >
                <MenuItem value={Provider.ProviderSelfHosted}>
                  {t("settings.sync.provider_selfhosted")}
                </MenuItem>
                <MenuItem value={Provider.ProviderWebDAV}>
                  {t("settings.sync.provider_webdav")}
                </MenuItem>
                <MenuItem value={Provider.ProviderS3}>
                  {t("settings.sync.provider_s3")}
                </MenuItem>
              </Select>
            </FormControl>
          </FormRow>
          <Typography variant="body2" color="text.secondary">
            {t(providerHintKey)}
          </Typography>

          {localConfig.provider === Provider.ProviderSelfHosted && (
            <>
              <FormRow label={t("settings.sync.server_url")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.selfhosted?.serverUrl || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      selfhosted: {
                        ...localConfig.selfhosted,
                        serverUrl: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.server_url_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.sync_id")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.selfhosted?.syncId || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      selfhosted: {
                        ...localConfig.selfhosted,
                        syncId: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.sync_id_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.user_key")}>
                <SecretKeyField
                  value={localConfig.selfhosted?.userKey}
                  encrypted={localConfig.selfhosted?.userKeyEncrypted}
                  placeholder={t("settings.sync.user_key_placeholder")}
                  onChange={(userKey) =>
                    setLocalConfig({
                      ...localConfig,
                      selfhosted: {
                        ...localConfig.selfhosted,
                        userKey,
                      },
                    })
                  }
                />
              </FormRow>
            </>
          )}

          {localConfig.provider === Provider.ProviderWebDAV && (
            <>
              <FormRow label={t("settings.sync.webdav_url")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.webdav?.url || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      webdav: {
                        ...localConfig.webdav,
                        url: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.webdav_url_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.webdav_username")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.webdav?.username || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      webdav: {
                        ...localConfig.webdav,
                        username: e.target.value.trim(),
                      },
                    })
                  }
                />
              </FormRow>
              <FormRow label={t("settings.sync.webdav_password")}>
                <SecretKeyField
                  value={localConfig.webdav?.password}
                  encrypted={localConfig.webdav?.passwordEncrypted}
                  placeholder={t("settings.sync.webdav_password")}
                  onChange={(password) =>
                    setLocalConfig({
                      ...localConfig,
                      webdav: {
                        ...localConfig.webdav,
                        password,
                      },
                    })
                  }
                />
              </FormRow>
              <FormRow label={t("settings.sync.webdav_remote_path")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.webdav?.remotePath || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      webdav: {
                        ...localConfig.webdav,
                        remotePath: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t(
                    "settings.sync.webdav_remote_path_placeholder",
                  )}
                />
              </FormRow>
            </>
          )}

          {localConfig.provider === Provider.ProviderS3 && (
            <>
              <FormRow label={t("settings.sync.s3_endpoint")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.s3?.endpoint || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        endpoint: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.s3_endpoint_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.s3_region")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.s3?.region || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        region: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.s3_region_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.s3_bucket")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.s3?.bucket || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        bucket: e.target.value.trim(),
                      },
                    })
                  }
                />
              </FormRow>
              <FormRow label={t("settings.sync.s3_prefix")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.s3?.prefix || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        prefix: e.target.value.trim(),
                      },
                    })
                  }
                  placeholder={t("settings.sync.s3_prefix_placeholder")}
                />
              </FormRow>
              <FormRow label={t("settings.sync.s3_access_key")}>
                <TextField
                  fullWidth
                  size="small"
                  value={localConfig.s3?.accessKey || ""}
                  onChange={(e) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        accessKey: e.target.value.trim(),
                      },
                    })
                  }
                />
              </FormRow>
              <FormRow label={t("settings.sync.s3_secret_key")}>
                <SecretKeyField
                  value={localConfig.s3?.secretKey}
                  encrypted={localConfig.s3?.secretKeyEncrypted}
                  placeholder={t("settings.sync.s3_secret_key")}
                  onChange={(secretKey) =>
                    setLocalConfig({
                      ...localConfig,
                      s3: {
                        ...localConfig.s3,
                        secretKey,
                      },
                    })
                  }
                />
              </FormRow>
            </>
          )}

          <Box
            sx={{
              display: "flex",
              gap: 2,
              mt: 2,
              alignItems: "center",
              justifyContent: "flex-end",
            }}
          >
            <Button
              variant="contained"
              startIcon={<Save />}
              onClick={handleSaveConfig}
              loading={saving}
            >
              {t("common.save")}
            </Button>
            <Box sx={{ display: "flex", gap: 2, alignItems: "center" }}>
              {healthStatus && (
                <Chip
                  label={
                    healthStatus === "connected"
                      ? t("settings.sync.connected")
                      : t("settings.sync.disconnected")
                  }
                  color={healthStatus === "connected" ? "success" : "error"}
                  size="small"
                />
              )}
              <Button
                variant="outlined"
                size="small"
                onClick={() => void checkHealth()}
                disabled={checkingHealth || !isConfigured}
                startIcon={
                  checkingHealth ? <CircularProgress size={16} /> : <Refresh />
                }
              >
                {t("common.refresh")}
              </Button>
            </Box>
          </Box>
        </Stack>
      </Paper>

      {isConfigured && (
        <Paper sx={{ p: 3 }} elevation={1}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Typography variant="h6">{t("settings.sync.history")}</Typography>
            <IconButton
              size="small"
              onClick={() => void loadBackups()}
              disabled={refreshingHistory}
              title={t("settings.sync.refresh_backups")}
              aria-label={t("settings.sync.refresh_backups")}
            >
              {refreshingHistory ? (
                <CircularProgress size={18} />
              ) : (
                <Refresh />
              )}
            </IconButton>
          </Box>
          {backups.length === 0 ? (
            <Typography color="text.secondary">
              {t("settings.sync.no_backups")}
            </Typography>
          ) : (
            <List dense>
              {backups.map((backup) => (
                <ListItem
                  key={backup.name}
                  divider
                  secondaryAction={
                    <Button
                      size="small"
                      color="error"
                      startIcon={<DeleteIcon />}
                      onClick={() => {
                        setBackupToDelete(backup.name);
                        setDeleteDialogOpen(true);
                      }}
                    >
                      {t("common.delete")}
                    </Button>
                  }
                >
                  <ListItemText
                    primary={backup.name}
                    secondary={`${formatFileSize(backup.file_size)} • ${formatDateTime(
                      backup.created_at,
                    )}`}
                  />
                </ListItem>
              ))}
            </List>
          )}
        </Paper>
      )}

      <Dialog
        open={deleteDialogOpen}
        onClose={handleDeleteCancel}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("settings.sync.confirm_delete_title")}</DialogTitle>
        <DialogContent>
          <Typography>
            {tf("settings.sync.confirm_delete_body", backupToDelete ?? "")}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteCancel}>{t("common.cancel")}</Button>
          <Button
            variant="contained"
            color="error"
            onClick={() => void handleDeleteConfirm()}
            disabled={deleting}
          >
            {t("common.delete")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default SyncSettings;
