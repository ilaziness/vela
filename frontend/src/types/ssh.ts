import type { ITheme } from "@xterm/xterm";

export enum ConnectionStatus {
  Connected = "connected",
  Disconnected = "disconnected",
  Connecting = "connecting",
}

export type {
  SessionLink as SSHLinkInfo,
  SessionTab as SSHTab,
  ConnectRequest,
} from "./session";

export { toConnectRequest, SessionProtocol } from "./session";

export interface Message {
  open: boolean;
  text: string;
  type: "error" | "success" | "info";
}

export interface MessageStore {
  message: Message;
  setClose: () => void;
  errorMessage: (message: string) => void;
  successMessage: (message: string) => void;
  infoMessage: (message: string) => void;
}

// 终端主题配置映射
export type TerminalThemes = {
  light: ITheme;
  dark: ITheme;
  eyeCare: ITheme;
};

// 应用主题类型定义
export type AppTheme = "light" | "dark" | "eyeCare";
