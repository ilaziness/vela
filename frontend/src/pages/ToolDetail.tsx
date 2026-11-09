import React, { Suspense } from "react";
import { useParams } from "react-router";
import { Box, CircularProgress, Typography } from "@mui/material";
import ToolLayout from "../components/tools/ToolLayout";
import { tf } from "../i18n";

const toolComponents: Record<
  string,
  React.LazyExoticComponent<React.ComponentType>
> = {
  "json-yaml": React.lazy(() => import("../components/tools/JSONYAMLTool")),
  timestamp: React.lazy(() => import("../components/tools/TimestampTool")),
  encoder: React.lazy(() => import("../components/tools/EncoderTool")),
  hash: React.lazy(() => import("../components/tools/HashTool")),
  regex: React.lazy(() => import("../components/tools/RegexTool")),
  cron: React.lazy(() => import("../components/tools/CronTool")),
  cidr: React.lazy(() => import("../components/tools/CIDRTool")),
  "port-check": React.lazy(() => import("../components/tools/PortCheckTool")),
  jwt: React.lazy(() => import("../components/tools/JWTTool")),
  random: React.lazy(() => import("../components/tools/RandomTool")),
  "base-convert": React.lazy(() => import("../components/tools/BaseConvertTool")),
  "text-diff": React.lazy(() => import("../components/tools/TextDiffTool")),
  chmod: React.lazy(() => import("../components/tools/ChmodTool")),
};

export default function ToolDetail() {
  const { toolId } = useParams<{ toolId: string }>();
  const ToolComponent = toolId ? toolComponents[toolId] : null;

  return (
    <ToolLayout>
      {ToolComponent ? (
        <Suspense
          fallback={
            <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
              <CircularProgress size={28} />
            </Box>
          }
        >
          <ToolComponent />
        </Suspense>
      ) : (
        <Box sx={{ textAlign: "center", mt: 8 }}>
          <Typography variant="h6" color="text.secondary">
            {tf("tools.unknown", toolId ?? "")}
          </Typography>
        </Box>
      )}
    </ToolLayout>
  );
}
