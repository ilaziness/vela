import { Box, Tooltip } from "@mui/material";
import React from "react";
import CloseIcon from "@mui/icons-material/Close";
import { Draggable } from "@hello-pangea/dnd";
import { ConnectionStatus } from "../types/ssh";
import { displaySSHTabName, useLanguage } from "../i18n";

export interface DraggableTabProps {
  item: {
    index: string;
    name: string;
    connectionStatus?: ConnectionStatus;
  };
  index: number;
  isActive: boolean;
  onClick: () => void;
  onClose: (e: React.MouseEvent) => void;
  onContextMenu: (e: React.MouseEvent) => void;
}

export const DraggableTab = React.memo(function DraggableTab({
  item,
  index,
  isActive,
  onClick,
  onClose,
  onContextMenu,
}: DraggableTabProps) {
  useLanguage();
  const label = displaySSHTabName(item.name);
  const getBackgroundColor = (isActive: boolean, isDragging: boolean) => {
    if (isActive) return "action.selected";
    if (isDragging) return "action.hover";
    return "inherit";
  };

  let connectionStatusColor = "text.disabled";

  if (item.connectionStatus === ConnectionStatus.Connected) {
    connectionStatusColor = "success.main";
  } else if (item.connectionStatus === ConnectionStatus.Disconnected) {
    connectionStatusColor = "error.main";
  } else if (item.connectionStatus === ConnectionStatus.Connecting) {
    connectionStatusColor = "warning.main";
  }

  return (
    <Draggable draggableId={item.index} index={index}>
      {(provided, snapshot) => (
        <Box
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          onClick={onClick}
          onContextMenu={onContextMenu}
          sx={{
            display: "flex",
            alignItems: "center",
            minHeight: 34,
            minWidth: 108,
            width: 108,
            px: 0.75,
            gap: 0.25,
            borderRight: 2,
            borderColor: "divider",
            backgroundColor: getBackgroundColor(isActive, snapshot.isDragging),
            opacity: snapshot.isDragging ? 0.9 : 1,
            boxShadow: snapshot.isDragging
              ? "0 4px 8px rgba(0,0,0,0.3)"
              : "none",
            cursor: "pointer",
            userSelect: "none",
            "--wails-draggable": "no-drag",
            transition: "background-color 0.2s",
            "&:hover": {
              backgroundColor: isActive ? "action.selected" : "action.hover",
            },
            "&:active": {
              cursor: "grabbing",
            },
          }}
        >
          <Box
            sx={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              flexShrink: 0,
              backgroundColor: connectionStatusColor,
            }}
          />
          <Tooltip title={label}>
            <Box
              sx={{
                flex: 1,
                minWidth: 0,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {label}
            </Box>
          </Tooltip>
          <Box
            component="span"
            onClick={onClose}
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 16,
              height: 16,
              borderRadius: "50%",
              cursor: "pointer",
              opacity: 0.7,
              flexShrink: 0,
              "&:hover": {
                opacity: 1,
                backgroundColor: "action.hover",
              },
            }}
          >
            <CloseIcon sx={{ fontSize: 12 }} />
          </Box>
        </Box>
      )}
    </Draggable>
  );
});
