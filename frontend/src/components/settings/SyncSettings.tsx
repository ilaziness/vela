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
} from "@mui/material";
import { Refresh, Save, Delete as DeleteIcon } from "@mui/icons-material";
import FormRow from "../FormRow";
import { useMessageStore } from "../../stores/message";
import {
  ListSyncVersions,
  DeleteSyncVersion,
  HealthCheck,
} from "../../../bindings/github.com/ilaziness/vela/services/syncservice";
import { Config, LogService } from "../../../bindings/github.com/ilaziness/vela/services";
import { formatDateTime, t, tf } from "../../i18n";
import { formatFileSize, translateServiceError } from "../../func/service";

type SyncConfig = Config["Sync"];

interface SyncSettingsProps {
  syncConfig: SyncConfig;
  onChange: (config: SyncConfig) => void | Promise<void>;
}

interface VersionInfo {
  version_number: number;
  file_size: number;
  created_at: string;
}

const SyncSettings: React.FC<SyncSettingsProps> = ({
  syncConfig,
  onChange,
}) => {
  const [versions, setVersions] = useState<VersionInfo[]>([]);
  const [healthStatus, setHealthStatus] = useState<string>("");
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [versionToDelete, setVersionToDelete] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [saving, setSaving] = useState(false);
  const [refreshingHistory, setRefreshingHistory] = useState(false);
  const { successMessage, errorMessage } = useMessageStore();

  const [localConfig, setLocalConfig] = useState<SyncConfig>({
    serverUrl: syncConfig.serverUrl || "",
    syncId: syncConfig.syncId || "",
    userKey: syncConfig.userKey || "",
  });

  useEffect(() => {
    setLocalConfig({
      serverUrl: syncConfig.serverUrl || "",
      syncId: syncConfig.syncId || "",
      userKey: syncConfig.userKey || "",
    });
  }, [syncConfig]);

  // History/health against the server use saved config, not unsaved form draft.
  const isConfigured = Boolean(
    syncConfig.serverUrl && syncConfig.syncId && syncConfig.userKey,
  );

  const validateSyncConfig = (): boolean => {
    if (!localConfig.serverUrl.trim()) {
      errorMessage(t("settings.sync.server_url_required"));
      return false;
    }
    if (!localConfig.syncId.trim()) {
      errorMessage(t("settings.sync.sync_id_required"));
      return false;
    }
    if (!localConfig.userKey.trim()) {
      errorMessage(t("settings.sync.user_key_required"));
      return false;
    }
    return true;
  };

  const checkHealth = async () => {
    if (!validateSyncConfig()) return;

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

  const loadVersions = async () => {
    if (!isConfigured) return;
    setRefreshingHistory(true);
    try {
      const result = await ListSyncVersions(10, 0);
      setVersions(result || []);
    } catch (error) {
      LogService.Error("Failed to load versions: " + String(error));
      errorMessage(
        t("settings.sync.load_history_failed", translateServiceError(error)),
      );
    } finally {
      setRefreshingHistory(false);
    }
  };

  const onConfigured = useEffectEvent(() => {
    void checkHealth();
    void loadVersions();
  });

  useEffect(() => {
    if (isConfigured) {
      onConfigured();
      return;
    }
    setHealthStatus("");
    setVersions([]);
  }, [isConfigured]);

  const handleSaveConfig = async () => {
    setSaving(true);
    try {
      await onChange({ ...localConfig });
      if (
        localConfig.serverUrl.trim() &&
        localConfig.syncId.trim() &&
        localConfig.userKey.trim()
      ) {
        void checkHealth();
        void loadVersions();
      } else {
        setHealthStatus("");
        setVersions([]);
      }
    } catch {
      // Parent already surfaces the error toast.
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!versionToDelete) return;
    setDeleting(true);
    try {
      await DeleteSyncVersion(versionToDelete);
      successMessage(t("settings.sync.delete_version_success"));
      await loadVersions();
    } catch (error) {
      LogService.Error("Delete version failed: " + String(error));
      errorMessage(
        t("settings.sync.delete_version_failed", translateServiceError(error)),
      );
    } finally {
      setDeleting(false);
      setDeleteDialogOpen(false);
      setVersionToDelete(null);
    }
  };

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false);
    setVersionToDelete(null);
  };

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
          <FormRow label={t("settings.sync.server_url")}>
            <TextField
              fullWidth
              size="small"
              value={localConfig.serverUrl || ""}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  serverUrl: e.target.value.trim(),
                })
              }
              placeholder={t("settings.sync.server_url_placeholder")}
            />
          </FormRow>
          <FormRow label={t("settings.sync.sync_id")}>
            <TextField
              fullWidth
              size="small"
              value={localConfig.syncId || ""}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  syncId: e.target.value.trim(),
                })
              }
              placeholder={t("settings.sync.sync_id_placeholder")}
            />
          </FormRow>
          <FormRow label={t("settings.sync.user_key")}>
            <TextField
              fullWidth
              size="small"
              type="password"
              value={localConfig.userKey || ""}
              onChange={(e) =>
                setLocalConfig({
                  ...localConfig,
                  userKey: e.target.value.trim(),
                })
              }
              placeholder={t("settings.sync.user_key_placeholder")}
            />
          </FormRow>
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
                onClick={checkHealth}
                disabled={checkingHealth}
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
              onClick={() => void loadVersions()}
              disabled={refreshingHistory}
              title={t("settings.sync.refresh_versions")}
              aria-label={t("settings.sync.refresh_versions")}
            >
              {refreshingHistory ? (
                <CircularProgress size={18} />
              ) : (
                <Refresh />
              )}
            </IconButton>
          </Box>
          {versions.length === 0 ? (
            <Typography color="text.secondary">
              {t("settings.sync.no_versions")}
            </Typography>
          ) : (
            <List dense>
              {versions.map((version) => (
                <ListItem
                  key={version.version_number}
                  divider
                  secondaryAction={
                    <Button
                      size="small"
                      color="error"
                      startIcon={<DeleteIcon />}
                      onClick={() => {
                        setVersionToDelete(version.version_number);
                        setDeleteDialogOpen(true);
                      }}
                    >
                      {t("common.delete")}
                    </Button>
                  }
                >
                  <ListItemText
                    primary={tf(
                      "settings.sync.version_n",
                      version.version_number,
                    )}
                    secondary={`${formatFileSize(version.file_size)} • ${formatDateTime(
                      version.created_at,
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
            {tf("settings.sync.confirm_delete_body", versionToDelete ?? 0)}
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
