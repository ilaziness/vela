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
import { translateServiceError } from "../func/service";
import { ProxyMode, ProxyType } from "../types/proxy";
import { collectTagSuggestions } from "../func/bookmarkMeta";
import { BookmarkColor } from "../types/bookmark";
import {
  displayBookmarkGroupName,
  t,
  tf,
} from "../i18n";

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
      errorMessage(t("bookmark.load_failed", translateServiceError(error)));
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
      errorMessage(
        t("bookmark.rename_group_failed", translateServiceError(error)),
      );
    }
  };

  const handleGroupIconChange = async (groupName: string, icon: string) => {
    try {
      await BookmarkService.UpdateGroup(groupName, groupName, icon);
      await loadBookmarks();
    } catch (error) {
      LogService.Warn(`Failed to update group icon: ${error}`);
      errorMessage(
        t("bookmark.update_group_icon_failed", translateServiceError(error)),
      );
    }
  };

  const handleGroupAdd = async (groupName: string) => {
    try {
      await BookmarkService.AddGroup(groupName);
      await loadBookmarks();
    } catch (error) {
      LogService.Warn(`Failed to add group: ${error}`);
      errorMessage(
        t("bookmark.add_group_failed", translateServiceError(error)),
      );
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
      errorMessage(
        t("bookmark.delete_group_failed", translateServiceError(error)),
      );
    }
  };

  const handleBookmarkAdd = (groupName: string) => {
    const group = bookmarks.find((g) => g.name === groupName);
    const bookmarkCount = group?.bookmarks?.length || 0;
    const newBookmark: SSHBookmark = {
      id: "",
      title: tf("bookmark.new", bookmarkCount + 1),
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
        successMessage(tf("bookmark.import_success", created));
      } else {
        successMessage(t("bookmark.import_none"));
      }
      showWarnings(res?.warnings);
    } catch (error) {
      LogService.Warn(`Import SSH config failed: ${error}`);
      errorMessage(t("bookmark.import_failed", translateServiceError(error)));
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
        successMessage(t("bookmark.export_success"));
      } else if ((res?.warnings || []).length === 0) {
        errorMessage(t("bookmark.export_empty"));
      }
      showWarnings(res?.warnings);
    } catch (error) {
      LogService.Warn(`Export SSH config failed: ${error}`);
      errorMessage(t("bookmark.export_failed", translateServiceError(error)));
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
      successMessage(t("bookmark.copy_success"));
    } catch (error) {
      LogService.Warn(`Failed to copy bookmark: ${error}`);
      errorMessage(t("bookmark.copy_failed", translateServiceError(error)));
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
      errorMessage(t("bookmark.delete_failed", translateServiceError(error)));
    }
  };

  const handleSaveBookmark = async (bookmark: SSHBookmark): Promise<SSHBookmark> => {
    try {
      const bookmarkId = await BookmarkService.SaveBookmark(bookmark);
      const savedBookmark = { ...bookmark, id: bookmarkId };
      successMessage(t("bookmark.save_success"));
      await loadBookmarks();
      setSelectedBookmark(savedBookmark);
      return savedBookmark;
    } catch (error) {
      LogService.Warn(`Failed to save bookmark: ${error}`);
      errorMessage(t("bookmark.save_failed", translateServiceError(error)));
      throw error;
    }
  };

  const handleTestConnection = async (bookmark: SSHBookmark) => {
    try {
      await BookmarkService.TestConnection(bookmark);
      successMessage(t("bookmark.test_success"));
    } catch (error) {
      LogService.Warn(`Connection test failed: ${error}`);
      errorMessage(t("bookmark.test_failed", translateServiceError(error)));
    }
  };

  const handleSaveAndConnect = async (bookmark: SSHBookmark): Promise<SSHBookmark> => {
    try {
      const bookmarkId = await BookmarkService.SaveAndConnect(bookmark);
      const savedBookmark = { ...bookmark, id: bookmarkId };
      successMessage(t("bookmark.save_connect_success"));
      await loadBookmarks();
      setSelectedBookmark(savedBookmark);
      onRequestClose?.();
      return savedBookmark;
    } catch (error) {
      LogService.Warn(`Failed to save and connect: ${error}`);
      errorMessage(
        t("bookmark.save_connect_failed", translateServiceError(error)),
      );
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
            {t("bookmark.import_config")}
          </Button>
          <Button
            size="small"
            variant="outlined"
            onClick={() => {
              setExportGroup(EXPORT_ALL);
              setExportOpen(true);
            }}
          >
            {t("bookmark.export_config")}
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
        <DialogTitle>{t("bookmark.import_dialog_title")}</DialogTitle>
        <DialogContent>
          <TextField
            select
            fullWidth
            size="small"
            label={t("bookmark.import_target_group")}
            value={importGroup}
            onChange={(e) => setImportGroup(e.target.value)}
            sx={{ mt: 1, minWidth: 320 }}
          >
            <MenuItem value={IMPORT_GROUP_DEFAULT}>
              {t("bookmark.group.imported")}
            </MenuItem>
            {bookmarks
              .map((g) => g.name)
              .filter((n) => n !== IMPORT_GROUP_DEFAULT)
              .map((name) => (
                <MenuItem key={name} value={name}>
                  {displayBookmarkGroupName(name)}
                </MenuItem>
              ))}
          </TextField>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
            {t("bookmark.import_hint")}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setImportOpen(false)} disabled={busyIO}>
            {t("common.cancel")}
          </Button>
          <Button variant="contained" onClick={handleImportSSHConfig} disabled={busyIO}>
            {t("bookmark.import_select")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={exportOpen} onClose={() => !busyIO && setExportOpen(false)}>
        <DialogTitle>{t("bookmark.export_dialog_title")}</DialogTitle>
        <DialogContent>
          <TextField
            select
            fullWidth
            size="small"
            label={t("bookmark.export_scope")}
            value={exportGroup}
            onChange={(e) => setExportGroup(e.target.value)}
            sx={{ mt: 1, minWidth: 320 }}
          >
            <MenuItem value={EXPORT_ALL}>{t("bookmark.export_all")}</MenuItem>
            {bookmarks.map((g) => (
              <MenuItem key={g.name} value={g.name}>
                {displayBookmarkGroupName(g.name)}
              </MenuItem>
            ))}
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportOpen(false)} disabled={busyIO}>
            {t("common.cancel")}
          </Button>
          <Button variant="contained" onClick={handleExportSSHConfig} disabled={busyIO}>
            {t("common.export")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={warningsOpen} onClose={() => setWarningsOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{t("bookmark.warnings_title")}</DialogTitle>
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
          <Button onClick={() => setWarningsOpen(false)}>{t("common.close")}</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default Bookmark;
