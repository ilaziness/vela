import { create } from "zustand";
import type { ITheme } from "@xterm/xterm";
import { AppTheme } from "../types/ssh";
import terminalThemes from "../theme/terminalThemes";
import { CODE_FONT_FAMILY } from "../theme/typography";

const VALID_THEMES: AppTheme[] = ["light", "dark", "eyeCare"];

function normalizeTheme(theme: AppTheme): AppTheme {
  return VALID_THEMES.includes(theme) ? theme : "dark";
}

export interface TerminalConfigSlice {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  scrollback: number;
  bell: boolean;
  copyOnSelect: boolean;
  rightClickPaste: boolean;
}

export interface TerminalSettings extends TerminalConfigSlice {
  theme: AppTheme;
  getCurrentTheme: () => ITheme;
  syncWithGlobalTheme: (globalTheme: AppTheme) => void;
  applyFromConfig: (cfg: Partial<TerminalConfigSlice>) => void;
}

const defaultSlice: TerminalConfigSlice = {
  fontFamily: CODE_FONT_FAMILY,
  fontSize: 14,
  lineHeight: 1,
  scrollback: 10000,
  bell: true,
  copyOnSelect: false,
  rightClickPaste: false,
};

export const useTerminalStore = create<TerminalSettings>((set, get) => ({
  ...defaultSlice,
  theme: "dark",
  getCurrentTheme: (): ITheme => {
    const { theme } = get();
    const validTheme = normalizeTheme(theme);
    return terminalThemes[validTheme];
  },
  syncWithGlobalTheme: (globalTheme: AppTheme) => {
    set(() => ({ theme: normalizeTheme(globalTheme) }));
  },
  applyFromConfig: (cfg: Partial<TerminalConfigSlice>) => {
    set((state) => ({
      fontFamily: cfg.fontFamily ?? state.fontFamily,
      fontSize: cfg.fontSize ?? state.fontSize,
      lineHeight: cfg.lineHeight ?? state.lineHeight,
      scrollback: cfg.scrollback ?? state.scrollback,
      bell: cfg.bell ?? state.bell,
      copyOnSelect: cfg.copyOnSelect ?? state.copyOnSelect,
      rightClickPaste: cfg.rightClickPaste ?? state.rightClickPaste,
    }));
  },
}));

export default useTerminalStore;
