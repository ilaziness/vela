import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Dialog,
  AppBar,
  Toolbar,
  Typography,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Backdrop,
} from "@mui/material";
import SettingsIcon from "@mui/icons-material/Settings";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarksIcon from "@mui/icons-material/Bookmarks";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import CloseIcon from "@mui/icons-material/Close";
import CloudUploadIcon from "@mui/icons-material/CloudUpload";
import HandymanIcon from "@mui/icons-material/Handyman";
import TerminalIcon from "@mui/icons-material/Terminal";
import AddToQueueIcon from "@mui/icons-material/AddToQueue";
import AutoAwesomeIcon from "@mui/icons-material/AutoAwesome";
import UpgradeIcon from "@mui/icons-material/Upgrade";
import {
  AppService,
  ConfigService,
  CommandService,
  LogService,
  ToolService,
  NewVersion,
} from "../../bindings/github.com/ilaziness/vela/services";
import { UploadSync } from "../../bindings/github.com/ilaziness/vela/services/syncservice";
import { ReadConfig } from "../../bindings/github.com/ilaziness/vela/services/configservice";
import { useSSHTabsStore } from "../stores/ssh";
import { useAIAssistantStore } from "../stores/aiAssistant";
import { useUIStore } from "../stores/ui";
import { genTabIndex, parseCallServiceError } from "../func/service";
import { APP_HEADER_RAIL_WIDTH } from "../func/aiSidebar";
import React, { useState, useEffect, useMemo, useCallback } from "react";
import { useMessageStore } from "../stores/message";
import ThemeSwitcher from "./ThemeSwitcher";
import { railIconButtonSx } from "./railIconButtonSx";
import { Events } from "@wailsio/runtime";
import { dragRegionSx, useFrameless } from "../hooks/useFrameless";
import BookmarkManager from "./Bookmark";
import BookmarkConnectPanel from "./BookmarkConnectPanel";
import Loading from "./Loading";

const UpdateAvailableDialog = React.lazy(
  () => import("./UpdateAvailableDialog"),
);

export default function Header() {
  const { sshTabs, pushTab } = useSSHTabsStore();
  const { errorMessage, successMessage } = useMessageStore();
  const { toggleSidebarOpen } = useAIAssistantStore();
  const bookmarkManageOpen = useUIStore((s) => s.bookmarkManageOpen);
  const setBookmarkManageOpen = useUIStore((s) => s.setBookmarkManageOpen);
  const bookmarkConnectOpen = useUIStore((s) => s.bookmarkConnectOpen);
  const setBookmarkConnectOpen = useUIStore((s) => s.setBookmarkConnectOpen);
  const toggleBookmarkConnect = useUIStore((s) => s.toggleBookmarkConnect);
  const frameless = useFrameless();

  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [newVersion, setNewVersion] = useState<NewVersion | null>(null);
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false);
  const [updateDialogMounted, setUpdateDialogMounted] = useState(false);

  useEffect(() => {
    if (updateDialogOpen) {
      setUpdateDialogMounted(true);
    }
  }, [updateDialogOpen]);

  const showSettingWindow = useCallback(() => {
    ConfigService.ShowWindow();
  }, []);

  const handleAIAssistant = useCallback(() => {
    toggleSidebarOpen();
  }, [toggleSidebarOpen]);

  const handleAddTab = useCallback(() => {
    const number = sshTabs.length + 1;
    const newIndex = `${genTabIndex()}`;
    pushTab({
      index: newIndex,
      name: `新建连接 ${number}`,
    });
  }, [sshTabs.length, pushTab]);

  const handleNewMainWindow = useCallback(() => {
    AppService.NewMainWindow();
  }, []);

  const handleBackupClick = useCallback(async () => {
    try {
      const config = await ReadConfig();
      const syncConfig = config?.Sync;
      if (
        !syncConfig?.serverUrl ||
        !syncConfig?.syncId ||
        !syncConfig?.userKey
      ) {
        errorMessage("请先前往设置页面配置同步参数");
        return;
      }
      setConfirmDialogOpen(true);
    } catch (error) {
      LogService.Error("Failed to read config: " + String(error));
      errorMessage("读取配置失败");
    }
  }, [errorMessage]);

  const menuItems = useMemo(
    () => [
      { title: "新建空白标签", icon: <AddBoxIcon />, onClick: handleAddTab },
      {
        title: "新建窗口",
        icon: <AddToQueueIcon />,
        onClick: handleNewMainWindow,
      },
      {
        title: "书签",
        icon: <BookmarkIcon />,
        onClick: toggleBookmarkConnect,
        active: bookmarkConnectOpen,
      },
      {
        title: "书签管理",
        icon: <BookmarksIcon />,
        onClick: () => setBookmarkManageOpen(true),
      },
      {
        title: "命令面板",
        icon: <TerminalIcon />,
        onClick: () => CommandService.ShowWindow(),
      },
      {
        title: "AI助手",
        icon: <AutoAwesomeIcon />,
        onClick: handleAIAssistant,
      },
      {
        title: "上传备份",
        icon: <CloudUploadIcon />,
        onClick: handleBackupClick,
      },
      {
        title: "工具",
        icon: <HandymanIcon />,
        onClick: () => ToolService.ShowWindow(),
      },
    ],
    [
      handleAddTab,
      handleNewMainWindow,
      toggleBookmarkConnect,
      bookmarkConnectOpen,
      handleBackupClick,
      handleAIAssistant,
      setBookmarkManageOpen,
    ],
  );

  useEffect(() => {
    const unsubscribeUpdate = Events.On("eventNewVersion", (event: any) => {
      const data = event?.data as NewVersion | undefined;
      if (data?.Version) {
        setNewVersion(data);
      }
    });

    return () => {
      unsubscribeUpdate();
    };
  }, []);

  const handleCancelBackup = () => {
    setConfirmDialogOpen(false);
  };

  const handleConfirmBackup = () => {
    setConfirmDialogOpen(false);
    handleUpload();
  };

  const handleUpload = async () => {
    setUploading(true);
    try {
      await UploadSync();
      successMessage("数据上传成功");
    } catch (error) {
      LogService.Error("Upload failed: " + String(error));
      errorMessage("数据上传失败: " + parseCallServiceError(error));
    } finally {
      setUploading(false);
    }
  };

  return (
    <Box
      component="header"
      sx={{
        display: "flex",
        flexDirection: "row",
        flexShrink: 0,
        height: "100%",
      }}
    >
      <Box
        sx={(theme) => ({
          width: APP_HEADER_RAIL_WIDTH,
          flexShrink: 0,
          height: "100%",
          overflow: "hidden",
          backgroundColor: theme.palette.background.paper,
          borderRight: `1px solid ${theme.palette.divider}`,
        })}
      >
        <Stack
          direction="column"
          spacing={1}
          sx={{ alignItems: "center", padding: 0.5, height: "100%" }}
        >
          <Box
            sx={{
              ...dragRegionSx(frameless),
              cursor: frameless ? "move" : undefined,
            }}
          >
            <img
              src="/appicon.png"
              alt="Logo"
              style={{ width: "32px", height: "32px" }}
            />
          </Box>

          {menuItems.map((item, index) => (
            <Tooltip key={index} title={item.title}>
              <IconButton
                sx={railIconButtonSx}
                onClick={item.onClick}
                color={item.active ? "primary" : "default"}
                aria-pressed={item.active ? true : undefined}
              >
                {item.icon}
              </IconButton>
            </Tooltip>
          ))}

          <Box sx={{ flex: 1 }} />

          <ThemeSwitcher />

          {newVersion ? (
            <Tooltip title="发现新版本">
              <IconButton
                sx={railIconButtonSx}
                color="primary"
                onClick={() => setUpdateDialogOpen(true)}
              >
                <UpgradeIcon />
              </IconButton>
            </Tooltip>
          ) : null}

          <Tooltip title="设置">
            <IconButton sx={railIconButtonSx} onClick={showSettingWindow}>
              <SettingsIcon />
            </IconButton>
          </Tooltip>
        </Stack>
      </Box>

      <BookmarkConnectPanel
        open={bookmarkConnectOpen}
        onRequestClose={() => setBookmarkConnectOpen(false)}
      />

      {updateDialogMounted && (
        <React.Suspense fallback={null}>
          <UpdateAvailableDialog
            open={updateDialogOpen}
            onClose={() => setUpdateDialogOpen(false)}
            newVersion={newVersion}
          />
        </React.Suspense>
      )}

      <Dialog
        fullScreen
        open={bookmarkManageOpen}
        onClose={() => setBookmarkManageOpen(false)}
      >
        <AppBar
          position="static"
          color="default"
          elevation={1}
          sx={{ ...dragRegionSx(frameless) }}
        >
          <Toolbar>
            <IconButton
              edge="start"
              onClick={() => setBookmarkManageOpen(false)}
              size="large"
            >
              <CloseIcon />
            </IconButton>
            <Typography sx={{ ml: 1 }} variant="h6">
              书签管理
            </Typography>
          </Toolbar>
        </AppBar>
        <Box sx={{ height: "calc(100% - 64px)" }}>
          <BookmarkManager
            onRequestClose={() => setBookmarkManageOpen(false)}
          />
        </Box>
      </Dialog>

      <Dialog
        open={confirmDialogOpen}
        onClose={handleCancelBackup}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>确认上传</DialogTitle>
        <DialogContent>
          <Typography>确定要上传备份吗？</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCancelBackup}>取消</Button>
          <Button variant="contained" onClick={handleConfirmBackup}>
            确定
          </Button>
        </DialogActions>
      </Dialog>

      <Backdrop
        open={uploading}
        sx={(theme) => ({
          zIndex: theme.zIndex.modal + 1,
          backgroundColor: "rgba(0, 0, 0, 0.5)",
        })}
      >
        <Loading message="正在上传备份..." size={60} />
      </Backdrop>
    </Box>
  );
}
