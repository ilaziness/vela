import { Box, IconButton, Stack, Tooltip } from "@mui/material";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import { useSSHTabsStore } from "../../stores/ssh";
import { genTabIndex } from "../../func/service";
import { APP_HEADER_RAIL_WIDTH } from "../../func/aiSidebar";
import { useState, useMemo, useCallback } from "react";
import { dragRegionSx, useFrameless } from "../../hooks/useFrameless";
import { railIconButtonSx } from "../railIconButtonSx";
import BookmarkConnectPanel from "../BookmarkConnectPanel";

export default function Header() {
  const { sshTabs, pushTab } = useSSHTabsStore();
  const frameless = useFrameless();
  const [bookmarkPanelOpen, setBookmarkPanelOpen] = useState(false);

  const handleAddTab = useCallback(() => {
    const number = sshTabs.length + 1;
    const newIndex = `${genTabIndex()}`;
    pushTab({
      index: newIndex,
      name: `新建连接 ${number}`,
    });
  }, [sshTabs.length, pushTab]);

  const handleToggleBookmarkPanel = useCallback(() => {
    setBookmarkPanelOpen((prev) => !prev);
  }, []);

  const menuItems = useMemo(
    () => [
      { title: "新建空白标签", icon: <AddBoxIcon />, onClick: handleAddTab },
      {
        title: "书签",
        icon: <BookmarkIcon />,
        onClick: handleToggleBookmarkPanel,
        active: bookmarkPanelOpen,
      },
    ],
    [handleAddTab, handleToggleBookmarkPanel, bookmarkPanelOpen],
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
        </Stack>
      </Box>

      <BookmarkConnectPanel
        open={bookmarkPanelOpen}
        onRequestClose={() => setBookmarkPanelOpen(false)}
      />
    </Box>
  );
}
