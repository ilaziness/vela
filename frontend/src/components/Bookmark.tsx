import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import {
  BookmarkService,
  LogService,
  SSHBookmark,
  BookmarkGroup,
} from "../../bindings/github.com/ilaziness/vela/services";
import BookmarkTree from "./BookmarkTree";
import BookmarkForm from "./BookmarkForm";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";
import { ProxyMode, ProxyType } from "../types/proxy";
import { collectTagSuggestions } from "../func/bookmarkMeta";
import { BookmarkColor } from "../types/bookmark";

const IMPORT_GROUP_DEFAULT = "Imported";
const EXPORT_ALL = "";

interface BookmarkProps {
  onRequestClose?: () => void;
}

const Bookmark: React.FC<BookmarkProps> = ({ onRequestClose }) => {
  const [bookmarks, setBookmarks] = useState<BookmarkGroup[]>([]);
  const [selectedBookmark, setSelectedBookmark] = useState<SSHBookmark | null>(
    null,
  );
  const [importOpen, setImportOpen] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const [importGroup, setImportGroup] = useState(IMPORT_GROUP_DEFAULT);
  const [exportGroup, setExportGroup] = useState(EXPORT_ALL);
  const [busyIO, setBusyIO] = useState(false);
  const [warningsOpen, setWarningsOpen] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const { errorMessage, successMessage } = useMessageStore();

  const tagSuggestions = useMemo(
    () => collectTagSuggestions(bookmarks),
    [bookmarks],
  );

  const loadBookmarks = useCallback(async () => {
    try {
      const config = await BookmarkService.ListBookmarks();
      if (config && Array.isArray(config)) {
        const validBookmarks = config.filter(
          (group): group is BookmarkGroup =>
            group !== null && group !== undefined,
        );
        setBookmarks(validBookmarks);
      } else {
        setBookmarks([]);
      }
    } catch (error) {
      LogService.Warn(`Failed to load bookmarks:${error}`);
      errorMessage("加载书签失败: " + parseCallServiceError(error));
    }
  }, [errorMessage]);

  useEffect(() => {
    void loadBookmarks();
  }, [loadBookmarks]);

  const handleBookmarkSelect = (bookmark: SSHBookmark) => {
    setSelectedBookmark(bookmark);
  };

  const handleGroupRename = async (
    oldGroupName: string,
    newGroupName: string,
  ) => {
    try {
      const group = bookmarks.find((g) => g.name === oldGroupName);
      await BookmarkService.UpdateGroup(
        oldGroupName,
        newGroupName,
        group?.icon || "",
      );
      await loadBookmarks();
      setSelectedBookmark((prev) =>
        prev?.group_name === oldGroupName
          ? { ...prev, group_name: newGroupName }
          : prev,
      );
    } catch (error) {
      LogService.Warn(`Failed to rename group: ${error}`);
      errorMessage("重命名分组失败: " + parseCallServiceError(error));
    }
  };

  const handleGroupIconChange = async (groupName: string, icon: string) => {
    try {
      await BookmarkService.UpdateGroup(groupName, groupName, icon);
      await loadBookmarks();
    } catch (error) {
      LogService.Warn(`Failed to update group icon: ${error}`);
      errorMessage("更新分组图标失败: " + parseCallServiceError(error));
    }
  };

  const handleGroupAdd = async (groupName: string) => {
    try {
      await BookmarkService.AddGroup(groupName);
      await loadBookmarks();
    } catch (error) {
      LogService.Warn(`Failed to add group: ${error}`);
      errorMessage("添加分组失败: " + parseCallServiceError(error));
    }
  };

  const handleGroupDelete = async (groupName: string) => {
    try {
      await BookmarkService.DeleteGroup(groupName);
      await loadBookmarks();
      if (selectedBookmark?.group_name === groupName) {
        setSelectedBookmark(null);
      }
    } catch (error) {
      LogService.Warn(`Failed to delete group: ${error}`);
      errorMessage("删除分组失败: " + parseCallServiceError(error));
    }
  };

  const handleBookmarkAdd = (groupName: string) => {
    const group = bookmarks.find((g) => g.name === groupName);
    const bookmarkCount = group?.bookmarks?.length || 0;
    const newBookmark: SSHBookmark = {
      id: "",
      title: `新书签 ${bookmarkCount + 1}`,
      group_name: groupName,
      host: "",
      port: 22,
      private_key: "",
      private_key_password: "",
      proxy_jump_id: "",
      user: "",
      password: "",
      icon: "",
      use_agent: true,
      ssh_key_id: "",
      certificate: "",
      forward_agent: false,
      proxy_mode: ProxyMode.Inherit,
      proxy_type: ProxyType.None,
      proxy_host: "",
      proxy_port: 0,
      proxy_user: "",
      proxy_password: "",
      startup_cmd: "",
      env_vars: "",
      term: "",
      charset: "utf-8",
      tags: [],
      color: BookmarkColor.None,
    };
    setSelectedBookmark(newBookmark);
  };

  const showWarnings = (list: string[] | null | undefined) => {
    const cleaned = (list || []).filter(Boolean);
    if (cleaned.length === 0) {
      return;
    }
    setWarnings(cleaned);
    setWarningsOpen(true);
  };

  const handleImportSSHConfig = async () => {
    setBusyIO(true);
    try {
      const res = await BookmarkService.ImportSSHConfig(importGroup.trim() || IMPORT_GROUP_DEFAULT);
      if (res?.cancelled) {
        return;
      }
      setImportOpen(false);
      await loadBookmarks();
      const created = res?.created ?? 0;
      if (created > 0) {
        successMessage(`已导入 ${created} 个书签`);
      } else {
        successMessage("导入完成（未新建书签）");
      }
      showWarnings(res?.warnings);
    } catch (error) {
      LogService.Warn(`Import SSH config failed: ${error}`);
      errorMessage("导入 ssh_config 失败: " + parseCallServiceError(error));
    } finally {
      setBusyIO(false);
    }
  };

  const handleExportSSHConfig = async () => {
    setBusyIO(true);
    try {
      const res = await BookmarkService.ExportSSHConfig(exportGroup);
      if (res?.cancelled) {
        return;
      }
      setExportOpen(false);
      if (res?.written) {
        successMessage("已导出 OpenSSH 配置");
      } else if ((res?.warnings || []).length === 0) {
        errorMessage("没有可导出的内容");
      }
      showWarnings(res?.warnings);
    } catch (error) {
      LogService.Warn(`Export SSH config failed: ${error}`);
      errorMessage("导出 ssh_config 失败: " + parseCallServiceError(error));
    } finally {
      setBusyIO(false);
    }
  };

  const handleBookmarkCopy = async (bookmarkId: string) => {
    try {
      const copied = await BookmarkService.CopyBookmark(bookmarkId);
      await loadBookmarks();
      if (copied) {
        setSelectedBookmark(copied);
      }
      successMessage("书签复制成功");
    } catch (error) {
      LogService.Warn(`Failed to copy bookmark: ${error}`);
      errorMessage("复制书签失败: " + parseCallServiceError(error));
    }
  };

  const handleBookmarkDelete = async (bookmarkId: string) => {
    try {
      await BookmarkService.DeleteBookmark(bookmarkId);
      await loadBookmarks();
      if (selectedBookmark?.id === bookmarkId) {
        setSelectedBookmark(null);
      }
    } catch (error) {
      LogService.Warn(`Failed to delete bookmark: ${error}`);
      errorMessage("删除书签失败: " + parseCallServiceError(error));
    }
  };

  const handleSaveBookmark = async (bookmark: SSHBookmark): Promise<SSHBookmark> => {
    try {
      const bookmarkId = await BookmarkService.SaveBookmark(bookmark);
      const savedBookmark = { ...bookmark, id: bookmarkId };
      successMessage("书签保存成功");
      await loadBookmarks();
      setSelectedBookmark(savedBookmark);
      return savedBookmark;
    } catch (error) {
      LogService.Warn(`Failed to save bookmark: ${error}`);
      errorMessage("保存书签失败: " + parseCallServiceError(error));
      throw error;
    }
  };

  const handleTestConnection = async (bookmark: SSHBookmark) => {
    try {
      await BookmarkService.TestConnection(bookmark);
      successMessage("连接测试成功");
    } catch (error) {
      LogService.Warn(`Connection test failed: ${error}`);
      errorMessage("连接测试失败: " + parseCallServiceError(error));
    }
  };

  const handleSaveAndConnect = async (bookmark: SSHBookmark): Promise<SSHBookmark> => {
    try {
      const bookmarkId = await BookmarkService.SaveAndConnect(bookmark);
      const savedBookmark = { ...bookmark, id: bookmarkId };
      successMessage("书签保存并连接成功");
      await loadBookmarks();
      setSelectedBookmark(savedBookmark);
      onRequestClose?.();
      return savedBookmark;
    } catch (error) {
      LogService.Warn(`Failed to save and connect: ${error}`);
      errorMessage("保存并连接失败: " + parseCallServiceError(error));
      throw error;
    }
  };

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        overflow: "hidden",
      }}
    >
      <Paper
        sx={{
          width: 280,
          flexShrink: 0,
          borderRight: 1,
          borderColor: "divider",
          display: "flex",
          flexDirection: "column",
        }}
        elevation={0}
        square
      >
        <Stack
          direction="row"
          spacing={1}
          sx={{ px: 1.5, pt: 1.5, pb: 0.5 }}
        >
          <Button
            size="small"
            variant="outlined"
            onClick={() => {
              setImportGroup(IMPORT_GROUP_DEFAULT);
              setImportOpen(true);
            }}
          >
            导入配置
          </Button>
          <Button
            size="small"
            variant="outlined"
            onClick={() => {
              setExportGroup(EXPORT_ALL);
              setExportOpen(true);
            }}
          >
            导出配置
          </Button>
        </Stack>
        <BookmarkTree
          bookmarks={bookmarks}
          tagSuggestions={tagSuggestions}
          selectedBookmark={selectedBookmark}
          onBookmarkSelect={handleBookmarkSelect}
          onGroupRename={handleGroupRename}
          onGroupIconChange={handleGroupIconChange}
          onGroupAdd={handleGroupAdd}
          onGroupDelete={handleGroupDelete}
          onBookmarkAdd={handleBookmarkAdd}
          onBookmarkCopy={handleBookmarkCopy}
          onBookmarkDelete={handleBookmarkDelete}
        />
      </Paper>
      <Box
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <BookmarkForm
          bookmark={selectedBookmark}
          groupNames={bookmarks.map((g) => g.name)}
          tagSuggestions={tagSuggestions}
          onSave={handleSaveBookmark}
          onTestConnection={handleTestConnection}
          onSaveAndConnect={handleSaveAndConnect}
        />
      </Box>

      <Dialog open={importOpen} onClose={() => !busyIO && setImportOpen(false)}>
        <DialogTitle>导入 OpenSSH 配置</DialogTitle>
        <DialogContent>
          <TextField
            select
            fullWidth
            size="small"
            label="目标分组"
            value={importGroup}
            onChange={(e) => setImportGroup(e.target.value)}
            sx={{ mt: 1, minWidth: 320 }}
          >
            <MenuItem value={IMPORT_GROUP_DEFAULT}>{IMPORT_GROUP_DEFAULT}</MenuItem>
            {bookmarks
              .map((g) => g.name)
              .filter((n) => n !== IMPORT_GROUP_DEFAULT)
              .map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
          </TextField>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
            将选择本机 ssh_config 文件，解析 Host 并生成书签。无法识别的指令会提示。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setImportOpen(false)} disabled={busyIO}>
            取消
          </Button>
          <Button variant="contained" onClick={handleImportSSHConfig} disabled={busyIO}>
            选择文件并导入
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={exportOpen} onClose={() => !busyIO && setExportOpen(false)}>
        <DialogTitle>导出 OpenSSH 配置</DialogTitle>
        <DialogContent>
          <TextField
            select
            fullWidth
            size="small"
            label="导出范围"
            value={exportGroup}
            onChange={(e) => setExportGroup(e.target.value)}
            sx={{ mt: 1, minWidth: 320 }}
          >
            <MenuItem value={EXPORT_ALL}>全部书签</MenuItem>
            {bookmarks.map((g) => (
              <MenuItem key={g.name} value={g.name}>
                {g.name}
              </MenuItem>
            ))}
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportOpen(false)} disabled={busyIO}>
            取消
          </Button>
          <Button variant="contained" onClick={handleExportSSHConfig} disabled={busyIO}>
            导出
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={warningsOpen} onClose={() => setWarningsOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>导入/导出提示</DialogTitle>
        <DialogContent>
          <Box
            component="ul"
            sx={{
              m: 0,
              pl: 2,
              maxHeight: 360,
              overflow: "auto",
            }}
          >
            {warnings.map((w, i) => (
              <Typography component="li" key={`${i}-${w}`} variant="body2" sx={{ mb: 0.5 }}>
                {w}
              </Typography>
            ))}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setWarningsOpen(false)}>关闭</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default Bookmark;
