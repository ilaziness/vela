import React, { useState, useEffect, useMemo, useCallback } from "react";
import { Slide, Box, Avatar, CircularProgress, Button, Typography } from "@mui/material";
import { ChatBox, ChatMessageAvatar } from "@mui/x-chat";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { LogService } from "../../../bindings/github.com/ilaziness/vela/services/index";
import { GenkitAdapter, aiUser, currentUser } from "./GenkitAdapter";
import { aiPartRenderers } from "./aiPartRenderers";
import SessionPanel from "./SessionPanel";
import {
  clampSidebarWidth,
  persistSidebarWidth,
  SIDEBAR_RESIZE_HANDLE_WIDTH,
  SIDEBAR_WIDTH,
  SSH_STATUS_BAR_HEIGHT,
} from "../../func/aiSidebar";

const LetterAvatar = React.forwardRef<HTMLDivElement, any>(
  function LetterAvatar(props, ref) {
    const { ownerState, ...other } = props;
    const isUser = ownerState?.role === "user";
    return (
      <Avatar
        ref={ref}
        {...other}
        sx={{
          width: 32,
          height: 32,
          fontSize: isUser ? 14 : 11,
          fontWeight: 600,
          bgcolor: isUser ? "primary.main" : "secondary.main",
        }}
      >
        {isUser ? "Y" : "AI"}
      </Avatar>
    );
  }
);

const CustomMessageAvatar = React.forwardRef<HTMLDivElement, any>(
  function CustomMessageAvatar(props, ref) {
    return (
      <ChatMessageAvatar
        ref={ref}
        {...props}
        slots={{ ...props.slots, avatar: LetterAvatar }}
      />
    );
  }
);

const AISideBar = () => {
  const sidebarOpen = useAIAssistantStore((state) => state.sidebarOpen);
  const sidebarWidth = useAIAssistantStore((state) => state.sidebarWidth);
  const activeSessionId = useAIAssistantStore((state) => state.activeSessionId);
  const loadingSessions = useAIAssistantStore((state) => state.loadingSessions);
  const composerValue = useAIAssistantStore((state) => state.composerValue);
  const loadSessions = useAIAssistantStore((state) => state.loadSessions);
  const refreshActiveSessionTitle = useAIAssistantStore((state) => state.refreshActiveSessionTitle);
  const setSidebarWidth = useAIAssistantStore((state) => state.setSidebarWidth);
  const setComposerValue = useAIAssistantStore((state) => state.setComposerValue);

  const adapter = useMemo(() => new GenkitAdapter(), []);

  const [chatContainer, setChatContainer] = useState<HTMLDivElement | null>(null);

  const setChatRef = useCallback((node: HTMLDivElement | null) => {
    setChatContainer(node);
  }, []);

  const [isResizing, setIsResizing] = useState(false);
  const [isHovering, setIsHovering] = useState(false);

  useEffect(() => {
    if (!sidebarOpen) {
      useAIAssistantStore.getState().setHistoryDrawerOpen(false);
      return;
    }
    void loadSessions();
  }, [sidebarOpen, loadSessions]);

  useEffect(() => {
    const handleResize = () => {
      setSidebarWidth(useAIAssistantStore.getState().sidebarWidth, true);
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [setSidebarWidth]);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
  };

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;
      const newWidth = clampSidebarWidth(window.innerWidth - e.clientX);
      setSidebarWidth(newWidth, false);
    };

    const handleMouseUp = () => {
      if (isResizing) {
        persistSidebarWidth(useAIAssistantStore.getState().sidebarWidth);
      }
      setIsResizing(false);
    };

    if (isResizing) {
      window.addEventListener("mousemove", handleMouseMove);
      window.addEventListener("mouseup", handleMouseUp);
    }

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing, setSidebarWidth]);

  const showChat = Boolean(activeSessionId);

  return (
    <Slide in={sidebarOpen} direction="left" mountOnEnter>
      <Box
        aria-hidden={!sidebarOpen}
        inert={!sidebarOpen || undefined}
        sx={(theme) => ({
          width: sidebarWidth,
          minWidth: SIDEBAR_WIDTH.MIN,
          height: `calc(100% - ${SSH_STATUS_BAR_HEIGHT}px)`,
          position: "absolute",
          top: 0,
          right: 0,
          zIndex: theme.zIndex.drawer,
          display: "flex",
          bgcolor: "background.paper",
          pointerEvents: sidebarOpen ? "auto" : "none",
        })}
      >
        <Box
          onMouseDown={handleMouseDown}
          onMouseEnter={() => setIsHovering(true)}
          onMouseLeave={() => setIsHovering(false)}
          sx={{
            width: SIDEBAR_RESIZE_HANDLE_WIDTH,
            cursor: "col-resize",
            backgroundColor: isHovering ? "primary.main" : "transparent",
            transition: isResizing ? "none" : "background-color 0.2s",
            "&:hover": {
              backgroundColor: "primary.main",
            },
          }}
        />
        <Box
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            borderLeft: 1,
            borderColor: "divider",
            minWidth: 0,
            overflow: "hidden",
            position: "relative",
          }}
        >
          <SessionPanel drawerContainer={chatContainer} />
          <Box
            ref={setChatRef}
            sx={{
              flex: 1,
              minHeight: 0,
              display: "flex",
              flexDirection: "column",
              position: "relative",
              bgcolor: "background.default",
            }}
          >
            {showChat ? (
              <ChatBox
                key={activeSessionId}
                adapter={adapter}
                currentUser={currentUser}
                members={[currentUser, aiUser]}
                activeConversationId={activeSessionId}
                onActiveConversationChange={() => {}}
                composerValue={composerValue}
                onComposerValueChange={setComposerValue}
                partRenderers={aiPartRenderers}
                features={{
                  attachments: false,
                  conversationHeader: false,
                }}
                onFinish={() => void refreshActiveSessionTitle()}
                onError={(err: any) => {
                  useAIAssistantStore.getState().setStreaming(false);
                  LogService.Error(`[AI Chat] ${err.source}: ${err.message}`);
                }}
                slots={{ messageAvatar: CustomMessageAvatar }}
                slotProps={{
                  composerInput: { placeholder: "输入消息..." },
                }}
                sx={{ flex: 1, minHeight: 0 }}
              />
            ) : loadingSessions ? (
              <Box
                sx={{
                  flex: 1,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <CircularProgress size={24} />
              </Box>
            ) : (
              <Box
                sx={{
                  flex: 1,
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 1,
                  px: 2,
                }}
              >
                <Typography variant="body2" color="text.secondary" align="center">
                  无法加载或创建会话
                </Typography>
                <Button size="small" variant="outlined" onClick={() => void loadSessions()}>
                  重试
                </Button>
              </Box>
            )}
          </Box>
        </Box>
      </Box>
    </Slide>
  );
};

export default AISideBar;
