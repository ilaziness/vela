import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Paper,
  Button,
  TextField,
  MenuItem,
  Stack,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  FormControlLabel,
  Radio,
  RadioGroup,
  IconButton,
  Tooltip,
} from "@mui/material";
import { ContentCopy, Delete, FileDownload } from "@mui/icons-material";
import {
  KeyService,
  SSHKeyInfo,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { useMessageStore } from "../../stores/message";
import { formatDateTime, t, tf } from "../../i18n";
import { translateServiceError } from "../../func/service";

enum KeyAlgorithm {
  Ed25519 = "ed25519",
  EcdsaP256 = "ecdsa-p256",
  Rsa4096 = "rsa-4096",
}

enum KeySaveMode {
  Store = "store",
  Export = "export",
}

const algorithmOptions: { value: KeyAlgorithm; label: string }[] = [
  { value: KeyAlgorithm.Ed25519, label: "Ed25519" },
  { value: KeyAlgorithm.EcdsaP256, label: "ECDSA P-256" },
  { value: KeyAlgorithm.Rsa4096, label: "RSA 4096" },
];

interface PublicView {
  publicKey: string;
  fingerprint: string;
}

const KeySettings: React.FC = () => {
  const { successMessage, errorMessage } = useMessageStore();
  const [keys, setKeys] = useState<SSHKeyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState("");
  const [algorithm, setAlgorithm] = useState<KeyAlgorithm>(KeyAlgorithm.Ed25519);
  const [comment, setComment] = useState("");
  const [saveMode, setSaveMode] = useState<KeySaveMode>(KeySaveMode.Store);
  const [passphrase, setPassphrase] = useState("");
  const [generated, setGenerated] = useState<PublicView | null>(null);
  const [exportTarget, setExportTarget] = useState<SSHKeyInfo | null>(null);
  const [exportPassphrase, setExportPassphrase] = useState("");
  const [exporting, setExporting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SSHKeyInfo | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadKeys = useCallback(async () => {
    try {
      const list = await KeyService.List();
      setKeys((list ?? []).filter((item): item is SSHKeyInfo => item != null));
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setLoading(false);
    }
  }, [errorMessage]);

  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  const resetGenerate = () => {
    setName("");
    setAlgorithm(KeyAlgorithm.Ed25519);
    setComment("");
    setSaveMode(KeySaveMode.Store);
    setPassphrase("");
    setGenerated(null);
  };

  const closeGenerate = () => {
    setGenerateOpen(false);
    resetGenerate();
  };

  const copyPublicKey = async (publicKey: string) => {
    try {
      await navigator.clipboard.writeText(publicKey);
      successMessage(t("settings.keys.public_key_copied"));
    } catch (error) {
      errorMessage(t("settings.keys.copy_public_failed"));
      console.error(error);
    }
  };

  const handleGenerate = async () => {
    setSaving(true);
    try {
      if (saveMode === KeySaveMode.Store) {
        const info = await KeyService.GenerateAndSave(name.trim(), algorithm, comment.trim());
        setGenerated({ publicKey: info.public_key, fingerprint: info.fingerprint });
        successMessage(t("settings.keys.saved_encrypted"));
        await loadKeys();
        return;
      }
      const [info, written] = await KeyService.GenerateAndExport(algorithm, comment.trim(), passphrase);
      if (!written) {
        return;
      }
      setGenerated({ publicKey: info.public_key, fingerprint: info.fingerprint });
      successMessage(t("settings.keys.private_exported"));
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setSaving(false);
    }
  };

  const handleExport = async () => {
    if (!exportTarget) {
      return;
    }
    setExporting(true);
    try {
      const written = await KeyService.Export(exportTarget.id, exportPassphrase);
      if (written) {
        successMessage(t("settings.keys.private_exported"));
        setExportTarget(null);
        setExportPassphrase("");
      }
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setExporting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    try {
      await KeyService.Delete(deleteTarget.id);
      successMessage(t("settings.keys.deleted"));
      setDeleteTarget(null);
      await loadKeys();
    } catch (error) {
      errorMessage(translateServiceError(error));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Box>
      <Stack direction="row" sx={{ mb: 3, alignItems: "center", justifyContent: "space-between" }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          {t("settings.keys.title")}
        </Typography>
        <Button variant="contained" onClick={() => setGenerateOpen(true)}>
          {t("settings.keys.generate")}
        </Button>
      </Stack>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t("settings.keys.col_name")}</TableCell>
              <TableCell>{t("settings.keys.col_algorithm")}</TableCell>
              <TableCell>{t("settings.keys.col_fingerprint")}</TableCell>
              <TableCell>{t("settings.keys.col_comment")}</TableCell>
              <TableCell>{t("settings.keys.col_created")}</TableCell>
              <TableCell align="right">{t("settings.keys.col_actions")}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {keys.length === 0 && (
              <TableRow>
                <TableCell colSpan={6}>
                  <Typography variant="body2" color="text.secondary">
                    {loading ? t("common.loading") : t("settings.keys.empty")}
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {keys.map((key) => (
              <TableRow key={key.id} hover>
                <TableCell>{key.name}</TableCell>
                <TableCell>{key.algorithm}</TableCell>
                <TableCell>{key.fingerprint}</TableCell>
                <TableCell>{key.comment}</TableCell>
                <TableCell>{formatDateTime(key.created_at)}</TableCell>
                <TableCell align="right">
                  <Tooltip title={t("settings.keys.copy_public")}>
                    <IconButton
                      size="small"
                      onClick={() => copyPublicKey(key.public_key)}
                    >
                      <ContentCopy fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t("settings.keys.export")}>
                    <IconButton
                      size="small"
                      onClick={() => {
                        setExportPassphrase("");
                        setExportTarget(key);
                      }}
                    >
                      <FileDownload fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t("common.delete")}>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => setDeleteTarget(key)}
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

      <Dialog open={generateOpen} onClose={closeGenerate} maxWidth="sm" fullWidth>
        <DialogTitle>
          {generated
            ? t("settings.keys.dialog_public")
            : t("settings.keys.dialog_generate")}
        </DialogTitle>
        <DialogContent>
          {generated ? (
            <Stack spacing={2} sx={{ pt: 1 }}>
              <Typography variant="body2" color="text.secondary">
                {tf("settings.keys.fingerprint", generated.fingerprint)}
              </Typography>
              <TextField
                value={generated.publicKey}
                multiline
                minRows={3}
                fullWidth
                slotProps={{ input: { readOnly: true } }}
              />
            </Stack>
          ) : (
            <Stack spacing={2} sx={{ pt: 1 }}>
              <TextField
                select
                label={t("settings.keys.algorithm")}
                value={algorithm}
                onChange={(e) => setAlgorithm(e.target.value as KeyAlgorithm)}
                fullWidth
              >
                {algorithmOptions.map((item) => (
                  <MenuItem key={item.value} value={item.value}>
                    {item.label}
                  </MenuItem>
                ))}
              </TextField>
              <TextField
                label={t("settings.keys.comment")}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                fullWidth
                placeholder={t("settings.keys.comment_placeholder")}
              />
              <RadioGroup
                value={saveMode}
                onChange={(e) => setSaveMode(e.target.value as KeySaveMode)}
              >
                <FormControlLabel value={KeySaveMode.Store} control={<Radio />} label={t("settings.keys.save_store")} />
                <FormControlLabel value={KeySaveMode.Export} control={<Radio />} label={t("settings.keys.save_export")} />
              </RadioGroup>
              {saveMode === KeySaveMode.Store && (
                <TextField
                  label={t("settings.keys.name")}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  fullWidth
                  placeholder={t("settings.keys.name_placeholder")}
                />
              )}
              {saveMode === KeySaveMode.Export && (
                <TextField
                  label={t("settings.keys.passphrase")}
                  type="password"
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  fullWidth
                  placeholder={t("settings.keys.passphrase_placeholder")}
                />
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          {generated ? (
            <>
              <Button onClick={() => copyPublicKey(generated.publicKey)} startIcon={<ContentCopy />}>
                {t("settings.keys.copy_public")}
              </Button>
              <Button variant="contained" onClick={closeGenerate}>
                {t("settings.keys.done")}
              </Button>
            </>
          ) : (
            <>
              <Button onClick={closeGenerate} variant="outlined">
                {t("common.cancel")}
              </Button>
              <Button variant="contained" onClick={handleGenerate} loading={saving}>
                {t("settings.keys.generate")}
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>

      <Dialog
        open={exportTarget != null}
        onClose={() => {
          setExportTarget(null);
          setExportPassphrase("");
        }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{t("settings.keys.export_private_title")}</DialogTitle>
        <DialogContent>
          <TextField
            label={t("settings.keys.passphrase")}
            type="password"
            value={exportPassphrase}
            onChange={(e) => setExportPassphrase(e.target.value)}
            fullWidth
            sx={{ mt: 1 }}
            placeholder={t("settings.keys.passphrase_placeholder")}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportTarget(null)} variant="outlined">
            {t("common.cancel")}
          </Button>
          <Button variant="contained" onClick={handleExport} loading={exporting}>
            {t("settings.keys.pick_path")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={deleteTarget != null} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>{t("settings.keys.delete_title")}</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            {tf("settings.keys.delete_body", deleteTarget?.name ?? "")}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} variant="outlined">
            {t("common.cancel")}
          </Button>
          <Button color="error" variant="contained" onClick={handleDelete} loading={deleting}>
            {t("common.delete")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default KeySettings;
