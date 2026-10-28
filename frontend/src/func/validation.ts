/**
 * Tunnel / address validation helpers (messages via i18n catalog).
 */

import { t } from "../i18n";

export const validateIpPort = (addr: string): { valid: boolean; error: string } => {
  if (!addr.trim()) {
    return { valid: false, error: t("validation.addr_required") };
  }

  const parts = addr.split(":");
  if (parts.length !== 2) {
    return { valid: false, error: t("validation.addr_ip_port") };
  }

  const [ip, portStr] = parts;

  const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
  if (!ipRegex.test(ip)) {
    return { valid: false, error: t("validation.ip_invalid") };
  }

  const ipParts = ip.split(".");
  for (const part of ipParts) {
    const num = Number.parseInt(part, 10);
    if (num < 0 || num > 255) {
      return { valid: false, error: t("validation.ip_octet_range") };
    }
  }

  const port = Number.parseInt(portStr, 10);
  if (Number.isNaN(port) || port < 1 || port > 65535) {
    return { valid: false, error: t("validation.port_range") };
  }

  return { valid: true, error: "" };
};

export const validateRemoteAddr = (
  addr: string,
): { valid: boolean; error: string } => {
  if (!addr.trim()) {
    return { valid: false, error: t("validation.remote_addr_required") };
  }

  const parts = addr.split(":");
  if (parts.length !== 2) {
    return { valid: false, error: t("validation.addr_host_port") };
  }

  const [host, portStr] = parts;
  if (!host.trim()) {
    return { valid: false, error: t("validation.host_required") };
  }

  const port = Number.parseInt(portStr, 10);
  if (Number.isNaN(port) || port < 1 || port > 65535) {
    return { valid: false, error: t("validation.port_range") };
  }

  return { valid: true, error: "" };
};

export const validatePort = (portStr: string): { valid: boolean; error: string } => {
  if (!portStr.trim()) {
    return { valid: false, error: t("validation.port_required") };
  }

  const port = Number.parseInt(portStr, 10);
  if (Number.isNaN(port) || port < 1 || port > 65535) {
    return { valid: false, error: t("validation.port_range") };
  }

  return { valid: true, error: "" };
};

export interface TunnelFormData {
  localAddr: string;
  remoteAddr: string;
  remotePort: string;
}

export interface TunnelFormErrors {
  localAddr: string;
  remoteAddr: string;
  remotePort: string;
}

export type TunnelType = "local" | "remote" | "dynamic";

export const calculateTunnelFormErrors = (
  tunnelType: TunnelType,
  formData: TunnelFormData,
): TunnelFormErrors => {
  const newErrors: TunnelFormErrors = {
    localAddr: "",
    remoteAddr: "",
    remotePort: "",
  };

  const localResult = validateIpPort(formData.localAddr);
  newErrors.localAddr = localResult.error;

  if (tunnelType === "local") {
    const remoteResult = validateRemoteAddr(formData.remoteAddr);
    newErrors.remoteAddr = remoteResult.error;
  }

  if (tunnelType === "remote") {
    const portResult = validatePort(formData.remotePort);
    newErrors.remotePort = portResult.error;
  }

  return newErrors;
};

export const isTunnelFormValid = (
  tunnelType: TunnelType,
  formData: TunnelFormData,
): boolean => {
  const errors = calculateTunnelFormErrors(tunnelType, formData);
  return !errors.localAddr && !errors.remoteAddr && !errors.remotePort;
};

export const getTunnelTypeName = (tunnelType: TunnelType): string => {
  switch (tunnelType) {
    case "local":
      return t("validation.tunnel_local");
    case "remote":
      return t("validation.tunnel_remote");
    case "dynamic":
      return t("validation.tunnel_dynamic");
    default:
      return t("validation.tunnel");
  }
};

export const getLocalAddrPlaceholder = (tunnelType: TunnelType): string => {
  return tunnelType === "remote"
    ? t("validation.example_local_target")
    : t("validation.example_local_listen");
};

export const getLocalAddrHelperText = (
  tunnelType: TunnelType,
  error: string,
): string => {
  if (error) return error;
  return tunnelType === "remote"
    ? t("validation.helper_local_target")
    : t("validation.helper_local_listen");
};

export const getRemoteAddrHelperText = (error: string): string => {
  return error || t("validation.helper_remote_addr");
};

export const getRemotePortHelperText = (error: string): string => {
  return error || t("validation.helper_remote_port");
};
