import { useEffect } from "react";
import {
  Box,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
} from "@mui/material";
import { useNavigate, useParams } from "react-router";
import { useToolStore } from "../stores/tool";
import OpBar from "./OpBar";
import { getToolIcon } from "../func/toolIcons";
import { useFrameless } from "../hooks/useFrameless";
import { displayToolName } from "../func/toolDisplay";

interface ToolLayoutProps {
  readonly children: React.ReactNode;
}

export default function ToolLayout({ children }: ToolLayoutProps) {
  const navigate = useNavigate();
  const { toolId } = useParams<{ toolId: string }>();
  const { tools, fetchTools } = useToolStore();
  const frameless = useFrameless() === true;

  useEffect(() => {
    if (tools.length === 0) {
      fetchTools();
    }
  }, [tools.length, fetchTools]);

  const handleToolClick = (id: string) => {
    navigate(`/tools/${id}`);
  };

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        bgcolor: "background.default",
      }}
    >
      {frameless && (
        <Box sx={{ height: 40 }}>
          <OpBar />
        </Box>
      )}

      {/* 内容区域 */}
      <Box sx={{ flex: 1, display: "flex", overflow: "hidden" }}>
        {/* 左侧工具列表 */}
        <Drawer
          variant="permanent"
          sx={{
            width: 200,
            flexShrink: 0,
            "& .MuiDrawer-paper": {
              width: 200,
              position: "relative",
              border: "none",
              borderRight: 1,
              borderColor: "divider",
            },
          }}
        >
          <List dense>
            {tools.map((tool) => {
              const IconComponent = getToolIcon(tool.icon);
              const isActive = tool.id === toolId;

              return (
                <ListItem key={tool.id} disablePadding>
                  <ListItemButton
                    selected={isActive}
                    onClick={() => handleToolClick(tool.id)}
                  >
                    <ListItemIcon sx={{ minWidth: 36 }}>
                      <IconComponent
                        fontSize="small"
                        color={isActive ? "primary" : "inherit"}
                      />
                    </ListItemIcon>
                    <ListItemText
                      primary={displayToolName(tool)}
                      slotProps={{
                        primary: {
                          sx: {
                            fontWeight: isActive ? "bold" : "normal",
                          }
                        },
                      }}
                    />
                  </ListItemButton>
                </ListItem>
              );
            })}
          </List>
        </Drawer>

        {/* 右侧内容区域 */}
        <Box
          sx={{
            flex: 1,
            overflow: "auto",
            p: 3,
          }}
        >
          {children}
        </Box>
      </Box>
    </Box>
  );
}
