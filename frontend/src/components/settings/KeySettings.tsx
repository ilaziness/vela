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
import { parseCallServiceError } from "../../func/service";

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
      errorMessage(parseCallServiceError(error));
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
      successMessage("公钥已复制");
    } catch (error) {
      errorMessage("复制公钥失败");
      console.error(error);
    }
  };

  const handleGenerate = async () => {
    setSaving(true);
    try {
      if (saveMode === KeySaveMode.Store) {
        const info = await KeyService.GenerateAndSave(name.trim(), algorithm, comment.trim());
        setGenerated({ publicKey: info.public_key, fingerprint: info.fingerprint });
        successMessage("密钥已加密保存");
        await loadKeys();
        return;
      }
      const [info, written] = await KeyService.GenerateAndExport(algorithm, comment.trim(), passphrase);
      if (!written) {
        return;
      }
      setGenerated({ publicKey: info.public_key, fingerprint: info.fingerprint });
      successMessage("私钥已导出");
    } catch (error) {
      errorMessage(parseCallServiceError(error));
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
        successMessage("私钥已导出");
        setExportTarget(null);
        setExportPassphrase("");
      }
    } catch (error) {
      errorMessage(parseCallServiceError(error));
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
      successMessage("密钥已删除");
      setDeleteTarget(null);
      await loadKeys();
    } catch (error) {
      errorMessage(parseCallServiceError(error));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Box>
      <Stack direction="row" sx={{ mb: 3, alignItems: "center", justifyContent: "space-between" }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          身份密钥
        </Typography>
        <Button variant="contained" onClick={() => setGenerateOpen(true)}>
          生成密钥
        </Button>
      </Stack>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>名称</TableCell>
              <TableCell>算法</TableCell>
              <TableCell>指纹</TableCell>
              <TableCell>注释</TableCell>
              <TableCell>创建时间</TableCell>
              <TableCell align="right">操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {keys.length === 0 && (
              <TableRow>
                <TableCell colSpan={6}>
                  <Typography variant="body2" color="text.secondary">
                    {loading ? "加载中…" : "还没有密钥"}
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
                <TableCell>{key.created_at}</TableCell>
                <TableCell align="right">
                  <Tooltip title="复制公钥">
                    <IconButton
                      size="small"
                      onClick={() => copyPublicKey(key.public_key)}
                    >
                      <ContentCopy fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="导出">
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
                  <Tooltip title="删除">
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
        <DialogTitle>{generated ? "公钥" : "生成密钥"}</DialogTitle>
        <DialogContent>
          {generated ? (
            <Stack spacing={2} sx={{ pt: 1 }}>
              <Typography variant="body2" color="text.secondary">
                指纹 {generated.fingerprint}
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
                label="算法"
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
                label="注释"
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                fullWidth
                placeholder="写入公钥行末尾，例如 user@host"
              />
              <RadioGroup
                value={saveMode}
                onChange={(e) => setSaveMode(e.target.value as KeySaveMode)}
              >
                <FormControlLabel value={KeySaveMode.Store} control={<Radio />} label="加密保存到应用" />
                <FormControlLabel value={KeySaveMode.Export} control={<Radio />} label="仅导出到文件" />
              </RadioGroup>
              {saveMode === KeySaveMode.Store && (
                <TextField
                  label="名称"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  fullWidth
                  placeholder="留空则按算法和时间命名"
                />
              )}
              {saveMode === KeySaveMode.Export && (
                <TextField
                  label="私钥口令"
                  type="password"
                  value={passphrase}
                  onChange={(e) => setPassphrase(e.target.value)}
                  fullWidth
                  placeholder="可选，留空则不加密文件"
                />
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          {generated ? (
            <>
              <Button onClick={() => copyPublicKey(generated.publicKey)} startIcon={<ContentCopy />}>
                复制公钥
              </Button>
              <Button variant="contained" onClick={closeGenerate}>
                完成
              </Button>
            </>
          ) : (
            <>
              <Button onClick={closeGenerate} variant="outlined">
                取消
              </Button>
              <Button variant="contained" onClick={handleGenerate} loading={saving}>
                生成
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
        <DialogTitle>导出私钥</DialogTitle>
        <DialogContent>
          <TextField
            label="私钥口令"
            type="password"
            value={exportPassphrase}
            onChange={(e) => setExportPassphrase(e.target.value)}
            fullWidth
            sx={{ mt: 1 }}
            placeholder="可选，留空则不加密文件"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportTarget(null)} variant="outlined">
            取消
          </Button>
          <Button variant="contained" onClick={handleExport} loading={exporting}>
            选择路径
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={deleteTarget != null} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>删除密钥</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            删除「{deleteTarget?.name}」后无法恢复。正在被书签使用的密钥不能删除。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} variant="outlined">
            取消
          </Button>
          <Button color="error" variant="contained" onClick={handleDelete} loading={deleting}>
            删除
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default KeySettings;
