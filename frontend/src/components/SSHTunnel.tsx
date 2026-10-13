import React, { useState, useEffect, useCallback } from "react";
import {
  Box,
  List,
  ListItem,
  Typography,
  IconButton,
  Slide,
  Tooltip,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  CircularProgress,
  Chip,
} from "@mui/material";
import {
  Add,
  Close,
  Stop,
  ExpandMore,
  Lan,
  SettingsEthernet,
  Public,
} from "@mui/icons-material";
import { useMessageStore } from "../stores/message";
import {
  TunnelList,
  TunnelInfo,
} from "../../bindings/github.com/ilaziness/vela/internal/tunnel/models";
import { SSHTunnelService } from "../../bindings/github.com/ilaziness/vela/services";
import TunnelForm from "./TunnelForm";
import { parseCallServiceError } from "../func/service";

interface SSHTunnelProps {
  sessionID: string;
  open: boolean;
  statusBarHeight: string;
  onClose: () => void;
}

const SSHTunnel: React.FC<SSHTunnelProps> = ({
  sessionID,
  open,
  statusBarHeight,
  onClose,
}) => {
  const { errorMessage, successMessage } = useMessageStore();

  // 状态管理
  const [tunnelGroups, setTunnelGroups] = useState<TunnelList[]>([]);
  const [loading, setLoading] = useState(false);
  const [formDialogOpen, setFormDialogOpen] = useState(false);
  const [stoppingIds, setStoppingIds] = useState<Set<string>>(new Set());

  // 获取隧道列表
  const fetchTunnelList = useCallback(async () => {
    if (!open) return;

    setLoading(true);
    try {
      const groups = await SSHTunnelService.TunnelList();
      setTunnelGroups(groups ?? []);
    } catch (err) {
      errorMessage("获取隧道列表失败");
      console.error("Failed to fetch tunnel list:", err);
    } finally {
      setLoading(false);
    }
  }, [open, errorMessage]);

  // 组件打开时刷新列表
  useEffect(() => {
    if (open) {
      void fetchTunnelList();
    }
  }, [open, fetchTunnelList]);

  // 处理表单打开
  const handleFormOpen = () => {
    setFormDialogOpen(true);
  };

  // 处理表单关闭
  const handleFormClose = () => {
    setFormDialogOpen(false);
  };

  // 处理隧道创建成功
  const handleTunnelCreated = () => {
    fetchTunnelList();
  };

  // 停止隧道
  const handleStopTunnel = async (tunnelId: string) => {
    setStoppingIds((prev) => new Set(prev).add(tunnelId));
    try {
      await SSHTunnelService.StopByID(tunnelId);
      successMessage("隧道停止成功");
      fetchTunnelList();
    } catch (err) {
      errorMessage(`停止隧道失败：${parseCallServiceError(err)}`);
      console.error("Failed to stop tunnel:", err);
    } finally {
      setStoppingIds((prev) => {
        const newSet = new Set(prev);
        newSet.delete(tunnelId);
        return newSet;
      });
    }
  };

  // 获取隧道类型显示信息
  const getTunnelTypeInfo = (type: string) => {
    switch (type) {
      case "local":
        return {
          label: "本地端口转发",
          icon: <Lan />,
          color: "primary" as const,
        };
      case "remote":
        return {
          label: "远程端口转发",
          icon: <SettingsEthernet />,
          color: "secondary" as const,
        };
      case "dynamic":
        return {
          label: "动态端口转发",
          icon: <Public />,
          color: "info" as const,
        };
      default:
        return { label: type, icon: <Lan />, color: "default" as const };
    }
  };

  // 渲染隧道项
  const renderTunnelItem = (tunnel: TunnelInfo) => {
    const isStopping = stoppingIds.has(tunnel.id);
    const typeInfo = getTunnelTypeInfo(tunnel.tunnelType);

    return (
      <ListItem
        key={tunnel.id}
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          mb: 1,
          "&:hover": {
            backgroundColor: "action.hover",
            borderColor: "primary.main",
          },
          transition: "all 0.2s ease-in-out",
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            width: "100%",
            gap: 2,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 2, flex: 1 }}>
            <Chip
              icon={typeInfo.icon}
              label={typeInfo.label}
              color={typeInfo.color}
              size="small"
              variant="outlined"
            />
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" sx={{ fontWeight: "medium" }}>
                本地地址: {tunnel.localAddr}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                远程地址: {tunnel.remoteAddr}
              </Typography>
            </Box>
          </Box>

          <Tooltip title="停止隧道">
            <span>
              <IconButton
                size="small"
                onClick={() => handleStopTunnel(tunnel.id)}
                disabled={isStopping}
                color="error"
              >
                {isStopping ? (
                  <CircularProgress size={20} />
                ) : (
                  <Stop fontSize="small" />
                )}
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      </ListItem>
    );
  };

  // 渲染隧道分组
  const renderTunnelGroups = () => {
    // 计算总隧道数
    const totalTunnels = tunnelGroups.reduce(
      (total, group) => total + group.tunnels.length,
      0,
    );

    if (totalTunnels === 0) {
      return (
        <Box sx={{ p: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            暂无隧道
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
            点击右上角 + 按钮创建隧道
          </Typography>
        </Box>
      );
    }

    // 直接渲染后端返回的分组数据
    return tunnelGroups.map((group) => {
      const typeInfo = getTunnelTypeInfo(group.tunnelType);

      return (
        <Accordion key={group.tunnelType} defaultExpanded sx={{ mb: 1 }}>
          <AccordionSummary expandIcon={<ExpandMore />}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              {typeInfo.icon}
              <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
                {typeInfo.label}
              </Typography>
              <Chip
                label={group.tunnels.length}
                size="small"
                color={typeInfo.color}
                variant="outlined"
              />
            </Box>
          </AccordionSummary>
          <AccordionDetails sx={{ p: 0 }}>
            <List sx={{ width: "100%" }}>
              {group.tunnels.map((tunnel) => renderTunnelItem(tunnel))}
            </List>
          </AccordionDetails>
        </Accordion>
      );
    });
  };

  return (
    <>
      {/* 主组件 */}
      <Box
        sx={{
          position: "absolute",
          bottom: `${statusBarHeight}`,
          left: 0,
          right: 0,
          height: "50%",
          zIndex: 10,
          overflow: "hidden",
          pointerEvents: open ? "auto" : "none",
        }}
      >
        <Slide in={open} direction="up" mountOnEnter unmountOnExit>
          <Box
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              bgcolor: "background.default",
              borderTop: 1,
              borderColor: "divider",
            }}
          >
            {/* 标题栏 */}
            <Box
              sx={{
                p: 1,
                px: 2,
                borderBottom: 1,
                borderColor: "divider",
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                height: "40px",
              }}
            >
              <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
                SSH 隧道管理
              </Typography>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Tooltip title="创建隧道">
                  <IconButton
                    onClick={handleFormOpen}
                    size="small"
                    color="primary"
                  >
                    <Add fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title="关闭">
                  <IconButton onClick={onClose} size="small">
                    <Close fontSize="small" />
                  </IconButton>
                </Tooltip>
              </Box>
            </Box>

            {/* 内容区域 */}
            <Box
              sx={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                overflow: "hidden",
                p: 2,
              }}
            >
              {loading ? (
                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "center",
                    alignItems: "center",
                    flex: 1,
                  }}
                >
                  <CircularProgress size={40} />
                </Box>
              ) : (
                <Box sx={{ flex: 1, overflow: "auto" }}>
                  {renderTunnelGroups()}
                </Box>
              )}
            </Box>
          </Box>
        </Slide>
      </Box>

      {/* 隧道创建表单 */}
      <TunnelForm
        open={formDialogOpen}
        onClose={handleFormClose}
        sessionID={sessionID}
        onSuccess={handleTunnelCreated}
      />
    </>
  );
};

export default SSHTunnel;
