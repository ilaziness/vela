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
import { parseCallServiceError } from "../../func/service";

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
      errorMessage(parseCallServiceError(error));
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
      successMessage("已删除已知主机");
      setDeleteTarget(null);
      await loadEntries();
    } catch (error) {
      errorMessage(parseCallServiceError(error));
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
          已知主机
        </Typography>
        <Button
          startIcon={<Refresh />}
          onClick={() => {
            setLoading(true);
            void loadEntries();
          }}
          disabled={loading}
        >
          刷新
        </Button>
      </Box>
      <Paper sx={{ p: 2 }} elevation={1}>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          已信任的 SSH 服务器指纹（known_hosts）。删除后再次连接将重新确认。
        </Typography>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>主机</TableCell>
              <TableCell>类型</TableCell>
              <TableCell>指纹</TableCell>
              <TableCell align="right" width={80}>
                操作
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} align="center">
                  {loading ? "加载中…" : "暂无已知主机"}
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
                  <Tooltip title="删除">
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
        <DialogTitle>删除已知主机</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            确定删除 {deleteTarget?.host}（{deleteTarget?.keyType}）的信任记录？
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} disabled={deleting}>
            取消
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => void handleDelete()}
            loading={deleting}
          >
            删除
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default KnownHostsSettings;
