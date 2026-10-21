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
const EVENT_TERMINAL = "eventTerminal";

interface Appearance {
  Font?: string;
  FontSize?: number;
}

const VALID_THEMES: AppTheme[] = ["light", "dark", "eyeCare"];

/** Apply terminal prefs from config. Booleans must be assigned even when false. */
function applyTerminalSettings(t: Config["Terminal"] | undefined | null): void {
  if (!t) {
    return;
  }
  useTerminalStore.getState().applyFromConfig({
    fontFamily: t.fontFamily || undefined,
    fontSize: t.fontSize > 0 ? t.fontSize : undefined,
    lineHeight: t.lineHeight > 0 ? t.lineHeight : undefined,
    scrollback: t.scrollback > 0 ? t.scrollback : undefined,
    bell: t.bell,
    copyOnSelect: t.copyOnSelect,
    rightClickPaste: t.rightClickPaste,
  });
}

function normalizeTheme(theme: string | undefined): AppTheme {
  if (theme && VALID_THEMES.includes(theme as AppTheme)) {
    return theme as AppTheme;
  }
  return "dark";
}

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

  useEffect(() => {
    const unsubscribe = Events.On(
      EVENT_TERMINAL,
      (event: { data?: Config["Terminal"] }) => {
        applyTerminalSettings(event?.data);
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
