import { Box, Menu, MenuItem, CircularProgress } from "@mui/material";
import React, { Suspense, useEffect, useRef, useCallback, useState } from "react";
import SessionTabBody from "./SessionTabBody.tsx";
import { Events } from "@wailsio/runtime";
import {
  useSessionTabsStore,
  useReloadSessionTabStore,
} from "../../stores/sessionTabs";
import { ConnectionStatus } from "../../types/ssh";
import { parseSessionProtocol } from "../../types/session";
import { useTransferStore } from "../../stores/transfer.ts";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import OpBar from "../common/OpBar.tsx";
import { DraggableTab } from "./DraggableTab.tsx";
import { useSSHContextMenu } from "../../hooks/useSSHContextMenu";
import { genTabIndex, translateServiceError } from "../../func/service";
import { t, tf } from "../../i18n";
import {
  ProgressData,
  TransferQueuedData,
} from "../../../bindings/github.com/ilaziness/vela/services/models";
import {
  LogService,
  BookmarkService,
} from "../../../bindings/github.com/ilaziness/vela/services/index.ts";
import { DragDropContext, Droppable, DropResult } from "@hello-pangea/dnd";
import { useMessageStore } from "../../stores/message";
import { toggleWindowMaximise, useFrameless } from "../../hooks/useFrameless";

const AISideBar = React.lazy(() => import("../ai/AISideBar"));

interface SessionTabsProps {
  onClose?: () => void;
}

export default function SessionTabs({ onClose }: SessionTabsProps) {
  const tabs = useSessionTabsStore((state) => state.sessionTabs);
  const currentTab = useSessionTabsStore((state) => state.currentTab);
  const delTab = useSessionTabsStore((state) => state.delTab);
  const pushTab = useSessionTabsStore((state) => state.pushTab);
  const getByIndex = useSessionTabsStore((state) => state.getByIndex);
  const setCurrentTab = useSessionTabsStore((state) => state.setCurrentTab);
  const reorderTabs = useSessionTabsStore((state) => state.reorderTabs);
  const doTabReload = useReloadSessionTabStore((state) => state.doTabReload);
  const addProgress = useTransferStore((state) => state.addProgress);
  const infoMessage = useMessageStore((state) => state.infoMessage);
  const errorMessage = useMessageStore((state) => state.errorMessage);
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
      LogService.Debug(`tabs eventProgress: ${JSON.stringify(event)}`);
      const eventData = event.data as ProgressData;
      addProgress(eventData);
    });

    const unsubscribeQueued = Events.On("eventTransferQueued", (event: any) => {
      const data = event.data as TransferQueuedData;
      if (!data?.sessionID || data.count < 1) return;
      // Emit 会广播到所有窗口；仅本窗口持有该会话时提示
      const ownsSession = useSessionTabsStore
        .getState()
        .sessionTabs.some((t) => t.sessionLink?.linkID === data.sessionID);
      if (!ownsSession) return;
      infoMessage(
        data.count > 1
          ? tf("transfer.added_n", data.count)
          : t("transfer.added"),
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
            errorMessage(t("bookmark.not_found"));
            return;
          }

          const protocol = parseSessionProtocol(bookmark.protocol);
          if (!protocol) {
            errorMessage(
              tf("session.unknown_protocol", bookmark.protocol || "?"),
            );
            return;
          }

          const newTab = {
            index: genTabIndex(),
            name: bookmark.title,
            sessionLink: {
              protocol,
              bookmarkID: bookmarkID,
              host: bookmark.host,
              port: bookmark.port,
              user: bookmark.user,
              proxyJumpID: bookmark.proxy_jump_id,
              serialDevice: bookmark.serial_device,
              baudRate: bookmark.baud_rate,
              dataBits: bookmark.data_bits,
              parity: bookmark.parity,
              stopBits: bookmark.stop_bits,
              flowControl: bookmark.flow_control,
              charset: bookmark.charset,
            },
            connectionStatus: ConnectionStatus.Connecting,
          };

          pushTab(newTab);
          setCurrentTab(newTab.index);
        } catch (error) {
          errorMessage(translateServiceError(error));
        }
      },
    );

    return () => {
      unsubscribeProgress();
      unsubscribeQueued();
      unsubscribeConnectBookmark();
    };
  }, [addProgress, infoMessage, errorMessage, pushTab, setCurrentTab]);

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
      sessionLink: currentTabInfo?.sessionLink
        ? { ...currentTabInfo.sessionLink, linkID: undefined }
        : undefined,
      connectionStatus: currentTabInfo?.sessionLink
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
        flex: 1,
        minWidth: 0,
        display: "flex",
        flexDirection: "column",
      }}
    >
      {/* tabs */}
      <Box
        sx={{
          borderBottom: 1,
          borderColor: "divider",
          display: "flex",
          flexShrink: 0,
          width: "100%",
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
                  overflow: "auto",
                  scrollbarWidth: "none",
                  "&::-webkit-scrollbar": {
                    display: "none",
                  },
                }}
              >
                {tabs.map((tab, index) => (
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
          <MenuItem onClick={handleCloseTab}>{t("common.close")}</MenuItem>
          <MenuItem onClick={handleDuplicateTab}>{t("common.copy")}</MenuItem>
          <MenuItem onClick={handleRefreshTab}>{t("common.refresh")}</MenuItem>
        </Menu>
      </Box>

      {/* content: SSH body host + AI sidebar as flex siblings */}
      <Box
        sx={{
          width: "100%",
          flex: 1,
          minHeight: 0,
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
          {tabs.map((item) => (
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
              <SessionTabBody
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
