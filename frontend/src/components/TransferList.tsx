import React, { useMemo } from "react";
import {
  Box,
  List,
  ListItem,
  LinearProgress,
  Typography,
  IconButton,
  Slide,
  Tooltip,
} from "@mui/material";
import {
  CloudUpload,
  CloudDownload,
  Close,
  ArrowForward,
  ArrowBack,
  ClearAll,
  Cancel,
  Replay,
  DeleteSweep,
} from "@mui/icons-material";
import { useTransferStore } from "../stores/transfer";
import { formatFileSize, parseCallServiceError } from "../func/service";
import { ProgressData } from "../../bindings/github.com/ilaziness/vela/services/models";
import { SftpService } from "../../bindings/github.com/ilaziness/vela/services";
import { useMessageStore } from "../stores/message";

interface TransferListProps {
  sessionID: string;
  open: boolean;
  statusBarHeight: string;
  onClose: () => void;
  sftpReady?: boolean;
}

const TransferList: React.FC<TransferListProps> = ({
  sessionID,
  open,
  statusBarHeight,
  onClose,
  sftpReady = false,
}) => {
  const {
    transfers: transfersMap,
    removeProgress,
    clearCompletedTransfers,
  } = useTransferStore();
  const { errorMessage } = useMessageStore();
  const transfersList = useMemo(() => {
    return transfersMap.get(sessionID) || [];
  }, [transfersMap, sessionID]);

  const handleRemove = async (id: string, dismissQueue: boolean) => {
    if (dismissQueue) {
      try {
        await SftpService.DismissTransfer(id);
      } catch (err) {
        errorMessage(parseCallServiceError(err));
        return;
      }
    }
    removeProgress(sessionID, id);
  };

  const handleClear = () => {
    clearCompletedTransfers(sessionID);
  };

  const handleClearFailed = async () => {
    const failed = transfersList.filter(
      (t) => t.done && t.error && t.error.trim() !== "",
    );
    let firstErr: unknown;
    for (const t of failed) {
      try {
        await SftpService.DismissTransfer(t.id);
        removeProgress(sessionID, t.id);
      } catch (err) {
        if (!firstErr) firstErr = err;
      }
    }
    if (firstErr) {
      errorMessage(parseCallServiceError(firstErr));
    }
  };

  const handleRetry = async (transfer: ProgressData) => {
    try {
      await SftpService.RetryTransfer(sessionID, transfer.id);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  return (
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
              传输列表
            </Typography>
            <Box sx={{ display: "flex", gap: 1 }}>
              <Tooltip title="清除失败任务">
                <IconButton onClick={() => void handleClearFailed()} size="small">
                  <DeleteSweep fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title="清空已成功任务">
                <IconButton onClick={handleClear} size="small">
                  <ClearAll fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title="关闭">
                <IconButton onClick={onClose} size="small">
                  <Close fontSize="small" />
                </IconButton>
              </Tooltip>
            </Box>
          </Box>
          {transfersList.length === 0 ? (
            <Box sx={{ p: 2, textAlign: "center" }}>
              <Typography variant="body2" color="text.secondary">
                {sftpReady
                  ? "暂无传输任务"
                  : "请先打开 SFTP 标签并连接成功后，可查看未完成的传输记录"}
              </Typography>
            </Box>
          ) : (
            <Box
              sx={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                overflow: "hidden",
                p: 0,
              }}
            >
              <List sx={{ flex: 1, overflow: "auto" }}>
                {transfersList.map((transfer: ProgressData) => {
                  const progress = transfer.done && !transfer.error
                    ? 100
                    : transfer.rate;
                  const isUpload =
                    transfer.transferType.toLowerCase().includes("upload");
                  const isCompleted = transfer.done;
                  const hasError =
                    transfer.error && transfer.error.trim() !== "";

                  return (
                    <ListItem
                      key={transfer.id}
                      sx={{
                        border: 1,
                        borderColor: hasError ? "error.main" : "divider",
                        borderRadius: 1,
                        mb: 1,
                        "&:hover": {
                          backgroundColor: "action.hover",
                          borderColor: hasError ? "error.main" : "primary.main",
                        },
                        transition: "all 0.2s ease-in-out",
                      }}
                    >
                      <Box
                        sx={{
                          display: "flex",
                          flexDirection: "column",
                          width: "100%",
                          gap: 1,
                        }}
                      >
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            width: "100%",
                            gap: 1,
                            flexWrap: "wrap",
                          }}
                        >
                          {isUpload ? (
                            <CloudUpload
                              color={hasError ? "error" : "primary"}
                              sx={{ flexShrink: 0 }}
                            />
                          ) : (
                            <CloudDownload
                              color={hasError ? "error" : "secondary"}
                              sx={{ flexShrink: 0 }}
                            />
                          )}

                          <Typography
                            variant="body2"
                            sx={{
                              flex: 1,
                              minWidth: 0,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                            }}
                            title={transfer.localFile}
                          >
                            {transfer.localFile}
                          </Typography>

                          {isUpload ? (
                            <ArrowForward
                              sx={{
                                flexShrink: 0,
                                color: hasError
                                  ? "error.main"
                                  : "primary.light",
                              }}
                            />
                          ) : (
                            <ArrowBack
                              sx={{
                                flexShrink: 0,
                                color: hasError
                                  ? "error.main"
                                  : "primary.light",
                              }}
                            />
                          )}

                          <Typography
                            variant="body2"
                            sx={{
                              flex: 1,
                              minWidth: 0,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                            }}
                            title={transfer.remoteFile}
                          >
                            {transfer.remoteFile}
                          </Typography>

                          <Box sx={{ flex: 2, minWidth: 100, maxWidth: 200 }}>
                            <LinearProgress
                              variant="determinate"
                              value={progress}
                              sx={{
                                height: 6,
                                borderRadius: 3,
                                "& .MuiLinearProgress-bar": {
                                  backgroundColor: hasError
                                    ? "error.main"
                                    : isCompleted
                                      ? "success.main"
                                      : undefined,
                                },
                              }}
                            />
                          </Box>

                          <Typography
                            variant="caption"
                            sx={{ flexShrink: 0, minWidth: "fit-content" }}
                          >
                            {formatFileSize(transfer.totalSize)}
                          </Typography>

                          <Typography
                            variant="caption"
                            sx={{ flexShrink: 0, minWidth: "fit-content" }}
                          >
                            {Number(progress).toFixed(2)}%
                          </Typography>

                          {!transfer.done ? (
                            <Tooltip title="取消传输">
                              <IconButton
                                size="small"
                                onClick={() => {
                                  SftpService.CancelTransfer(transfer.id).catch(
                                    (err) =>
                                      errorMessage(parseCallServiceError(err)),
                                  );
                                }}
                                sx={{ flexShrink: 0 }}
                              >
                                <Cancel fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          ) : (
                            <>
                              {hasError && (
                                <Tooltip title="重试">
                                  <IconButton
                                    size="small"
                                    onClick={() => void handleRetry(transfer)}
                                    sx={{ flexShrink: 0 }}
                                  >
                                    <Replay fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                              )}
                              <Tooltip title="删除">
                                <IconButton
                                  size="small"
                                  onClick={() =>
                                    void handleRemove(transfer.id, Boolean(hasError))
                                  }
                                  sx={{ flexShrink: 0 }}
                                >
                                  <Close fontSize="small" />
                                </IconButton>
                              </Tooltip>
                            </>
                          )}
                        </Box>

                        {hasError && (
                          <Typography
                            variant="caption"
                            color="error"
                            sx={{
                              px: 1,
                              wordBreak: "break-word",
                            }}
                          >
                            错误: {transfer.error}
                          </Typography>
                        )}
                      </Box>
                    </ListItem>
                  );
                })}
              </List>
            </Box>
          )}
        </Box>
      </Slide>
    </Box>
  );
};

export default TransferList;
