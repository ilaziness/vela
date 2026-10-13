import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  List,
  ListItem,
  ListItemText,
  ListItemButton,
  Collapse,
  IconButton,
  TextField,
  Typography,
  alpha,
  Tooltip,
  Chip,
  InputAdornment,
} from "@mui/material";
import {
  ExpandMore,
  ChevronRight,
  Edit as EditIcon,
  Add as AddIcon,
  Delete as DeleteIcon,
  ContentCopy as ContentCopyIcon,
  Search as SearchIcon,
  Clear as ClearIcon,
} from "@mui/icons-material";
import {
  SSHBookmark,
  BookmarkGroup,
} from "../../bindings/github.com/ilaziness/vela/services";
import {
  BookmarkIconView,
  GroupIconView,
  GroupIconPicker,
} from "./icons/bookmarkIcons";
import {
  bookmarkColorHex,
  filterBookmarkGroups,
  isBookmarkInFilteredGroups,
} from "../func/bookmarkMeta";

interface BookmarkTreeProps {
  bookmarks: BookmarkGroup[];
  tagSuggestions: string[];
  selectedBookmark: SSHBookmark | null;
  onBookmarkSelect: (bookmark: SSHBookmark) => void;
  onGroupRename: (oldGroupName: string, newGroupName: string) => void;
  onGroupIconChange: (groupName: string, icon: string) => void;
  onGroupAdd: (groupName: string) => void;
  onGroupDelete: (groupName: string) => void;
  onBookmarkAdd: (groupName: string) => void;
  onBookmarkCopy: (bookmarkId: string) => void;
  onBookmarkDelete: (bookmarkId: string) => void;
}

interface GroupState {
  [key: string]: boolean;
}

const BookmarkTree: React.FC<BookmarkTreeProps> = ({
  bookmarks,
  tagSuggestions,
  selectedBookmark,
  onBookmarkSelect,
  onGroupRename,
  onGroupIconChange,
  onGroupAdd,
  onGroupDelete,
  onBookmarkAdd,
  onBookmarkCopy,
  onBookmarkDelete,
}) => {
  const [expandedGroups, setExpandedGroups] = useState<GroupState>({});
  const [editingGroup, setEditingGroup] = useState<string | null>(null);
  const [newGroupName, setNewGroupName] = useState<string>("");
  const [newGroupInput, setNewGroupInput] = useState<boolean>(false);
  const [newGroupNameInput, setNewGroupNameInput] = useState<string>("");
  const [hoveredGroup, setHoveredGroup] = useState<string | null>(null);
  const [hoveredBookmark, setHoveredBookmark] = useState<string | null>(null);
  const [groupIconAnchor, setGroupIconAnchor] = useState<{
    el: HTMLElement;
    name: string;
    icon: string;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedTag, setSelectedTag] = useState("");

  useEffect(() => {
    if (!selectedTag) {
      return;
    }
    const stillExists = tagSuggestions.some(
      (t) => t.toLowerCase() === selectedTag.toLowerCase(),
    );
    if (!stillExists) {
      setSelectedTag("");
    }
  }, [tagSuggestions, selectedTag]);

  const filtering = Boolean(searchQuery.trim() || selectedTag);
  const visibleGroups = useMemo(
    () => filterBookmarkGroups(bookmarks, searchQuery, selectedTag),
    [bookmarks, searchQuery, selectedTag],
  );

  const selectedOutsideFilter =
    filtering &&
    Boolean(selectedBookmark?.id) &&
    !isBookmarkInFilteredGroups(visibleGroups, selectedBookmark?.id);

  const matchingGroupNames = useMemo(
    () => (filtering ? visibleGroups.map((g) => g.name).join("\0") : ""),
    [filtering, visibleGroups],
  );

  useEffect(() => {
    if (!matchingGroupNames) {
      return;
    }
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

  const startEditingGroup = (groupName: string, currentName: string) => {
    setEditingGroup(groupName);
    setNewGroupName(currentName);
  };

  const finishEditingGroup = (groupName: string) => {
    if (newGroupName.trim() && newGroupName !== groupName) {
      onGroupRename(groupName, newGroupName.trim());
    }
    setEditingGroup(null);
  };

  const handleGroupRenameKeyDown = (
    e: React.KeyboardEvent,
    groupName: string,
  ) => {
    if (e.key === "Enter") {
      finishEditingGroup(groupName);
    } else if (e.key === "Escape") {
      setEditingGroup(null);
    }
  };

  const handleAddNewGroup = () => {
    if (newGroupNameInput.trim()) {
      onGroupAdd(newGroupNameInput.trim());
    }
    setNewGroupInput(false);
    setNewGroupNameInput("");
  };

  const handleNewGroupKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleAddNewGroup();
    } else if (e.key === "Escape") {
      setNewGroupInput(false);
      setNewGroupNameInput("");
    }
  };

  const clearFilters = () => {
    setSearchQuery("");
    setSelectedTag("");
  };

  const isGroupExpanded = (name: string) => Boolean(expandedGroups[name]);

  return (
    <Box sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
      <Box
        sx={{
          p: 2,
          pb: 1.5,
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        <Typography variant="h6" sx={{ fontWeight: 600, fontSize: "1.1rem" }}>
          书签
        </Typography>
        <TextField
          size="small"
          fullWidth
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="名称 / 主机 / 用户 / 标签"
          sx={{ mt: 1.5 }}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" color="action" />
                </InputAdornment>
              ),
              endAdornment: searchQuery ? (
                <InputAdornment position="end">
                  <IconButton
                    size="small"
                    onClick={() => setSearchQuery("")}
                    edge="end"
                  >
                    <ClearIcon fontSize="small" />
                  </IconButton>
                </InputAdornment>
              ) : null,
            },
          }}
        />
        {tagSuggestions.length > 0 ? (
          <Box
            sx={{
              mt: 1,
              display: "flex",
              flexWrap: "wrap",
              gap: 0.5,
              maxHeight: 72,
              overflowY: "auto",
            }}
          >
            {tagSuggestions.map((tag) => {
              const active = selectedTag.toLowerCase() === tag.toLowerCase();
              return (
                <Chip
                  key={tag}
                  label={tag}
                  size="small"
                  color={active ? "primary" : "default"}
                  variant={active ? "filled" : "outlined"}
                  onClick={() =>
                    setSelectedTag(active ? "" : tag)
                  }
                />
              );
            })}
          </Box>
        ) : null}
        {selectedOutsideFilter ? (
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ display: "block", mt: 1 }}
          >
            当前书签不在筛选结果中
          </Typography>
        ) : null}
      </Box>

      <Box sx={{ flex: 1, overflowY: "auto" }}>
        <List sx={{ py: 1 }}>
          {filtering && visibleGroups.length === 0 ? (
            <Box sx={{ px: 2, py: 3, textAlign: "center" }}>
              <Typography variant="body2" color="text.secondary">
                无匹配书签
              </Typography>
              <Chip
                label="清除筛选"
                size="small"
                onClick={clearFilters}
                sx={{ mt: 1 }}
              />
            </Box>
          ) : null}

          {visibleGroups.map((group) => (
            <React.Fragment key={group.name}>
              <ListItem
                disablePadding
                onMouseEnter={() => setHoveredGroup(group.name)}
                onMouseLeave={() => setHoveredGroup(null)}
                sx={{
                  "&:hover": {
                    backgroundColor: (theme) =>
                      alpha(theme.palette.primary.main, 0.04),
                  },
                }}
              >
                <ListItemButton
                  onClick={() => toggleGroup(group.name)}
                  sx={{ py: 0.75 }}
                >
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      width: "100%",
                      gap: 0.5,
                    }}
                  >
                    {isGroupExpanded(group.name) ? (
                      <ExpandMore sx={{ fontSize: 20, color: "text.secondary" }} />
                    ) : (
                      <ChevronRight
                        sx={{ fontSize: 20, color: "text.secondary" }}
                      />
                    )}
                    <Box
                      sx={{
                        display: "flex",
                        mr: 0.5,
                        cursor: "pointer",
                      }}
                      onClick={(e) => {
                        e.stopPropagation();
                        setGroupIconAnchor({
                          el: e.currentTarget,
                          name: group.name,
                          icon: group.icon || "",
                        });
                      }}
                    >
                      <GroupIconView icon={group.icon} fontSize={18} />
                    </Box>
                    {editingGroup === group.name ? (
                      <TextField
                        value={newGroupName}
                        onChange={(e) => setNewGroupName(e.target.value)}
                        onKeyDown={(e) =>
                          handleGroupRenameKeyDown(e, group.name)
                        }
                        onBlur={() => finishEditingGroup(group.name)}
                        onClick={(e) => e.stopPropagation()}
                        autoFocus
                        size="small"
                        sx={{
                          flex: 1,
                          "& .MuiInputBase-root": {
                            fontSize: "0.95rem",
                            fontWeight: 500,
                          },
                        }}
                      />
                    ) : (
                      <ListItemText
                        primary={group.name}
                        slotProps={{
                          primary: {
                            sx: {
                              fontSize: "0.95rem",
                              fontWeight: 500,
                            },
                          },
                        }}
                        sx={{ flex: 1, minWidth: 0 }}
                      />
                    )}
                    <Box
                      sx={{
                        display: "flex",
                        gap: 0.25,
                        visibility:
                          hoveredGroup === group.name ? "visible" : "hidden",
                      }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Tooltip title="重命名">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            startEditingGroup(group.name, group.name);
                          }}
                          sx={{ padding: "4px" }}
                        >
                          <EditIcon sx={{ fontSize: 16 }} />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="删除分组">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            onGroupDelete(group.name);
                          }}
                          sx={{ padding: "4px" }}
                        >
                          <DeleteIcon sx={{ fontSize: 16 }} />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </Box>
                </ListItemButton>
              </ListItem>

              <Collapse
                in={isGroupExpanded(group.name)}
                timeout="auto"
                unmountOnExit
              >
                <List component="div" disablePadding>
                  {(group.bookmarks || []).map((bookmark) => {
                    if (!bookmark) {
                      return null;
                    }
                    const colorHex = bookmarkColorHex(bookmark.color);
                    const tagHint = (bookmark.tags || []).filter(Boolean).join(", ");
                    return (
                      <ListItem
                        key={bookmark.id}
                        disablePadding
                        onMouseEnter={() => setHoveredBookmark(bookmark.id)}
                        onMouseLeave={() => setHoveredBookmark(null)}
                      >
                        <ListItemButton
                          selected={selectedBookmark?.id === bookmark.id}
                          onClick={() => onBookmarkSelect(bookmark)}
                          title={tagHint || undefined}
                          sx={{
                            pl: 2,
                            py: 0.5,
                            borderLeft: 5,
                            borderColor: colorHex || "transparent",
                            "&.Mui-selected": {
                              backgroundColor: (theme) =>
                                alpha(theme.palette.primary.main, 0.12),
                              "&:hover": {
                                backgroundColor: (theme) =>
                                  alpha(theme.palette.primary.main, 0.2),
                              },
                            },
                          }}
                        >
                          <Box
                            sx={{
                              mr: 1.5,
                              display: "flex",
                              flexShrink: 0,
                              ml: 2,
                            }}
                          >
                            <BookmarkIconView
                              icon={bookmark.icon}
                              fontSize={16}
                            />
                          </Box>
                          <ListItemText
                            primary={bookmark.title}
                            secondary={`${bookmark.user}@${bookmark.host}`}
                            slotProps={{
                              primary: {
                                sx: {
                                  fontSize: "0.9rem",
                                  whiteSpace: "nowrap",
                                  overflow: "hidden",
                                  textOverflow: "ellipsis",
                                },
                              },
                              secondary: {
                                sx: {
                                  fontSize: "0.75rem",
                                  whiteSpace: "nowrap",
                                  overflow: "hidden",
                                  textOverflow: "ellipsis",
                                },
                              },
                            }}
                            sx={{ flex: 1, minWidth: 0 }}
                          />
                          <Box
                            sx={{
                              display: "flex",
                              gap: 0.25,
                              visibility:
                                hoveredBookmark === bookmark.id
                                  ? "visible"
                                  : "hidden",
                            }}
                          >
                            <Tooltip title="复制书签">
                              <IconButton
                                size="small"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onBookmarkCopy(bookmark.id);
                                }}
                                sx={{ padding: "4px" }}
                              >
                                <ContentCopyIcon sx={{ fontSize: 16 }} />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="删除书签">
                              <IconButton
                                size="small"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onBookmarkDelete(bookmark.id);
                                }}
                                sx={{ padding: "4px" }}
                              >
                                <DeleteIcon sx={{ fontSize: 16 }} />
                              </IconButton>
                            </Tooltip>
                          </Box>
                        </ListItemButton>
                      </ListItem>
                    );
                  })}
                  {!filtering ? (
                    <ListItemButton
                      sx={{
                        pl: 5,
                        py: 0.5,
                        color: "primary.main",
                      }}
                      onClick={() => onBookmarkAdd(group.name)}
                    >
                      <AddIcon sx={{ fontSize: 16, mr: 1.5 }} />
                      <ListItemText
                        primary="添加书签"
                        slotProps={{
                          primary: {
                            sx: {
                              fontSize: "0.9rem",
                            },
                          },
                        }}
                      />
                    </ListItemButton>
                  ) : null}
                </List>
              </Collapse>
            </React.Fragment>
          ))}

          {!filtering ? (
            newGroupInput ? (
              <ListItem sx={{ px: 1.5, py: 0.5 }}>
                <TextField
                  value={newGroupNameInput}
                  onChange={(e) => setNewGroupNameInput(e.target.value)}
                  onKeyDown={handleNewGroupKeyDown}
                  onBlur={handleAddNewGroup}
                  autoFocus
                  size="small"
                  placeholder="分组名称"
                  fullWidth
                  sx={{
                    "& .MuiInputBase-root": {
                      fontSize: "0.9rem",
                    },
                  }}
                />
              </ListItem>
            ) : (
              <ListItemButton
                onClick={() => setNewGroupInput(true)}
                sx={{
                  py: 0.75,
                  px: 1.5,
                  color: "primary.main",
                }}
              >
                <AddIcon sx={{ fontSize: 20, mr: 1 }} />
                <ListItemText
                  primary="添加分组"
                  slotProps={{
                    primary: {
                      sx: {
                        fontSize: "0.95rem",
                        fontWeight: 500,
                      },
                    },
                  }}
                />
              </ListItemButton>
            )
          ) : null}
        </List>
      </Box>

      <GroupIconPicker
        value={groupIconAnchor?.icon || ""}
        onChange={(icon) => {
          if (groupIconAnchor) {
            onGroupIconChange(groupIconAnchor.name, icon);
          }
        }}
        anchorEl={groupIconAnchor?.el || null}
        open={Boolean(groupIconAnchor)}
        onClose={() => setGroupIconAnchor(null)}
      />
    </Box>
  );
};

export default BookmarkTree;
