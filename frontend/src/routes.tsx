import type { ComponentType } from "react";
import { Box, Typography, Button } from "@mui/material";
import { createHashRouter, useRouteError, isRouteErrorResponse } from "react-router";
import App from "./pages/App.tsx";
import SubMainWindow from "./pages/SubMainWindow";
import Loading from "./components/Loading";
import { t } from "./i18n";

/** Shown while a lazy route chunk is loading (e.g. setting/command/tools windows). */
function RouteLoadingFallback() {
  return <Loading message={t("common.loading")} />;
}

function RouteErrorFallback() {
  const error = useRouteError();
  const message = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText}`
    : error instanceof Error
      ? error.message
      : t("error.page_load_failed");

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 2,
        p: 3,
      }}
    >
      <Typography variant="h6">{t("error.page_load_failed")}</Typography>
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
      <Button variant="outlined" onClick={() => window.location.reload()}>
        {t("common.reload")}
      </Button>
    </Box>
  );
}

const lazyRoute = (importer: () => Promise<{ default: ComponentType }>) => ({
  HydrateFallback: RouteLoadingFallback,
  ErrorBoundary: RouteErrorFallback,
  lazy: async () => {
    const { default: Component } = await importer();
    return { Component };
  },
});

export default createHashRouter([
  {
    path: "/",
    Component: App,
  },
  {
    path: "/setting",
    ...lazyRoute(() => import("./pages/Setting")),
  },
  {
    path: "/command",
    ...lazyRoute(() => import("./pages/Command")),
  },
  {
    path: "/tools",
    ...lazyRoute(() => import("./pages/Tools")),
  },
  {
    path: "/tools/:toolId",
    ...lazyRoute(() => import("./pages/ToolDetail")),
  },
  {
    // Secondary main window: keep eager to avoid a blank webview on open.
    path: "/submainwindow",
    Component: SubMainWindow,
  },
]);
