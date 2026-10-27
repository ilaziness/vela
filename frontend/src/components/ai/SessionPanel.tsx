import React, { useState } from 'react';
import {
  Box,
  IconButton,
  Typography,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
  Tooltip,
} from '@mui/material';
import { Menu, Add, Delete, Close } from '@mui/icons-material';
import { useAIAssistantStore } from '../../stores/aiAssistant';
import { useSSHTabsStore } from '../../stores/ssh';
import {
  CurrentSSHBindStatus,
  resolveSSHBindState,
} from '../../func/aiContext';
import { getSidebarContentWidth } from '../../func/aiSidebar';

interface SessionPanelProps {
  drawerContainer: HTMLDivElement | null;
}

function pad2(n: number): string {
  return n.toString().padStart(2, '0');
}

function formatAbsoluteTime(unixSeconds: number): string {
  const d = new Date(unixSeconds * 1000);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`;
}

function formatRelativeTime(unixSeconds: number): string {
  const diff = Date.now() - unixSeconds * 1000;
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days <= 7) return `${days} 天前`;
  return formatAbsoluteTime(unixSeconds);
}

function useCurrentSSHIndicator(): { line: string; hint: string; status: CurrentSSHBindStatus } {
  const currentTab = useSSHTabsStore((s) => s.currentTab);
  const tab = useSSHTabsStore((s) => s.sshTabs.find((t) => t.index === currentTab));
  const bind = resolveSSHBindState(tab);

  switch (bind.status) {
    case CurrentSSHBindStatus.Connected:
      return { status: bind.status, line: `SSH：${bind.targetLabel}`, hint: '' };
    case CurrentSSHBindStatus.Connecting:
      return {
        status: bind.status,
        line: `SSH：连接中… ${bind.targetLabel}`,
        hint: '',
      };
    case CurrentSSHBindStatus.Disconnected:
      return {
        status: bind.status,
        line: `SSH：已断开 ${bind.targetLabel}`,
        hint: '当前无法执行远程命令',
      };
    default:
      return {
        status: CurrentSSHBindStatus.None,
        line: 'SSH：无连接',
        hint: '连接 SSH 后可执行远程命令',
      };
  }
}

const SessionPanel: React.FC<SessionPanelProps> = ({ drawerContainer }) => {
  const sessions = useAIAssistantStore((s) => s.sessions);
  const activeSessionId = useAIAssistantStore((s) => s.activeSessionId);
  const sidebarWidth = useAIAssistantStore((s) => s.sidebarWidth);
  const historyDrawerOpen = useAIAssistantStore((s) => s.historyDrawerOpen);
  const isStreaming = useAIAssistantStore((s) => s.isStreaming);
  const setHistoryDrawerOpen = useAIAssistantStore((s) => s.setHistoryDrawerOpen);
  const setSidebarOpen = useAIAssistantStore((s) => s.setSidebarOpen);
  const selectSession = useAIAssistantStore((s) => s.selectSession);
  const createSession = useAIAssistantStore((s) => s.createSession);
  const deleteSession = useAIAssistantStore((s) => s.deleteSession);
  const sshIndicator = useCurrentSSHIndicator();

  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);

  const activeSession = sessions.find((s) => s.id === activeSessionId);
  const deleteTarget = sessions.find((s) => s.id === deleteTargetId);
  const drawerPaperWidth = getSidebarContentWidth(sidebarWidth);

  const handleConfirmDelete = async () => {
    if (!deleteTargetId) return;
    await deleteSession(deleteTargetId);
    setDeleteTargetId(null);
  };

  const toggleHistoryDrawer = () => setHistoryDrawerOpen(!historyDrawerOpen);

  return (
    <>
      <Box
        sx={{
          minHeight: 40,
          display: 'flex',
          flexDirection: 'column',
          borderBottom: 1,
          borderColor: 'divider',
          flexShrink: 0,
        }}
      >
        <Box
          sx={{
            height: 40,
            minHeight: 40,
            display: 'flex',
            alignItems: 'center',
            px: 1,
            gap: 0.5,
          }}
        >
          <Tooltip title={historyDrawerOpen ? '关闭会话历史' : '会话历史'}>
            <span>
              <IconButton
                size="small"
                disabled={isStreaming || !drawerContainer}
                onClick={toggleHistoryDrawer}
                aria-label={historyDrawerOpen ? '关闭会话历史' : '打开会话历史'}
                aria-expanded={historyDrawerOpen}
              >
                <Menu fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Typography
            variant="body2"
            noWrap
            sx={{ flex: 1, fontWeight: 500, minWidth: 0 }}
          >
            {activeSession?.title || '新会话'}
          </Typography>
          <Tooltip title="新建会话">
            <span>
              <IconButton
                size="small"
                disabled={isStreaming}
                onClick={() => void createSession()}
                aria-label="新建会话"
              >
                <Add fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="关闭 AI 助手">
            <IconButton
              size="small"
              onClick={() => setSidebarOpen(false)}
              aria-label="关闭 AI 助手"
            >
              <Close fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        <Box
          sx={{
            px: 1.5,
            pb: 0.5,
            pt: 0,
            display: 'flex',
            alignItems: 'baseline',
            gap: 0.75,
            minWidth: 0,
          }}
        >
          <Typography
            variant="caption"
            noWrap
            color={
              sshIndicator.status === CurrentSSHBindStatus.Connected
                ? 'info.main'
                : 'warning.main'
            }
            sx={{ flexShrink: 0, fontWeight: 500 }}
            title={sshIndicator.line}
          >
            {sshIndicator.line}
          </Typography>
          {sshIndicator.hint ? (
            <Typography
              variant="caption"
              noWrap
              color="text.disabled"
              sx={{ minWidth: 0 }}
              title={sshIndicator.hint}
            >
              {sshIndicator.hint}
            </Typography>
          ) : null}
        </Box>
      </Box>

      {drawerContainer && (
        <Drawer
          anchor="left"
          open={historyDrawerOpen}
          onClose={() => setHistoryDrawerOpen(false)}
          variant="temporary"
          container={drawerContainer}
          slotProps={{
            root: { container: drawerContainer },
            paper: {
              sx: {
                position: 'absolute',
                width: drawerPaperWidth,
                height: '100%',
                boxSizing: 'border-box',
              },
            },
          }}
          sx={{
            position: 'absolute',
            zIndex: 1,
            '& .MuiBackdrop-root': { position: 'absolute' },
          }}
        >
          <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <Box
              sx={{
                px: 2,
                py: 1.5,
                borderBottom: 1,
                borderColor: 'divider',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <Typography variant="subtitle2">会话历史</Typography>
              <Button size="small" onClick={() => setHistoryDrawerOpen(false)}>
                关闭
              </Button>
            </Box>
            <List sx={{ flex: 1, overflow: 'auto', py: 0 }}>
              {sessions.map((session) => (
                <ListItem
                  key={session.id}
                  disablePadding
                  secondaryAction={
                    <IconButton
                      edge="end"
                      size="small"
                      disabled={isStreaming}
                      aria-label="删除会话"
                      onClick={() => setDeleteTargetId(session.id)}
                    >
                      <Delete fontSize="small" />
                    </IconButton>
                  }
                >
                  <ListItemButton
                    selected={session.id === activeSessionId}
                    disabled={isStreaming}
                    onClick={() => selectSession(session.id)}
                  >
                    <ListItemText
                      primary={session.title}
                      secondary={formatRelativeTime(session.updatedAt)}
                      slotProps={{
                        primary: { noWrap: true },
                        secondary: { variant: 'caption' },
                      }}
                    />
                  </ListItemButton>
                </ListItem>
              ))}
            </List>
          </Box>
        </Drawer>
      )}

      <Dialog open={deleteTargetId !== null} onClose={() => setDeleteTargetId(null)}>
        <DialogTitle>删除会话</DialogTitle>
        <DialogContent>
          <DialogContentText>
            确定删除会话「{deleteTarget?.title || '新会话'}」？此操作不可撤销。
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTargetId(null)}>取消</Button>
          <Button color="error" onClick={() => void handleConfirmDelete()}>
            删除
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

export default SessionPanel;
