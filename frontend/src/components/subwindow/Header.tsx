import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Menu,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Collapse,
} from "@mui/material";
import AddBoxIcon from "@mui/icons-material/AddBox";
import BookmarkIcon from "@mui/icons-material/Bookmark";
import ChevronRightIcon from "@mui/icons-material/ChevronRight";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";

import {
  BookmarkService,
  BookmarkGroup,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { useSSHTabsStore } from "../../stores/ssh";
import { ConnectionStatus } from "../../types/ssh";
import { genTabIndex, parseCallServiceError } from "../../func/service";
import { bookmarkColorHex } from "../../func/bookmarkMeta";
import React, { useState, useEffect, useMemo, useCallback } from "react";
import { useMessageStore } from "../../stores/message";
import { Events } from "@wailsio/runtime";
import { dragRegionSx, useFrameless } from "../../hooks/useFrameless";
import {
  BookmarkIconView,
  GroupIconView,
} from "../icons/bookmarkIcons";

export default function Header() {
  const { sshTabs, pushTab, setCurrentTab } = useSSHTabsStore();
  const { errorMessage } = useMessageStore();
  const frameless = useFrameless();

  const [bookmarks, setBookmarks] = useState<BookmarkGroup[]>([]);
  const [bookmarkAnchorEl, setBookmarkAnchorEl] = useState<null | HTMLElement>(
    null,
  );
  const [expandedGroups, setExpandedGroups] = useState<{
    [key: string]: boolean;
  }>({});

  const handleAddTab = useCallback(() => {
    const number = sshTabs.length + 1;
    const newIndex = `${genTabIndex()}`;
    pushTab({
      index: newIndex,
      name: `新建连接 ${number}`,
    });
  }, [sshTabs.length, pushTab]);

  const handleBookmark = useCallback((event: React.MouseEvent<HTMLElement>) => {
    setBookmarkAnchorEl(event.currentTarget);
  }, []);

  const menuItems = useMemo(
    () => [
      { title: "新建空白标签", icon: <AddBoxIcon />, onClick: handleAddTab },
      { title: "书签", icon: <BookmarkIcon />, onClick: handleBookmark },
    ],
    [handleAddTab, handleBookmark],
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
      console.error("Failed to load bookmarks:", error);
      errorMessage(`Failed to load bookmarks:${error}`);
    }
  }, [errorMessage]);

  useEffect(() => {
    void loadBookmarks();
    const unsubscribe = Events.On("eventBookmarkUpdate", () => {
      void loadBookmarks();
    });

    return () => {
      unsubscribe();
    };
  }, [loadBookmarks]);

  const closeBookmarkMenu = () => {
    setBookmarkAnchorEl(null);
    setExpandedGroups({});
  };

  const toggleGroup = (groupName: string) => {
    setExpandedGroups((prev) => ({
      ...prev,
      [groupName]: !prev[groupName],
    }));
  };

  const handleBookmarkSelect = async (bookmarkID: string) => {
    closeBookmarkMenu();
    try {
      const bookmark = await BookmarkService.GetBookmarkByID(bookmarkID);
      if (!bookmark) {
        errorMessage("bookmark not found");
        return;
      }
      const sshInfo = {
        bookmarkID: bookmark.id,
        host: bookmark.host,
        port: bookmark.port,
        user: bookmark.user,
        proxyJumpID: bookmark.proxy_jump_id,
      };
      const newIndex = genTabIndex();
      pushTab({
        index: newIndex,
        name: bookmark.title,
        sshInfo,
        connectionStatus: ConnectionStatus.Connecting,
      });
      setCurrentTab(newIndex);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  return (
    <Box
      component={"header"}
      sx={(theme) => ({
        width: "42px",
        overflow: "hidden",
        backgroundColor: theme.palette.background.paper,
        borderRight: `1px solid ${theme.palette.divider}`,
      })}
    >
      <Stack direction="column" spacing={1} sx={{ alignItems: "center", padding: 0.5 }}>
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
            <IconButton size="small" onClick={item.onClick}>
              {item.icon}
            </IconButton>
          </Tooltip>
        ))}

        <Menu
          anchorEl={bookmarkAnchorEl}
          open={Boolean(bookmarkAnchorEl)}
          onClose={closeBookmarkMenu}
          slotProps={{
            paper: {
              elevation: 8,
              sx: {
                overflow: "visible",
                mt: 1,
                minWidth: 200,
              },
            },
          }}
        >
          <List dense>
            {bookmarks.map((group) => (
              <div key={group.name}>
                <ListItem disablePadding>
                  <ListItemButton onClick={() => toggleGroup(group.name)}>
                    <ListItemIcon sx={{ minWidth: 32 }}>
                      {expandedGroups[group.name] ? (
                        <ExpandMoreIcon fontSize="small" />
                      ) : (
                        <ChevronRightIcon fontSize="small" />
                      )}
                    </ListItemIcon>
                    <ListItemIcon sx={{ minWidth: 28 }}>
                      <GroupIconView icon={group.icon} fontSize={18} />
                    </ListItemIcon>
                    <ListItemText primary={group.name} />
                  </ListItemButton>
                </ListItem>
                <Collapse
                  in={expandedGroups[group.name]}
                  timeout="auto"
                  unmountOnExit
                >
                  <List component="div" disablePadding dense>
                    {(group.bookmarks || []).map((bookmark) =>
                      bookmark ? (
                        <ListItem key={bookmark.id} disablePadding>
                          <ListItemButton
                            sx={{
                              pl: 4,
                              borderLeft: 5,
                              borderColor:
                                bookmarkColorHex(bookmark.color) || "transparent",
                            }}
                            onClick={() => handleBookmarkSelect(bookmark.id)}
                          >
                            <ListItemIcon sx={{ minWidth: 24 }}>
                              <BookmarkIconView
                                icon={bookmark.icon}
                                fontSize={16}
                              />
                            </ListItemIcon>
                            <ListItemText primary={bookmark.title} />
                          </ListItemButton>
                        </ListItem>
                      ) : null,
                    )}
                  </List>
                </Collapse>
              </div>
            ))}
          </List>
        </Menu>
      </Stack>
    </Box>
  );
}
