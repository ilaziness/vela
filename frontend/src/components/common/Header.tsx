import {
  Box,
  Stack,
  Dialog,
  AppBar,
  Toolbar,
  Typography,
  IconButton,
} from "@mui/material";
import SettingsIcon from "@mui/icons-material/Settings";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarksIcon from "@mui/icons-material/Bookmarks";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import CloseIcon from "@mui/icons-material/Close";
import SettingsBackupRestoreIcon from "@mui/icons-material/SettingsBackupRestore";
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
} from "../../../bindings/github.com/ilaziness/vela/services";
import { ReadConfig } from "../../../bindings/github.com/ilaziness/vela/services/configservice";
import { useSessionTabsStore } from "../../stores/sessionTabs";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { useUIStore } from "../../stores/ui";
import { genTabIndex } from "../../func/service";
import { isSyncConfigured } from "../../func/sync";
import { APP_HEADER_RAIL_WIDTH } from "../../func/aiSidebar";
import { ShortcutAction } from "../../func/shortcuts";
import React, { useState, useEffect, useMemo, useCallback } from "react";
import { useMessageStore } from "../../stores/message";
import ThemeSwitcher from "../common/ThemeSwitcher";
import AppLogo from "../common/AppLogo";
import { RailMenuButton } from "../common/RailIconButton";
import { Events } from "@wailsio/runtime";
import { dragRegionSx, useFrameless } from "../../hooks/useFrameless";
import BookmarkManager from "../bookmark/Bookmark";
import BookmarkConnectPanel from "../bookmark/BookmarkConnectPanel";
import BackupRestoreDialogs from "./BackupRestoreDialogs";
import { newConnectionTabName, t, useLanguage } from "../../i18n";

const UpdateAvailableDialog = React.lazy(
  () => import("./UpdateAvailableDialog"),
);

export default function Header() {
  const { sessionTabs, pushTab } = useSessionTabsStore();
  const { errorMessage } = useMessageStore();
  const { toggleSidebarOpen } = useAIAssistantStore();
  const bookmarkManageOpen = useUIStore((s) => s.bookmarkManageOpen);
  const setBookmarkManageOpen = useUIStore((s) => s.setBookmarkManageOpen);
  const bookmarkConnectOpen = useUIStore((s) => s.bookmarkConnectOpen);
  const setBookmarkConnectOpen = useUIStore((s) => s.setBookmarkConnectOpen);
  const toggleBookmarkConnect = useUIStore((s) => s.toggleBookmarkConnect);
  const frameless = useFrameless();
  const [backupDialogOpen, setBackupDialogOpen] = useState(false);
  const [newVersion, setNewVersion] = useState<NewVersion | null>(null);
  const lang = useLanguage();
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false);
  const [updateDialogMounted, setUpdateDialogMounted] = useState(false);

  useEffect(() => {
    if (updateDialogOpen) {
      setUpdateDialogMounted(true);
    }
  }, [updateDialogOpen]);

  const handleAddTab = useCallback(() => {
    const number = sessionTabs.length + 1;
    pushTab({
      index: `${genTabIndex()}`,
      name: newConnectionTabName(number),
    });
  }, [sessionTabs.length, pushTab]);

  const handleBackupClick = useCallback(async () => {
    if (backupDialogOpen) return;
    try {
      const config = await ReadConfig();
      if (!isSyncConfigured(config?.Sync)) {
        errorMessage(t("header.sync_config_required"));
        return;
      }
      setBackupDialogOpen(true);
    } catch (error) {
      LogService.Error("Failed to read config: " + String(error));
      errorMessage(t("header.read_config_failed"));
    }
  }, [backupDialogOpen, errorMessage]);

  const menuItems = useMemo(() => {
    void lang;
    return [
      {
        id: "new_blank_tab",
        title: t("header.new_blank_tab"),
        icon: <AddBoxIcon />,
        onClick: handleAddTab,
      },
      {
        id: "new_window",
        title: t("header.new_window"),
        icon: <AddToQueueIcon />,
        onClick: () => AppService.NewMainWindow(),
      },
      {
        id: "bookmarks",
        title: t("header.bookmarks"),
        icon: <BookmarkIcon />,
        onClick: toggleBookmarkConnect,
        active: bookmarkConnectOpen,
        shortcutAction: ShortcutAction.PanelBookmarkConnect,
      },
      {
        id: "bookmark_manager",
        title: t("header.bookmark_manager"),
        icon: <BookmarksIcon />,
        onClick: () => setBookmarkManageOpen(true),
        shortcutAction: ShortcutAction.PanelBookmarks,
      },
      {
        id: "command_palette",
        title: t("header.command_palette"),
        icon: <TerminalIcon />,
        onClick: () => CommandService.ShowWindow(),
        shortcutAction: ShortcutAction.PanelCommand,
      },
      {
        id: "ai_assistant",
        title: t("header.ai_assistant"),
        icon: <AutoAwesomeIcon />,
        onClick: () => toggleSidebarOpen(),
        shortcutAction: ShortcutAction.PanelAI,
      },
      {
        id: "backup_restore",
        title: t("header.backup_restore"),
        icon: <SettingsBackupRestoreIcon />,
        onClick: () => void handleBackupClick(),
      },
      {
        id: "tools",
        title: t("header.tools"),
        icon: <HandymanIcon />,
        onClick: () => ToolService.ShowWindow(),
        shortcutAction: ShortcutAction.PanelTools,
      },
    ];
  }, [
    lang,
    handleAddTab,
    toggleBookmarkConnect,
    bookmarkConnectOpen,
    handleBackupClick,
    toggleSidebarOpen,
    setBookmarkManageOpen,
  ]);

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
          spacing={0.25}
          sx={{ alignItems: "center", padding: 0.25, height: "100%" }}
        >
          <AppLogo frameless={frameless} />

          {menuItems.map((item) => (
            <RailMenuButton
              key={item.id}
              title={item.title}
              shortcutAction={item.shortcutAction}
              active={item.active}
              onClick={item.onClick}
            >
              {item.icon}
            </RailMenuButton>
          ))}

          <Box sx={{ flex: 1 }} />

          <ThemeSwitcher />

          {newVersion ? (
            <RailMenuButton
              title={t("header.update_available")}
              active
              onClick={() => setUpdateDialogOpen(true)}
            >
              <UpgradeIcon />
            </RailMenuButton>
          ) : null}

          <RailMenuButton
            title={t("header.settings")}
            shortcutAction={ShortcutAction.PanelSettings}
            onClick={() => ConfigService.ShowWindow()}
          >
            <SettingsIcon />
          </RailMenuButton>
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
              {t("header.bookmark_manager")}
            </Typography>
          </Toolbar>
        </AppBar>
        <Box sx={{ height: "calc(100% - 64px)" }}>
          <BookmarkManager
            onRequestClose={() => setBookmarkManageOpen(false)}
          />
        </Box>
      </Dialog>

      <BackupRestoreDialogs
        open={backupDialogOpen}
        onClose={() => setBackupDialogOpen(false)}
      />
    </Box>
  );
}
