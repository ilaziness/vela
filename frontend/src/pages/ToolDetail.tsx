import React, { Suspense } from "react";
import { useParams } from "react-router";
import { Box, CircularProgress, Typography } from "@mui/material";
import ToolLayout from "../components/ToolLayout";
import { tf } from "../i18n";

const toolComponents: Record<
  string,
  React.LazyExoticComponent<React.ComponentType>
> = {
  "json-yaml": React.lazy(() => import("../components/JSONYAMLTool")),
  timestamp: React.lazy(() => import("../components/TimestampTool")),
  encoder: React.lazy(() => import("../components/EncoderTool")),
  hash: React.lazy(() => import("../components/HashTool")),
  regex: React.lazy(() => import("../components/RegexTool")),
  cron: React.lazy(() => import("../components/CronTool")),
  cidr: React.lazy(() => import("../components/CIDRTool")),
  "port-check": React.lazy(() => import("../components/PortCheckTool")),
  jwt: React.lazy(() => import("../components/JWTTool")),
  random: React.lazy(() => import("../components/RandomTool")),
  "base-convert": React.lazy(() => import("../components/BaseConvertTool")),
  "text-diff": React.lazy(() => import("../components/TextDiffTool")),
  chmod: React.lazy(() => import("../components/ChmodTool")),
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
