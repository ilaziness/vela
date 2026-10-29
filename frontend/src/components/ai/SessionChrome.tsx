import React, { useState } from "react";
import {
  Avatar,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Tooltip,
  Typography,
} from "@mui/material";
import { Add, Close, Delete } from "@mui/icons-material";
import { ChatConversationHeaderActions } from "@mui/x-chat";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { useSSHTabsStore } from "../../stores/ssh";
import {
  CurrentSSHBindStatus,
  resolveSSHBindState,
} from "../../func/aiContext";
import { displaySessionTitle, t } from "../../i18n";

/** Conversation-list avatar placeholder (no avatarUrl on AI sessions). */
export const SessionListAvatar = React.forwardRef<HTMLDivElement, any>(
  function SessionListAvatar(props, ref) {
    // Strip list-item owner fields so they are not forwarded to the DOM.
    const {
      ownerState: _ownerState,
      conversation: _conversation,
      selected: _selected,
      unread: _unread,
      focused: _focused,
      slots: _slots,
      slotProps: _slotProps,
      ...other
    } = props;

    return (
      <Avatar
        ref={ref}
        {...other}
        sx={{
          width: 40,
          height: 40,
          fontSize: 11,
          fontWeight: 600,
          bgcolor: "secondary.main",
          color: "secondary.contrastText",
          flexShrink: 0,
        }}
      >
        AI
      </Avatar>
    );
  },
);

function useCurrentSSHIndicator(): {
  line: string;
  hint: string;
  status: CurrentSSHBindStatus;
} {
  const currentTab = useSSHTabsStore((s) => s.currentTab);
  const tab = useSSHTabsStore((s) =>
    s.sshTabs.find((item) => item.index === currentTab),
  );
  const bind = resolveSSHBindState(tab);

  switch (bind.status) {
    case CurrentSSHBindStatus.Connected:
      return {
        status: bind.status,
        line: t("ai.session.ssh_connected", bind.targetLabel),
        hint: "",
      };
    case CurrentSSHBindStatus.Connecting:
      return {
        status: bind.status,
        line: t("ai.session.ssh_connecting", bind.targetLabel),
        hint: "",
      };
    case CurrentSSHBindStatus.Disconnected:
      return {
        status: bind.status,
        line: t("ai.session.ssh_disconnected", bind.targetLabel),
        hint: t("ai.session.ssh_disconnected_hint"),
      };
    default:
      return {
        status: CurrentSSHBindStatus.None,
        line: t("ai.session.ssh_none"),
        hint: t("ai.session.ssh_none_hint"),
      };
  }
}

/** ChatBox conversationSubtitle slot — SSH bind status, not conversation.subtitle. */
export const SSHBindSubtitle = React.forwardRef<HTMLParagraphElement, any>(
  function SSHBindSubtitle(props, ref) {
    const {
      ownerState: _ownerState,
      slots: _slots,
      slotProps: _slotProps,
      classes: _classes,
      ...other
    } = props;
    const ssh = useCurrentSSHIndicator();
    const title = ssh.hint ? `${ssh.line} — ${ssh.hint}` : ssh.line;

    return (
      <Typography
        ref={ref}
        component="p"
        variant="caption"
        noWrap
        {...other}
        color={
          ssh.status === CurrentSSHBindStatus.Connected
            ? "info.main"
            : "warning.main"
        }
        title={title}
        sx={{
          m: 0,
          fontWeight: 500,
          lineHeight: 1.4,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {ssh.line}
        {ssh.hint ? (
          <Box
            component="span"
            sx={{ color: "text.disabled", fontWeight: 400, ml: 0.75 }}
          >
            {ssh.hint}
          </Box>
        ) : null}
      </Typography>
    );
  },
);

/** ChatBox conversationHeaderActions — new / delete active / close assistant. */
export const SessionHeaderActions = React.forwardRef<HTMLDivElement, any>(
  function SessionHeaderActions(props, ref) {
    const sessions = useAIAssistantStore((s) => s.sessions);
    const activeSessionId = useAIAssistantStore((s) => s.activeSessionId);
    const isStreaming = useAIAssistantStore((s) => s.isStreaming);
    const createSession = useAIAssistantStore((s) => s.createSession);
    const deleteSession = useAIAssistantStore((s) => s.deleteSession);
    const setSidebarOpen = useAIAssistantStore((s) => s.setSidebarOpen);

    const [deleteOpen, setDeleteOpen] = useState(false);
    const activeSession = sessions.find((s) => s.id === activeSessionId);

    const handleConfirmDelete = async () => {
      if (!activeSessionId) return;
      await deleteSession(activeSessionId);
      setDeleteOpen(false);
    };

    return (
      <>
        <ChatConversationHeaderActions ref={ref} {...props}>
          <Tooltip title={t("ai.session.new")}>
            <span>
              <IconButton
                size="small"
                disabled={isStreaming}
                onClick={() => void createSession()}
                aria-label={t("ai.session.new")}
              >
                <Add fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t("ai.session.delete_title")}>
            <span>
              <IconButton
                size="small"
                disabled={isStreaming || !activeSessionId}
                onClick={() => setDeleteOpen(true)}
                aria-label={t("ai.session.delete_title")}
              >
                <Delete fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t("ai.session.close_assistant")}>
            <IconButton
              size="small"
              onClick={() => setSidebarOpen(false)}
              aria-label={t("ai.session.close_assistant")}
            >
              <Close fontSize="small" />
            </IconButton>
          </Tooltip>
        </ChatConversationHeaderActions>

        <Dialog open={deleteOpen} onClose={() => setDeleteOpen(false)}>
          <DialogTitle>{t("ai.session.delete_title")}</DialogTitle>
          <DialogContent>
            <DialogContentText>
              {t(
                "ai.session.delete_confirm",
                displaySessionTitle(activeSession?.title),
              )}
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button color="error" onClick={() => void handleConfirmDelete()}>
              {t("common.delete")}
            </Button>
          </DialogActions>
        </Dialog>
      </>
    );
  },
);
