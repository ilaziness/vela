import { Box, Stack } from "@mui/material";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import { useSSHTabsStore } from "../../stores/ssh";
import { genTabIndex } from "../../func/service";
import { APP_HEADER_RAIL_WIDTH } from "../../func/aiSidebar";
import { ShortcutAction } from "../../func/shortcuts";
import { useMemo, useCallback } from "react";
import { dragRegionSx, useFrameless } from "../../hooks/useFrameless";
import { RailMenuButton } from "../RailIconButton";
import BookmarkConnectPanel from "../BookmarkConnectPanel";
import { useUIStore } from "../../stores/ui";

export default function Header() {
  const { sshTabs, pushTab } = useSSHTabsStore();
  const frameless = useFrameless();
  const bookmarkConnectOpen = useUIStore((s) => s.bookmarkConnectOpen);
  const setBookmarkConnectOpen = useUIStore((s) => s.setBookmarkConnectOpen);
  const toggleBookmarkConnect = useUIStore((s) => s.toggleBookmarkConnect);

  const handleAddTab = useCallback(() => {
    const number = sshTabs.length + 1;
    pushTab({
      index: `${genTabIndex()}`,
      name: `新建连接 ${number}`,
    });
  }, [sshTabs.length, pushTab]);

  const menuItems = useMemo(
    () => [
      { title: "新建空白标签", icon: <AddBoxIcon />, onClick: handleAddTab },
      {
        title: "书签",
        icon: <BookmarkIcon />,
        onClick: toggleBookmarkConnect,
        active: bookmarkConnectOpen,
        shortcutAction: ShortcutAction.PanelBookmarkConnect,
      },
    ],
    [handleAddTab, toggleBookmarkConnect, bookmarkConnectOpen],
  );

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
          <Box
            sx={{
              ...dragRegionSx(frameless),
              cursor: frameless ? "move" : undefined,
            }}
          >
            <img
              src="/appicon.png"
              alt="Logo"
              style={{ width: "24px", height: "24px" }}
            />
          </Box>

          {menuItems.map((item) => (
            <RailMenuButton
              key={item.title}
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
