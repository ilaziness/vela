import React, { useEffect, useMemo, useState } from "react";
import ReactDOM from "react-dom/client";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { RouterProvider } from "react-router/dom";
import { Events } from "@wailsio/runtime";
import routes from "./routes.tsx";
import { createAppTheme } from "./theme";
import { ReadConfig } from "../bindings/github.com/ilaziness/vela/services/configservice";
import { Config } from "../bindings/github.com/ilaziness/vela/services";
import useTerminalStore from "./stores/terminal";
import type { AppTheme } from "./types/ssh";

const EVENT_APPEARANCE = "eventAppearance";

interface Appearance {
  Font?: string;
  FontSize?: number;
}

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

let initialFont = "";
let initialFontSize = 14;

try {
  const config = await ReadConfig();
  if (config) {
    applyTerminalSettings(config.Terminal);
    syncTerminalTheme(config.General?.Theme);
    initialFont = config.General?.Font ?? "";
    if (config.General?.FontSize && config.General.FontSize > 0) {
      initialFontSize = config.General.FontSize;
    }
  }
} catch (err) {
  console.error("ReadConfig failed:", err);
}

function Root() {
  const [font, setFont] = useState(initialFont);
  const [fontSize, setFontSize] = useState(initialFontSize);

  useEffect(() => {
    const unsubscribe = Events.On(
      EVENT_APPEARANCE,
      (event: { data?: Appearance }) => {
        const data = event?.data;
        if (!data) {
          return;
        }
        setFont(data.Font ?? "");
        if (data.FontSize && data.FontSize > 0) {
          setFontSize(data.FontSize);
        }
      },
    );
    return () => {
      unsubscribe();
    };
  }, []);

  const theme = useMemo(
    () => createAppTheme(font, fontSize),
    [font, fontSize],
  );

  return (
    <ThemeProvider theme={theme} defaultMode="dark">
      <CssBaseline />
      <RouterProvider router={routes} />
    </ThemeProvider>
  );
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
