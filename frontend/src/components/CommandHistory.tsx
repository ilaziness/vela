import React from "react";
import { Box, List, ListItem, ListItemButton, ListItemIcon, ListItemText, Paper, Typography, IconButton, Tooltip } from "@mui/material";
import { Terminal, ClearAll } from "@mui/icons-material";
import { CommandHistory } from "../types/command";
import { formatDateTime, t } from "../i18n";

interface CommandHistoryProps {
  history: CommandHistory[];
  onItemClick: (command: string) => void;
  onClearHistory: () => void;
}

const formatTimestamp = (timestamp: number) => formatDateTime(timestamp);

const CommandHistoryList: React.FC<CommandHistoryProps> = ({
  history,
  onItemClick,
  onClearHistory,
}) => {
  return (
    <Paper
      sx={{
        flex: 1,
        mb: 1,
        overflow: "auto",
        position: "relative",
      }}
      elevation={1}
    >
      <Box
        sx={{
          p: 1,
          borderBottom: 1,
          borderColor: "divider",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="subtitle2" color="text.secondary">
          {t("command.history")}
        </Typography>
        <Tooltip title={t("command.clear_history")}>
          <IconButton size="small" onClick={onClearHistory}>
            <ClearAll />
          </IconButton>
        </Tooltip>
      </Box>
      <List dense>
        {history.map((item, index) => (
          <ListItem
            key={`${index}-${item.timestamp}`}
            disablePadding
          >
            <ListItemButton onClick={() => onItemClick(item.command)}>
              <ListItemIcon sx={{ minWidth: 32 }}>
                <Terminal fontSize="small" color="action" />
              </ListItemIcon>
              <ListItemText
                primary={item.command}
                secondary={formatTimestamp(item.timestamp)}
                slotProps={{
                  primary: {
                    sx: {
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      fontSize: "0.875rem",
                    }
                  },
                  secondary: {
                    sx: {
                      fontSize: "0.75rem",
                    }
                  },
                }}
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Paper>
  );
};

export default CommandHistoryList;
