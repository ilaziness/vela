import { Box, Stack } from "@mui/material";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import AutoAwesomeIcon from "@mui/icons-material/AutoAwesome";
import { useSessionTabsStore } from "../../stores/sessionTabs";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { genTabIndex } from "../../func/service";
import { APP_HEADER_RAIL_WIDTH } from "../../func/aiSidebar";
import { ShortcutAction } from "../../func/shortcuts";
import { useMemo, useCallback } from "react";
import { useFrameless } from "../../hooks/useFrameless";
import AppLogo from "../common/AppLogo";
import { RailMenuButton } from "../common/RailIconButton";
import BookmarkConnectPanel from "../bookmark/BookmarkConnectPanel";
import { useUIStore } from "../../stores/ui";
import { newConnectionTabName, t, useLanguage } from "../../i18n";

export default function Header() {
  const lang = useLanguage();
  const { sessionTabs, pushTab } = useSessionTabsStore();
  const frameless = useFrameless() === true;
  const bookmarkConnectOpen = useUIStore((s) => s.bookmarkConnectOpen);
  const setBookmarkConnectOpen = useUIStore((s) => s.setBookmarkConnectOpen);
  const toggleBookmarkConnect = useUIStore((s) => s.toggleBookmarkConnect);
  const toggleSidebarOpen = useAIAssistantStore((s) => s.toggleSidebarOpen);
  const handleAddTab = useCallback(() => {
    const number = sessionTabs.length + 1;
    pushTab({
      index: `${genTabIndex()}`,
      name: newConnectionTabName(number),
    });
  }, [sessionTabs.length, pushTab]);

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
        id: "bookmarks",
        title: t("header.bookmarks"),
        icon: <BookmarkIcon />,
        onClick: toggleBookmarkConnect,
        active: bookmarkConnectOpen,
        shortcutAction: ShortcutAction.PanelBookmarkConnect,
      },
      {
        id: "ai_assistant",
        title: t("header.ai_assistant"),
        icon: <AutoAwesomeIcon />,
        onClick: () => toggleSidebarOpen(),
        shortcutAction: ShortcutAction.PanelAI,
      },
    ];
  }, [
    lang,
    handleAddTab,
    toggleBookmarkConnect,
    bookmarkConnectOpen,
    toggleSidebarOpen,
  ]);

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
        </Stack>
      </Box>

      <BookmarkConnectPanel
        open={bookmarkConnectOpen}
        onRequestClose={() => setBookmarkConnectOpen(false)}
      />
    </Box>
  );
}
