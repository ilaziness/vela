import React from "react";
import {
  Box,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Collapse,
  IconButton,
  Tooltip,
  Button,
} from "@mui/material";
import { ExpandMore, ChevronRight, Delete, Add } from "@mui/icons-material";
import { CommandInfo } from "../types/command";
import { t } from "../i18n";

interface CommandTreeProps {
  commandsByCategory: { [key: string]: CommandInfo[] };
  expandedCategories: { [key: string]: boolean };
  onToggleCategory: (category: string) => void;
  onCommandClick: (command: string) => void;
  onDeleteCommand: (category: string, name: string) => void;
  onAddCommand: () => void;
}

const CommandTree: React.FC<CommandTreeProps> = ({
  commandsByCategory,
  expandedCategories,
  onToggleCategory,
  onCommandClick,
  onDeleteCommand,
  onAddCommand,
}) => {
  return (
    <>
      <List dense>
        {Object.entries(commandsByCategory).map(([category, cmds]) => (
          <React.Fragment key={category}>
            <ListItem disablePadding>
              <ListItemButton onClick={() => onToggleCategory(category)}>
                <ListItemIcon sx={{ minWidth: 32 }}>
                  {expandedCategories[category] ? (
                    <ExpandMore fontSize="small" />
                  ) : (
                    <ChevronRight fontSize="small" />
                  )}
                </ListItemIcon>
                <ListItemText primary={category} />
              </ListItemButton>
            </ListItem>
            <Collapse
              in={expandedCategories[category]}
              timeout="auto"
              unmountOnExit
            >
              <List component="div" disablePadding dense>
                {cmds.map((cmd) => (
                  <ListItem
                    key={`${cmd.category}-${cmd.name}`}
                    disablePadding
                    secondaryAction={
                      cmd.is_custom && (
                        <IconButton
                          edge="end"
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            onDeleteCommand(cmd.category, cmd.name);
                          }}
                        >
                          <Delete fontSize="small" />
                        </IconButton>
                      )
                    }
                  >
                    <ListItemButton
                      sx={{ pl: 4 }}
                      onClick={() => onCommandClick(cmd.command)}
                    >
                      <Tooltip title={cmd.description} placement="right">
                        <ListItemText
                          primary={cmd.name}
                          secondary={cmd.command}
                        />
                      </Tooltip>
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            </Collapse>
          </React.Fragment>
        ))}
      </List>
      <Box sx={{ p: 2, mt: "auto" }}>
        <Button
          fullWidth
          variant="contained"
          startIcon={<Add />}
          onClick={onAddCommand}
        >
          {t("command.add_custom")}
        </Button>
      </Box>
    </>
  );
};

export default CommandTree;
