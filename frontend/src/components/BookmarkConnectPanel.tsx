import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import {
  Box,
  Collapse,
  IconButton,
  InputAdornment,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  TextField,
  Typography,
} from "@mui/material";
import ChevronRightIcon from "@mui/icons-material/ChevronRight";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import SearchIcon from "@mui/icons-material/Search";
import ClearIcon from "@mui/icons-material/Clear";
import {
  BookmarkGroup,
  BookmarkService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { Events } from "@wailsio/runtime";
import { useSSHTabsStore } from "../stores/ssh";
import { ConnectionStatus } from "../types/ssh";
import { genTabIndex, parseCallServiceError } from "../func/service";
import {
  bookmarkColorHex,
  filterBookmarkGroups,
} from "../func/bookmarkMeta";
import { useMessageStore } from "../stores/message";
import { BookmarkIconView, GroupIconView } from "./icons/bookmarkIcons";

const PANEL_WIDTH = 220;
const PANEL_WIDTH_TRANSITION = "width 225ms ease";
const PANEL_WIDTH_TRANSITION_MS = 225;

interface BookmarkConnectPanelProps {
  open: boolean;
  onRequestClose?: () => void;
}

export default function BookmarkConnectPanel({
  open,
  onRequestClose,
}: BookmarkConnectPanelProps) {
  const pushTab = useSSHTabsStore((s) => s.pushTab);
  const setCurrentTab = useSSHTabsStore((s) => s.setCurrentTab);
  const errorMessage = useMessageStore((s) => s.errorMessage);

  const [bookmarks, setBookmarks] = useState<BookmarkGroup[]>([]);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(
    {},
  );
  const [searchQuery, setSearchQuery] = useState("");
  const searchInputRef = useRef<HTMLInputElement>(null);

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
      errorMessage("加载书签失败: " + parseCallServiceError(error));
    }
  }, [errorMessage]);

  // 打开时加载并订阅更新；关闭时退订（下次打开会重新拉取）
  useEffect(() => {
    if (!open) return;
    void loadBookmarks();
    const unsubscribe = Events.On("eventBookmarkUpdate", () => {
      void loadBookmarks();
    });
    const focusTimer = window.setTimeout(() => {
      searchInputRef.current?.focus();
    }, PANEL_WIDTH_TRANSITION_MS);
    return () => {
      unsubscribe();
      window.clearTimeout(focusTimer);
    };
  }, [open, loadBookmarks]);

  const filtering = Boolean(searchQuery.trim());
  const visibleGroups = useMemo(
    () => filterBookmarkGroups(bookmarks, searchQuery, ""),
    [bookmarks, searchQuery],
  );

  const matchingGroupNames = useMemo(
    () => (filtering ? visibleGroups.map((g) => g.name).join("\0") : ""),
    [filtering, visibleGroups],
  );

  useEffect(() => {
    if (!matchingGroupNames) return;
    const names = matchingGroupNames.split("\0").filter(Boolean);
    setExpandedGroups((prev) => {
      const next = { ...prev };
      for (const name of names) {
        next[name] = true;
      }
      return next;
    });
  }, [matchingGroupNames]);

  const toggleGroup = (groupName: string) => {
    setExpandedGroups((prev) => ({
      ...prev,
      [groupName]: !prev[groupName],
    }));
  };

  const handleBookmarkSelect = async (bookmarkID: string) => {
    try {
      const bookmark = await BookmarkService.GetBookmarkByID(bookmarkID);
      if (!bookmark) {
        errorMessage("书签不存在");
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

  const visibleWidth = open ? PANEL_WIDTH : 0;

  const handlePanelKeyDown = (e: KeyboardEvent) => {
    if (e.key !== "Escape" || !onRequestClose) return;
    e.preventDefault();
    e.stopPropagation();
    onRequestClose();
  };

  return (
    <Box
      aria-hidden={!open}
      inert={!open || undefined}
      onKeyDown={handlePanelKeyDown}
      sx={{
        width: visibleWidth,
        flexShrink: 0,
        height: "100%",
        overflow: "hidden",
        transition: PANEL_WIDTH_TRANSITION,
        display: "flex",
        bgcolor: "background.paper",
        borderRight: open ? 1 : 0,
        borderColor: "divider",
        pointerEvents: open ? "auto" : "none",
      }}
    >
      <Box
        sx={{
          width: PANEL_WIDTH,
          minWidth: PANEL_WIDTH,
          height: "100%",
          display: "flex",
          flexDirection: "column",
          flexShrink: 0,
        }}
      >
        <Box
          sx={{
            px: 1,
            pt: 1,
            pb: 0.75,
            borderBottom: 1,
            borderColor: "divider",
          }}
        >
          <TextField
            size="small"
            fullWidth
            inputRef={searchInputRef}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="搜索书签"
            sx={{
              "& .MuiInputBase-root": {
                height: 28,
                fontSize: "0.75rem",
              },
              "& .MuiInputBase-input": {
                py: 0,
                px: 0.5,
              },
              "& .MuiInputAdornment-root": {
                ml: 0.25,
                mr: 0,
              },
            }}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon sx={{ fontSize: 14 }} color="action" />
                  </InputAdornment>
                ),
                endAdornment: searchQuery ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() => setSearchQuery("")}
                      edge="end"
                      aria-label="清除搜索"
                      sx={{ p: 0.25 }}
                    >
                      <ClearIcon sx={{ fontSize: 14 }} />
                    </IconButton>
                  </InputAdornment>
                ) : null,
              },
            }}
          />
        </Box>

        <Box sx={{ flex: 1, overflowY: "auto" }}>
          <List dense sx={{ py: 0.5 }}>
            {filtering && visibleGroups.length === 0 ? (
              <Box sx={{ px: 2, py: 3, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  无匹配书签
                </Typography>
              </Box>
            ) : null}

            {visibleGroups.map((group) => (
              <Box key={group.name}>
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
                  in={Boolean(expandedGroups[group.name])}
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
                                bookmarkColorHex(bookmark.color) ||
                                "transparent",
                            }}
                            onClick={() =>
                              void handleBookmarkSelect(bookmark.id)
                            }
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
              </Box>
            ))}
          </List>
        </Box>
      </Box>
    </Box>
  );
}
