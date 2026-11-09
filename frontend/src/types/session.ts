import { ConnectionStatus } from "./ssh";

export enum SessionProtocol {
  SSH = "ssh",
  Serial = "serial",
}

export interface SessionLink {
  protocol: SessionProtocol;
  linkID?: string;
  bookmarkID?: string;
  host?: string;
  port?: number;
  user?: string;
  password?: string;
  key?: string;
  keyPassword?: string;
  proxyJumpID?: string;
  certificate?: string;
  serialDevice?: string;
  baudRate?: number;
  dataBits?: number;
  parity?: string;
  stopBits?: number;
  flowControl?: string;
  charset?: string;
}

/** Matches services.ConnectRequest JSON fields (camelCase). */
export interface ConnectRequest {
  protocol: string;
  host: string;
  port: number;
  user: string;
  password: string;
  key: string;
  keyPassword: string;
  proxyJumpID: string;
  certificate: string;
  serialDevice: string;
  baudRate: number;
  dataBits: number;
  parity: string;
  stopBits: number;
  flowControl: string;
  charset: string;
}

/** Build ConnectRequest with stable defaults for optional SessionLink fields. */
export function toConnectRequest(li: SessionLink): ConnectRequest {
  const protocol = li.protocol;
  const key = li.key || "";
  return {
    protocol,
    host: li.host || "",
    port: li.port || 0,
    user: li.user || "",
    password: li.password || "",
    key,
    keyPassword: key ? li.keyPassword || "" : "",
    proxyJumpID: li.proxyJumpID || "",
    certificate: li.certificate || "",
    serialDevice: li.serialDevice || "",
    baudRate: li.baudRate || 0,
    dataBits: li.dataBits || 0,
    parity: li.parity || "",
    stopBits: li.stopBits || 0,
    flowControl: li.flowControl || "",
    charset: li.charset || "",
  };
}

export function sessionTabLabel(li: SessionLink): string {
  if (li.protocol === SessionProtocol.Serial) {
    const device = li.serialDevice || "?";
    return li.baudRate ? `${device} @ ${li.baudRate}` : device;
  }
  return `${li.user || "?"}@${li.host || "?"}:${li.port || 0}`;
}

export function isSSHProtocol(li?: SessionLink | null): boolean {
  return li?.protocol === SessionProtocol.SSH;
}

export function parseSessionProtocol(
  raw: string | undefined | null,
): SessionProtocol | null {
  if (raw === SessionProtocol.SSH || raw === SessionProtocol.Serial) {
    return raw;
  }
  return null;
}

export interface SessionTab {
  index: string;
  name: string;
  sessionLink?: SessionLink;
  connectionStatus?: ConnectionStatus;
}

/** @deprecated Use SessionLink */
export type SSHLinkInfo = SessionLink;
/** @deprecated Use SessionTab */
export type SSHTab = SessionTab;
