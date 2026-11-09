import React, { useEffect, useState } from "react";
import {
  Alert,
  Backdrop,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  List,
  ListItemButton,
  ListItemText,
  Stack,
  Typography,
} from "@mui/material";
import CloudDownload from "@mui/icons-material/CloudDownload";
import CloudUpload from "@mui/icons-material/CloudUpload";
import { BackupInfo } from "../../../bindings/github.com/ilaziness/vela/internal/sync/models";
import {
  LogService,
  SyncProgress,
} from "../../../bindings/github.com/ilaziness/vela/services";
import {
  DownloadSync,
  GetSyncProgress,
  ListBackups,
  UploadSync,
} from "../../../bindings/github.com/ilaziness/vela/services/syncservice";
import { formatDateTime, t } from "../../i18n";
import { formatFileSize, translateServiceError } from "../../func/service";
import { useMessageStore } from "../../stores/message";

type Step =
  | "closed"
  | "choose"
  | "confirmUpload"
  | "restoreList"
  | "confirmOverwrite"
  | "busy"
  | "restart";

type BusyKind = "upload" | "download";

interface BackupRestoreDialogsProps {
  open: boolean;
  onClose: () => void;
}

const BackupRestoreDialogs: React.FC<BackupRestoreDialogsProps> = ({
  open,
  onClose,
}) => {
  const { successMessage, errorMessage } = useMessageStore();
  const [step, setStep] = useState<Step>("closed");
  const [busyKind, setBusyKind] = useState<BusyKind>("upload");
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  const [selectedBackup, setSelectedBackup] = useState<string | null>(null);
  const [restoreBackups, setRestoreBackups] = useState<BackupInfo[]>([]);
  const [restorePage, setRestorePage] = useState(0);
  const [restoreHasMore, setRestoreHasMore] = useState(true);
  const [restoreLoading, setRestoreLoading] = useState(false);

  useEffect(() => {
    if (!open) {
      setStep("closed");
      setBusyKind("upload");
      setProgress(null);
      setSelectedBackup(null);
      setRestoreBackups([]);
      setRestorePage(0);
      setRestoreHasMore(true);
      setRestoreLoading(false);
      return;
    }
    setStep("choose");
  }, [open]);

  useEffect(() => {
    if (step !== "busy") {
      setProgress(null);
      return;
    }

    const interval = setInterval(() => {
      void (async () => {
        try {
          const p = await GetSyncProgress();
          setProgress(p);
          if (p.isCompleted || p.error) {
            clearInterval(interval);
          }
        } catch (error) {
          LogService.Warn("Failed to get progress: " + String(error));
        }
      })();
    }, 500);

    return () => clearInterval(interval);
  }, [step]);

  const handleClose = () => {
    if (step === "busy") return;
    onClose();
  };

  const loadRestoreBackups = async (page: number) => {
    // Only block pagination overlap; page 0 must always be allowed to (re)load.
    if (page > 0 && restoreLoading) return;
    setRestoreLoading(true);
    try {
      const limit = 50;
      const result = (await ListBackups(limit, page * limit)) || [];
      if (page === 0) {
        setRestoreBackups(result);
      } else {
        setRestoreBackups((prev) => [...prev, ...result]);
      }
      setRestoreHasMore(result.length === limit);
      setRestorePage(page);
    } catch (error) {
      LogService.Error("Failed to load backups: " + String(error));
      errorMessage(
        t("settings.sync.load_backups_failed", translateServiceError(error)),
      );
    } finally {
      setRestoreLoading(false);
    }
  };

  const openRestoreList = () => {
    setSelectedBackup(null);
    setRestoreBackups([]);
    setRestorePage(0);
    setRestoreHasMore(true);
    setStep("restoreList");
    void loadRestoreBackups(0);
  };

  const handleRestoreScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = event.currentTarget;
    if (
      scrollHeight - scrollTop - clientHeight < 50 &&
      restoreHasMore &&
      !restoreLoading
    ) {
      void loadRestoreBackups(restorePage + 1);
    }
  };

  const runUpload = async () => {
    setBusyKind("upload");
    setStep("busy");
    setProgress(null);
    try {
      await UploadSync();
      successMessage(t("header.upload_success"));
      onClose();
    } catch (error) {
      LogService.Error("Upload failed: " + String(error));
      errorMessage(t("header.upload_failed", translateServiceError(error)));
      onClose();
    }
  };

  const runDownload = async () => {
    if (selectedBackup == null) {
      errorMessage(t("settings.sync.pick_backup"));
      return;
    }
    setBusyKind("download");
    setStep("busy");
    setProgress(null);
    try {
      await DownloadSync(selectedBackup);
      setStep("restart");
    } catch (error) {
      LogService.Error("Download failed: " + String(error));
      errorMessage(
        t("settings.sync.download_failed", translateServiceError(error)),
      );
      onClose();
    }
  };

  const busyMessage =
    busyKind === "upload"
      ? t("header.uploading_backup")
      : t("header.restoring_backup");

  return (
    <>
      <Dialog
        open={step === "choose"}
        onClose={handleClose}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("header.backup_restore")}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            <Button
              variant="contained"
              startIcon={<CloudUpload />}
              onClick={() => setStep("confirmUpload")}
              fullWidth
            >
              {t("header.backup")}
            </Button>
            <Button
              variant="outlined"
              startIcon={<CloudDownload />}
              onClick={openRestoreList}
              fullWidth
            >
              {t("header.restore")}
            </Button>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>{t("common.cancel")}</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={step === "confirmUpload"}
        onClose={() => setStep("choose")}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("header.confirm_upload_title")}</DialogTitle>
        <DialogContent>
          <Typography>{t("header.confirm_upload_message")}</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setStep("choose")}>{t("common.cancel")}</Button>
          <Button variant="contained" onClick={() => void runUpload()}>
            {t("common.ok")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={step === "restoreList"}
        onClose={() => setStep("choose")}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>{t("settings.sync.restore_dialog_title")}</DialogTitle>
        <DialogContent>
          <Typography gutterBottom>
            {t("settings.sync.restore_pick_backup")}
          </Typography>
          {restoreBackups.length === 0 && !restoreLoading ? (
            <Typography color="text.secondary">
              {t("settings.sync.no_backups")}
            </Typography>
          ) : (
            <Box
              sx={{ maxHeight: 400, overflow: "auto" }}
              onScroll={handleRestoreScroll}
            >
              <List>
                {restoreBackups.map((backup) => (
                  <ListItemButton
                    key={backup.name}
                    selected={selectedBackup === backup.name}
                    onClick={() => setSelectedBackup(backup.name)}
                  >
                    <ListItemText
                      primary={backup.name}
                      secondary={`${formatFileSize(backup.file_size)} • ${formatDateTime(
                        backup.created_at,
                      )}`}
                    />
                  </ListItemButton>
                ))}
              </List>
              {restoreLoading && (
                <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
                  <CircularProgress size={24} />
                </Box>
              )}
              {!restoreHasMore && restoreBackups.length > 0 && (
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ display: "block", textAlign: "center", py: 1 }}
                >
                  {t("settings.sync.all_backups_loaded")}
                </Typography>
              )}
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setStep("choose")}>{t("common.cancel")}</Button>
          <Button
            variant="contained"
            onClick={() => setStep("confirmOverwrite")}
            disabled={selectedBackup == null}
            startIcon={<CloudDownload />}
          >
            {t("settings.sync.restore")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={step === "confirmOverwrite"}
        onClose={() => setStep("restoreList")}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("header.confirm_restore_title")}</DialogTitle>
        <DialogContent>
          <Alert severity="warning" sx={{ mb: 1 }}>
            {t("header.confirm_restore_message")}
          </Alert>
          {selectedBackup != null && (
            <Typography variant="body2" color="text.secondary">
              {selectedBackup}
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setStep("restoreList")}>
            {t("common.cancel")}
          </Button>
          <Button
            variant="contained"
            color="warning"
            onClick={() => void runDownload()}
          >
            {t("settings.sync.restore")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={step === "restart"}
        onClose={onClose}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("settings.sync.restart_title")}</DialogTitle>
        <DialogContent>
          <Typography>{t("settings.sync.restart_body")}</Typography>
        </DialogContent>
        <DialogActions>
          <Button variant="contained" onClick={onClose}>
            {t("common.ok")}
          </Button>
        </DialogActions>
      </Dialog>

      <Backdrop
        open={step === "busy"}
        sx={(theme) => ({
          zIndex: theme.zIndex.modal + 1,
          backgroundColor: "rgba(0, 0, 0, 0.5)",
          flexDirection: "column",
          gap: 2,
          px: 3,
        })}
      >
        <Box
          sx={{
            bgcolor: "background.paper",
            borderRadius: 2,
            p: 3,
            minWidth: 320,
            maxWidth: 420,
          }}
        >
          <Typography variant="body1" sx={{ mb: 2, textAlign: "center" }}>
            {busyMessage}
          </Typography>
          {progress ? (
            <Box sx={{ mb: 2 }}>
              <Box sx={{ display: "flex", alignItems: "center", mb: 1 }}>
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {progress.stage}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {progress.progress.toFixed(1)}%
                </Typography>
              </Box>
              <LinearProgress
                variant="determinate"
                value={progress.progress}
                sx={{ mb: 1 }}
              />
              <Box sx={{ display: "flex", justifyContent: "space-between" }}>
                <Typography variant="caption" color="text.secondary">
                  {formatFileSize(progress.doneBytes)} /{" "}
                  {formatFileSize(progress.totalBytes)}
                </Typography>
                {progress.speed > 0 && (
                  <Typography variant="caption" color="text.secondary">
                    {formatFileSize(progress.speed)}/s
                  </Typography>
                )}
              </Box>
              {progress.error && (
                <Alert severity="error" sx={{ mt: 1 }}>
                  {progress.error}
                </Alert>
              )}
            </Box>
          ) : (
            <Box sx={{ display: "flex", justifyContent: "center" }}>
              <CircularProgress size={40} />
            </Box>
          )}
        </Box>
      </Backdrop>
    </>
  );
};

export default BackupRestoreDialogs;
