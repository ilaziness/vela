import { Box, Menu, MenuItem, CircularProgress } from "@mui/material";
import React, { Suspense, useEffect, useRef, useCallback, useState } from "react";
import SSHTabBody from "./SSHTabBody.tsx";
import { Events } from "@wailsio/runtime";
import { useSSHTabsStore, useReloadSSHTabStore } from "../stores/ssh";
import { ConnectionStatus } from "../types/ssh";
import { useTransferStore } from "../stores/transfer.ts";
import { useAIAssistantStore } from "../stores/aiAssistant";
import OpBar from "./OpBar.tsx";
import { DraggableTab } from "./DraggableTab.tsx";
import { useSSHContextMenu } from "../hooks/useSSHContextMenu";
import { genTabIndex } from "../func/service";
import {
  ProgressData,
  TransferQueuedData,
} from "../../bindings/github.com/ilaziness/vela/services/models";
import {
  LogService,
  BookmarkService,
} from "../../bindings/github.com/ilaziness/vela/services/index.ts";
import { DragDropContext, Droppable, DropResult } from "@hello-pangea/dnd";
import { useMessageStore } from "../stores/message";
import { APP_HEADER_RAIL_WIDTH } from "../func/aiSidebar";
import { toggleWindowMaximise, useFrameless } from "../hooks/useFrameless";

const AISideBar = React.lazy(() => import("./ai/AISideBar"));

interface SSHTabsProps {
  onClose?: () => void;
}

// tab栏的高度
const tabHeight = "40px";

export default function SSHTabs({ onClose }: SSHTabsProps) {
  const sshTabs = useSSHTabsStore((state) => state.sshTabs);
  const currentTab = useSSHTabsStore((state) => state.currentTab);
  const delTab = useSSHTabsStore((state) => state.delTab);
  const pushTab = useSSHTabsStore((state) => state.pushTab);
  const getByIndex = useSSHTabsStore((state) => state.getByIndex);
  const setCurrentTab = useSSHTabsStore((state) => state.setCurrentTab);
  const reorderTabs = useSSHTabsStore((state) => state.reorderTabs);
  const doTabReload = useReloadSSHTabStore((state) => state.doTabReload);
  const addProgress = useTransferStore((state) => state.addProgress);
  const infoMessage = useMessageStore((state) => state.infoMessage);
  const { anchorEl, tabIndex, isOpen, openMenu, closeMenu } =
    useSSHContextMenu();
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const sidebarOpen = useAIAssistantStore((state) => state.sidebarOpen);
  const sidebarWidth = useAIAssistantStore((state) => state.sidebarWidth);
  const [aiSidebarMounted, setAiSidebarMounted] = useState(false);
  const frameless = useFrameless() === true;

  useEffect(() => {
    if (sidebarOpen) {
      setAiSidebarMounted(true);
    }
  }, [sidebarOpen]);

  React.useEffect(() => {
    const unsubscribeProgress = Events.On("eventProgress", (event: any) => {
      LogService.Debug(`SSHTabs eventProgress: ${JSON.stringify(event)}`);
      const eventData = event.data as ProgressData;
      addProgress(eventData);
    });

    const unsubscribeQueued = Events.On("eventTransferQueued", (event: any) => {
      const data = event.data as TransferQueuedData;
      if (!data?.sessionID || data.count < 1) return;
      // Emit 会广播到所有窗口；仅本窗口持有该会话时提示
      const ownsSession = useSSHTabsStore
        .getState()
        .sshTabs.some((t) => t.sshInfo?.linkID === data.sessionID);
      if (!ownsSession) return;
      infoMessage(
        data.count > 1
          ? `已添加 ${data.count} 项到传输列表`
          : "已添加到传输列表",
      );
    });

    const unsubscribeConnectBookmark = Events.On(
      "eventConnectBookmark",
      async (event: any) => {
        try {
          const bookmarkID = event.data as string;
          LogService.Debug(`Connecting to bookmark: ${bookmarkID}`);

          const bookmark =
            await BookmarkService.GetBookmarkByID(bookmarkID);
          if (!bookmark) {
            LogService.Warn(`Bookmark not found: ${bookmarkID}`);
            return;
          }

          const newTab = {
            index: genTabIndex(),
            name: bookmark.title,
            sshInfo: {
              bookmarkID: bookmarkID,
              host: bookmark.host,
              port: bookmark.port,
              user: bookmark.user,
              proxyJumpID: bookmark.proxy_jump_id,
            },
            connectionStatus: ConnectionStatus.Connecting,
          };

          pushTab(newTab);
          setCurrentTab(newTab.index);
        } catch (error) {
          LogService.Warn(`Failed to connect bookmark: ${error}`);
        }
      },
    );

    return () => {
      unsubscribeProgress();
      unsubscribeQueued();
      unsubscribeConnectBookmark();
    };
  }, [addProgress, infoMessage, pushTab, setCurrentTab]);

  const handleCloseTab = useCallback(() => {
    if (tabIndex === null) return;
    delTab(tabIndex, currentTab);
    closeMenu();
  }, [tabIndex, currentTab, delTab, closeMenu]);

  const handleDuplicateTab = useCallback(() => {
    closeMenu();
    const currentTabInfo = getByIndex(tabIndex || "");
    const newIndex = genTabIndex();
    pushTab({
      index: newIndex,
      name: currentTabInfo?.name || '',
      sshInfo: currentTabInfo?.sshInfo
        ? { ...currentTabInfo.sshInfo, linkID: undefined }
        : undefined,
      connectionStatus: currentTabInfo?.sshInfo
        ? ConnectionStatus.Connecting
        : undefined,
    });
    setCurrentTab(newIndex);
  }, [tabIndex, getByIndex, pushTab, setCurrentTab, closeMenu]);

  const handleRefreshTab = useCallback(() => {
    closeMenu();
    if (tabIndex) {
      doTabReload(tabIndex);
    }
  }, [tabIndex, doTabReload, closeMenu]);

  const handleDragEnd = useCallback(
    (result: DropResult) => {
      if (!result.destination) return;

      const sourceIndex = result.source.index;
      const destinationIndex = result.destination.index;

      if (sourceIndex === destinationIndex) return;

      reorderTabs(sourceIndex, destinationIndex);
    },
    [reorderTabs],
  );

  return (
    <Box
      sx={{
        height: "100%",
        width: `calc(100% - ${APP_HEADER_RAIL_WIDTH}px)`,
        display: "flex",
        flexDirection: "column",
        flex: 1,
      }}
    >
      {/* tabs */}
      <Box
        sx={{
          borderBottom: 1,
          borderColor: "divider",
          display: "flex",
          alignItems: "center",
          width: "100%",
          height: tabHeight,
          pr: 1,
          bgcolor: "background.paper",
        }}
      >
        <DragDropContext onDragEnd={handleDragEnd}>
          <Droppable droppableId="ssh-tabs" direction="horizontal">
            {(provided) => (
              <Box
                ref={(el: HTMLDivElement | null) => {
                  provided.innerRef(el);
                  scrollContainerRef.current = el;
                }}
                {...provided.droppableProps}
                sx={{
                  display: "flex",
                  flex: frameless ? "0 1 auto" : 1,
                  minWidth: 0,
                  height: tabHeight,
                  overflow: "auto",
                  scrollbarWidth: "none",
                  "&::-webkit-scrollbar": {
                    display: "none",
                  },
                }}
              >
                {sshTabs.map((tab, index) => (
                  <DraggableTab
                    key={tab.index}
                    item={tab}
                    index={index}
                    isActive={currentTab === tab.index}
                    onClick={() => setCurrentTab(tab.index)}
                    onClose={(e) => {
                      e.stopPropagation();
                      delTab(tab.index, currentTab);
                    }}
                    onContextMenu={(e) =>
                      openMenu(
                        e as unknown as React.MouseEvent<HTMLElement>,
                        tab.index,
                      )
                    }
                  />
                ))}
                {provided.placeholder}
              </Box>
            )}
          </Droppable>
        </DragDropContext>
        {frameless && (
          <Box
            onDoubleClick={() => {
              toggleWindowMaximise();
            }}
            sx={{
              flex: "1 0 0",
              alignSelf: "stretch",
              minWidth: 0,
              "--wails-draggable": "drag",
            }}
          />
        )}
        {frameless && (
          <OpBar draggable={false} showBorder={false} onClose={onClose} />
        )}
        <Menu
          open={isOpen}
          anchorEl={anchorEl}
          onClose={closeMenu}
          anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
          transformOrigin={{ vertical: "top", horizontal: "center" }}
        >
          <MenuItem onClick={handleCloseTab}>关闭</MenuItem>
          <MenuItem onClick={handleDuplicateTab}>复制</MenuItem>
          <MenuItem onClick={handleRefreshTab}>刷新</MenuItem>
        </Menu>
      </Box>

      {/* content: SSH body host + AI sidebar as flex siblings */}
      <Box
        sx={{
          width: "100%",
          height: `calc(100% - ${tabHeight})`,
          display: "flex",
          flexDirection: "row",
          overflow: "hidden",
        }}
      >
        <Box
          sx={{
            flex: 1,
            minWidth: 0,
            height: "100%",
            position: "relative",
          }}
        >
          {sshTabs.map((item) => (
            <Box
              role="tabpanel"
              key={item.index}
              sx={{
                display: "flex",
                width: "100%",
                height: "100%",
                position: "absolute",
                top: 0,
                left: currentTab === item.index ? 0 : "-9999rem",
              }}
            >
              <SSHTabBody
                tabIndex={item.index}
                isActive={currentTab === item.index}
              />
            </Box>
          ))}
        </Box>
        {aiSidebarMounted && (
          <Suspense
            fallback={
              <Box
                sx={{
                  width: sidebarOpen ? sidebarWidth : 0,
                  flexShrink: 0,
                  height: "100%",
                  overflow: "hidden",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: "background.paper",
                  borderLeft: sidebarOpen ? 1 : 0,
                  borderColor: "divider",
                }}
              >
                {sidebarOpen ? <CircularProgress size={24} /> : null}
              </Box>
            }
          >
            <AISideBar />
          </Suspense>
        )}
      </Box>
    </Box>
  );
}
