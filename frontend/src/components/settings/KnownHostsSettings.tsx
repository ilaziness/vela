import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Paper,
  Button,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Tooltip,
} from "@mui/material";
import { Delete, Refresh } from "@mui/icons-material";
import { SSHService } from "../../../bindings/github.com/ilaziness/vela/services";
import type { KnownHostEntry } from "../../../bindings/github.com/ilaziness/vela/internal/ssh/models";
import { useMessageStore } from "../../stores/message";
import { t, tf } from "../../i18n";
import { translateServiceError } from "../../func/service";

const KnownHostsSettings: React.FC = () => {
  const { successMessage, errorMessage } = useMessageStore();
  const [entries, setEntries] = useState<KnownHostEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleteTarget, setDeleteTarget] = useState<KnownHostEntry | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadEntries = useCallback(async () => {
    try {
      const list = await SSHService.ListKnownHosts();
      setEntries(
        (list ?? []).filter((item): item is KnownHostEntry => item != null),
      );
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setLoading(false);
    }
  }, [errorMessage]);

  useEffect(() => {
    void loadEntries();
  }, [loadEntries]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await SSHService.DeleteKnownHost(
        deleteTarget.host,
        deleteTarget.keyType,
      );
      successMessage(t("settings.known_hosts.deleted"));
      setDeleteTarget(null);
      await loadEntries();
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          mb: 3,
        }}
      >
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          {t("settings.known_hosts.title")}
        </Typography>
        <Button
          startIcon={<Refresh />}
          onClick={() => {
            setLoading(true);
            void loadEntries();
          }}
          disabled={loading}
        >
          {t("common.refresh")}
        </Button>
      </Box>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {t("settings.known_hosts.hint")}
        </Typography>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t("settings.known_hosts.col_host")}</TableCell>
              <TableCell>{t("settings.known_hosts.col_type")}</TableCell>
              <TableCell>{t("settings.known_hosts.col_fingerprint")}</TableCell>
              <TableCell align="right" width={80}>
                {t("settings.known_hosts.col_actions")}
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} align="center">
                  {loading
                    ? t("common.loading")
                    : t("settings.known_hosts.empty")}
                </TableCell>
              </TableRow>
            )}
            {entries.map((entry) => (
              <TableRow key={`${entry.host}|${entry.keyType}`}>
                <TableCell>{entry.host}</TableCell>
                <TableCell>{entry.keyType}</TableCell>
                <TableCell sx={{ wordBreak: "break-all" }}>
                  {entry.fingerprint || "—"}
                </TableCell>
                <TableCell align="right">
                  <Tooltip title={t("common.delete")}>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => setDeleteTarget(entry)}
                    >
                      <Delete fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => !deleting && setDeleteTarget(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("settings.known_hosts.delete_title")}</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            {tf(
              "settings.known_hosts.delete_body",
              deleteTarget?.host ?? "",
              deleteTarget?.keyType ?? "",
            )}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} disabled={deleting}>
            {t("common.cancel")}
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => void handleDelete()}
            loading={deleting}
          >
            {t("common.delete")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default KnownHostsSettings;
