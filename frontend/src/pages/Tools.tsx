import { useEffect } from "react";
import {
  Box,
  Card,
  CardContent,
  Grid,
  Typography,
} from "@mui/material";
import { useNavigate } from "react-router";
import { useToolStore } from "../stores/tool";
import { Tool } from "../types/tool";
import OpBar from "../components/OpBar";
import { getToolIcon } from "../func/toolIcons";
import { dragRegionSx, useFrameless } from "../hooks/useFrameless";

interface ToolCardProps {
  readonly tool: Tool;
  readonly onClick: () => void;
}

function ToolCard({ tool, onClick }: ToolCardProps) {
  const IconComponent = getToolIcon(tool.icon);

  return (
    <Card
      onClick={onClick}
      sx={{
        cursor: "pointer",
        height: "100%",
        transition: "all 0.2s ease-in-out",
        "&:hover": {
          transform: "translateY(-4px)",
          boxShadow: (theme) => theme.shadows[8],
        },
      }}
    >
      <CardContent sx={{ textAlign: "center", p: 3 }}>
        <Box
          sx={{
            width: 64,
            height: 64,
            borderRadius: "50%",
            bgcolor: "primary.main",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 16px",
          }}
        >
          <IconComponent sx={{ fontSize: 32, color: "white" }} />
        </Box>
        <Typography variant="h6" gutterBottom>
          {tool.name}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {tool.description}
        </Typography>
        <Typography
          variant="caption"
          color="primary"
          sx={{ display: "block", mt: 1 }}
        >
          {tool.category}
        </Typography>
      </CardContent>
    </Card>
  );
}

export default function Tools() {
  const navigate = useNavigate();
  const { tools, fetchTools } = useToolStore();
  const frameless = useFrameless();

  useEffect(() => {
    fetchTools();
  }, [fetchTools]);

  const handleToolClick = (toolId: string) => {
    navigate(`/tools/${toolId}`);
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
      {frameless === true && (
        <Box sx={{ height: 40 }}>
          <OpBar />
        </Box>
      )}

      {/* 标题栏 */}
      <Box
        sx={{
          p: 2,
          borderBottom: 1,
          borderColor: "divider",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          ...dragRegionSx(frameless),
        }}
      >
        <Typography variant="h5" component="h1">
          运维工具箱
        </Typography>
      </Box>

      {/* 内容区域 */}
      <Box sx={{ flex: 1, overflow: "auto", p: 4 }}>
        <Grid container spacing={3} sx={{ justifyContent: "center" }}>
          {tools.map((tool) => (
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 3 }} key={tool.id}>
              <ToolCard
                tool={tool}
                onClick={() => handleToolClick(tool.id)}
              />
            </Grid>
          ))}
        </Grid>
      </Box>
    </Box>
  );
}
