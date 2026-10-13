import "@wailsio/runtime";
import React from "react";
import ReactDOM from "react-dom/client";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { RouterProvider } from "react-router/dom";
import routes from "./routes.tsx";
import theme from "./theme";
import { ReadConfig } from "../bindings/github.com/ilaziness/vela/services/configservice";
import { Config } from "../bindings/github.com/ilaziness/vela/services";
import useTerminalStore from "./stores/terminal";
import type { AppTheme } from "./types/ssh";

const VALID_THEMES: AppTheme[] = ["light", "dark", "eyeCare"];

// 提取终端设置应用函数，降低认知复杂度
function applyTerminalSettings(t: Config["Terminal"]): void {
  const updates: Partial<{
    fontFamily: string;
    fontSize: number;
    lineHeight: number;
  }> = {};

  if (t?.fontFamily) {
    updates.fontFamily = t.fontFamily;
  }
  if (t?.fontSize && t.fontSize > 0) {
    updates.fontSize = t.fontSize;
  }
  if (t?.lineHeight && t.lineHeight > 0) {
    updates.lineHeight = t.lineHeight;
  }

  if (Object.keys(updates).length > 0) {
    useTerminalStore.setState(updates as any);
  }
}

// 验证主题值并回退到默认值
function normalizeTheme(theme: string | undefined): AppTheme {
  if (theme && VALID_THEMES.includes(theme as AppTheme)) {
    return theme as AppTheme;
  }
  return "dark";
}

// 提取主题同步函数
function syncTerminalTheme(globalTheme: string | undefined): void {
  const validTheme = normalizeTheme(globalTheme);
  useTerminalStore.getState().syncWithGlobalTheme(validTheme);
}

try {
  const config = await ReadConfig();
  if (config) {
    applyTerminalSettings(config.Terminal);
    syncTerminalTheme(config.General?.Theme);
  }
} catch (err) {
  console.error("ReadConfig failed:", err);
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider theme={theme} defaultMode="dark">
      <CssBaseline />
      <RouterProvider router={routes} />
    </ThemeProvider>
  </React.StrictMode>,
);
