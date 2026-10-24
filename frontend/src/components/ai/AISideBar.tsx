import React, {
  useState,
  useEffect,
  useMemo,
  useCallback,
  useRef,
} from "react";
import {
  Box,
  Avatar,
  CircularProgress,
  Button,
  Typography,
} from "@mui/material";
import TerminalIcon from "@mui/icons-material/Terminal";
import { ChatBox, ChatMessageAvatar } from "@mui/x-chat";
import type { ChatToolInvocationState } from "@mui/x-chat/headless";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { LogService } from "../../../bindings/github.com/ilaziness/vela/services/index";
import { GenkitAdapter, aiUser, currentUser } from "./GenkitAdapter";
import ComposerSendStopButton from "./ComposerSendStopButton";
import { ComposerPlanDockRoot } from "./DockedPlanPanel";
import SessionPanel from "./SessionPanel";
import SSHToolState from "./SSHToolState";
import {
  RUN_SSH_COMMAND_TOOL,
  UPSERT_PLAN_TOOL,
  WRITE_LOCAL_FILE_TOOL,
} from "../../func/aiContext";
import {
  clampSidebarWidth,
  persistSidebarWidth,
  SIDEBAR_RESIZE_HANDLE_WIDTH,
  SIDEBAR_WIDTH,
} from "../../func/aiSidebar";

const SIDEBAR_WIDTH_TRANSITION = "width 225ms ease";
const SIDEBAR_WIDTH_TRANSITION_MS = 225;

const chatLocaleText = {
  messageToolApproveButtonLabel: "批准",
  messageToolDenyButtonLabel: "拒绝",
  messageToolInputLabel: "调用参数",
  messageToolOutputLabel: "执行结果",
  toolStateLabel: (state: ChatToolInvocationState) => {
    switch (state) {
      case "input-streaming":
        return "准备中";
      case "input-available":
        return "待执行";
      case "approval-requested":
        return "待批准";
      case "approval-responded":
        return "执行中";
      case "output-available":
        return "已完成";
      case "output-error":
        return "失败";
      case "output-denied":
        return "已拒绝";
      default:
        return state;
    }
  },
};

/** Keep upsert_plan out of the message stream; DockedPlanPanel owns that UI. */
function HiddenToolRoot() {
  return null;
}

const expandOnApproval = (ownerState: { section?: string; state?: string }) => {
  if (ownerState.section) return undefined;
  if (
    ownerState.state === "approval-requested" ||
    ownerState.state === "approval-responded" ||
    ownerState.state === "input-streaming"
  ) {
    return true;
  }
  return undefined;
};

const toolPartProps = {
  toolSlots: {
    [RUN_SSH_COMMAND_TOOL]: { icon: TerminalIcon, state: SSHToolState },
    [UPSERT_PLAN_TOOL]: { root: HiddenToolRoot },
  },
  // Built-in opens on approval-requested / input-streaming only; keep open through
  // approval-responded so approve→exec does not collapse (scroll jump) and Stop stays in context.
  // write_local_file expands so full content in the approval JSON is visible before approve.
  defaultExpanded: {
    [RUN_SSH_COMMAND_TOOL]: expandOnApproval,
    [WRITE_LOCAL_FILE_TOOL]: expandOnApproval,
  },
};

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
  },
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
  },
);

const AISideBar = () => {
  const sidebarOpen = useAIAssistantStore((state) => state.sidebarOpen);
  const sidebarWidth = useAIAssistantStore((state) => state.sidebarWidth);
  const activeSessionId = useAIAssistantStore((state) => state.activeSessionId);
  const loadingSessions = useAIAssistantStore((state) => state.loadingSessions);
  const composerValue = useAIAssistantStore((state) => state.composerValue);
  const loadSessions = useAIAssistantStore((state) => state.loadSessions);
  const refreshActiveSessionTitle = useAIAssistantStore(
    (state) => state.refreshActiveSessionTitle,
  );
  const setSidebarWidth = useAIAssistantStore((state) => state.setSidebarWidth);
  const setComposerValue = useAIAssistantStore(
    (state) => state.setComposerValue,
  );

  const adapter = useMemo(() => new GenkitAdapter(), []);

  const [chatContainer, setChatContainer] = useState<HTMLDivElement | null>(
    null,
  );

  const setChatRef = useCallback((node: HTMLDivElement | null) => {
    setChatContainer(node);
  }, []);

  const [isResizing, setIsResizing] = useState(false);
  const [isHovering, setIsHovering] = useState(false);
  // 开合：先开 transition 再改宽；openRef 延后更新以兼容 StrictMode；拖拽/clamp 即时改宽
  const [animateWidth, setAnimateWidth] = useState(false);
  const [visibleWidth, setVisibleWidth] = useState(0);
  const openRef = useRef(false);

  useEffect(() => {
    if (!sidebarOpen) {
      useAIAssistantStore.getState().setHistoryDrawerOpen(false);
      return;
    }
    void loadSessions();
  }, [sidebarOpen, loadSessions]);

  useEffect(() => {
    const openChanged = openRef.current !== sidebarOpen;

    if (openChanged) {
      setAnimateWidth(true);
      const targetOpen = sidebarOpen;
      const targetWidth = sidebarWidth;
      const frame = window.requestAnimationFrame(() => {
        openRef.current = targetOpen;
        setVisibleWidth(targetOpen ? targetWidth : 0);
      });
      const timer = window.setTimeout(
        () => setAnimateWidth(false),
        SIDEBAR_WIDTH_TRANSITION_MS,
      );
      return () => {
        window.cancelAnimationFrame(frame);
        window.clearTimeout(timer);
      };
    }

    if (sidebarOpen) {
      setVisibleWidth(sidebarWidth);
    }
  }, [sidebarOpen, sidebarWidth]);

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
    if (!isResizing) return;

    const prevUserSelect = document.body.style.userSelect;
    const prevCursor = document.body.style.cursor;
    document.body.style.userSelect = "none";
    document.body.style.cursor = "col-resize";

    const handleMouseMove = (e: MouseEvent) => {
      const newWidth = clampSidebarWidth(window.innerWidth - e.clientX);
      setSidebarWidth(newWidth, false);
    };

    const handleMouseUp = () => {
      persistSidebarWidth(useAIAssistantStore.getState().sidebarWidth);
      setIsResizing(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      document.body.style.userSelect = prevUserSelect;
      document.body.style.cursor = prevCursor;
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing, setSidebarWidth]);

  const showChat = Boolean(activeSessionId);

  return (
    <Box
      aria-hidden={!sidebarOpen}
      inert={!sidebarOpen || undefined}
      sx={{
        width: visibleWidth,
        flexShrink: 0,
        height: "100%",
        overflow: "hidden",
        transition:
          animateWidth && !isResizing ? SIDEBAR_WIDTH_TRANSITION : "none",
        display: "flex",
        bgcolor: "background.paper",
        pointerEvents: sidebarOpen ? "auto" : "none",
      }}
    >
      <Box
        sx={{
          width: sidebarWidth,
          minWidth: SIDEBAR_WIDTH.MIN,
          height: "100%",
          display: "flex",
          flexShrink: 0,
        }}
      >
        <Box
          onMouseDown={handleMouseDown}
          onMouseEnter={() => setIsHovering(true)}
          onMouseLeave={() => setIsHovering(false)}
          sx={{
            width: SIDEBAR_RESIZE_HANDLE_WIDTH,
            cursor: "col-resize",
            backgroundColor:
              isHovering || isResizing ? "primary.main" : "transparent",
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
                localeText={chatLocaleText}
                features={{
                  attachments: false,
                  conversationHeader: false,
                  autoScroll: { buffer: 2000 },
                }}
                onFinish={() => void refreshActiveSessionTitle()}
                onError={(err: any) => {
                  useAIAssistantStore.getState().setStreaming(false);
                  LogService.Error(`[AI Chat] ${err.source}: ${err.message}`);
                }}
                slots={{
                  messageAvatar: CustomMessageAvatar,
                  composerSendButton: ComposerSendStopButton,
                  composerRoot: ComposerPlanDockRoot,
                }}
                slotProps={{
                  // maxRows enables MUI auto-grow (rows=1 + height sync). Default TextArea CSS
                  // still has flex:1 + maxHeight:200 which stretches the field in a column
                  // composer and shows a useless scrollbar — override those.
                  composerInput: {
                    placeholder: "输入消息...",
                    maxRows: 8,
                    sx: {
                      flex: "0 0 auto",
                      alignSelf: "stretch",
                      width: "100%",
                      minHeight: "unset",
                      // ~8× body2 line box + padding; scroll only after the cap.
                      maxHeight: "calc(1.25rem * 8 + 8px)",
                      overflowY: "auto",
                    },
                  },
                  messageContent: {
                    partProps: {
                      tool: toolPartProps,
                      "dynamic-tool": toolPartProps,
                    },
                  },
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
                <Typography
                  variant="body2"
                  color="text.secondary"
                  align="center"
                >
                  无法加载或创建会话
                </Typography>
                <Button
                  size="small"
                  variant="outlined"
                  onClick={() => void loadSessions()}
                >
                  重试
                </Button>
              </Box>
            )}
          </Box>
        </Box>
      </Box>
    </Box>
  );
};

export default AISideBar;
