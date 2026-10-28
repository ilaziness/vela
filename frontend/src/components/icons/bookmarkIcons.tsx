import React from "react";
import {
  Box,
  IconButton,
  Popover,
  Tooltip,
  alpha,
  SvgIconProps,
} from "@mui/material";
import { t } from "../../i18n";
import {
  BookmarkBorderOutlined,
  FolderOutlined,
  Folder,
  FolderSpecial,
  FolderShared,
  FolderCopy,
  Cloud,
  CloudQueue,
  CloudDone,
  Dns,
  DnsOutlined,
  Storage,
  Computer,
  Lan,
  Router,
  Security,
  Work,
  Home,
  Star,
  Category,
  Devices,
  Hub,
  Memory,
  Public,
  SettingsEthernet,
  Terminal,
  Business,
  Groups,
  AccountTree,
  Inventory,
  Dataset,
  VpnKey,
  Lock,
  Speed,
  Build,
  Code,
  DataObject,
  Layers,
  Widgets,
  Dashboard,
  Domain,
  Factory,
  Apartment,
  Backup,
  Sync,
  Wifi,
  Cable,
  DeveloperBoard,
  Engineering,
} from "@mui/icons-material";
import {
  SiApple,
  SiMacos,
  SiLinux,
  SiUbuntu,
  SiDebian,
  SiCentos,
  SiFedora,
  SiArchlinux,
  SiFreebsd,
  SiOpenbsd,
  SiNetbsd,
  SiRedhat,
  SiRockylinux,
  SiAlmalinux,
  SiOpensuse,
  SiSuse,
  SiAlpinelinux,
  SiKalilinux,
  SiLinuxmint,
  SiManjaro,
  SiGentoo,
  SiNixos,
  SiAndroid,
  SiRaspberrypi,
  SiPopos,
  SiElementary,
  SiVoidlinux,
  SiArtixlinux,
  SiAsahilinux,
  SiDocker,
  SiProxmox,
  SiVmware,
  SiQemu,
  SiVirtualbox,
  SiKubernetes,
  SiKubuntu,
  SiLubuntu,
  SiXubuntu,
} from "react-icons/si";
import { FaWindows } from "react-icons/fa";
import type { IconType } from "react-icons";

export const OS_ICON_IDS = [
  "windows",
  "macos",
  "apple",
  "linux",
  "ubuntu",
  "debian",
  "centos",
  "fedora",
  "redhat",
  "rocky",
  "alma",
  "opensuse",
  "suse",
  "archlinux",
  "manjaro",
  "artixlinux",
  "asahilinux",
  "gentoo",
  "alpine",
  "nixos",
  "voidlinux",
  "linuxmint",
  "popos",
  "elementary",
  "kali",
  "kubuntu",
  "lubuntu",
  "xubuntu",
  "freebsd",
  "openbsd",
  "netbsd",
  "android",
  "raspberrypi",
  "docker",
  "kubernetes",
  "proxmox",
  "vmware",
  "qemu",
  "virtualbox",
] as const;

export type OsIconId = (typeof OS_ICON_IDS)[number];

const OS_ICON_MAP: Record<OsIconId, IconType> = {
  windows: FaWindows,
  macos: SiMacos,
  apple: SiApple,
  linux: SiLinux,
  ubuntu: SiUbuntu,
  debian: SiDebian,
  centos: SiCentos,
  fedora: SiFedora,
  redhat: SiRedhat,
  rocky: SiRockylinux,
  alma: SiAlmalinux,
  opensuse: SiOpensuse,
  suse: SiSuse,
  archlinux: SiArchlinux,
  manjaro: SiManjaro,
  artixlinux: SiArtixlinux,
  asahilinux: SiAsahilinux,
  gentoo: SiGentoo,
  alpine: SiAlpinelinux,
  nixos: SiNixos,
  voidlinux: SiVoidlinux,
  linuxmint: SiLinuxmint,
  popos: SiPopos,
  elementary: SiElementary,
  kali: SiKalilinux,
  kubuntu: SiKubuntu,
  lubuntu: SiLubuntu,
  xubuntu: SiXubuntu,
  freebsd: SiFreebsd,
  openbsd: SiOpenbsd,
  netbsd: SiNetbsd,
  android: SiAndroid,
  raspberrypi: SiRaspberrypi,
  docker: SiDocker,
  kubernetes: SiKubernetes,
  proxmox: SiProxmox,
  vmware: SiVmware,
  qemu: SiQemu,
  virtualbox: SiVirtualbox,
};

export const GROUP_ICON_IDS = [
  "Folder",
  "FolderSpecial",
  "FolderShared",
  "FolderCopy",
  "Cloud",
  "CloudQueue",
  "CloudDone",
  "Dns",
  "DnsOutlined",
  "Storage",
  "Computer",
  "Devices",
  "DeveloperBoard",
  "Lan",
  "Router",
  "Wifi",
  "Cable",
  "SettingsEthernet",
  "Hub",
  "Memory",
  "Public",
  "Domain",
  "Apartment",
  "Business",
  "Factory",
  "Security",
  "VpnKey",
  "Lock",
  "Terminal",
  "Code",
  "DataObject",
  "Dataset",
  "Inventory",
  "Backup",
  "Sync",
  "AccountTree",
  "Layers",
  "Widgets",
  "Dashboard",
  "Category",
  "Groups",
  "Work",
  "Home",
  "Star",
  "Speed",
  "Build",
  "Engineering",
] as const;

export type GroupIconId = (typeof GROUP_ICON_IDS)[number];

const GROUP_ICON_MAP: Record<
  GroupIconId,
  React.ComponentType<SvgIconProps>
> = {
  Folder,
  FolderSpecial,
  FolderShared,
  FolderCopy,
  Cloud,
  CloudQueue,
  CloudDone,
  Dns,
  DnsOutlined,
  Storage,
  Computer,
  Devices,
  DeveloperBoard,
  Lan,
  Router,
  Wifi,
  Cable,
  SettingsEthernet,
  Hub,
  Memory,
  Public,
  Domain,
  Apartment,
  Business,
  Factory,
  Security,
  VpnKey,
  Lock,
  Terminal,
  Code,
  DataObject,
  Dataset,
  Inventory,
  Backup,
  Sync,
  AccountTree,
  Layers,
  Widgets,
  Dashboard,
  Category,
  Groups,
  Work,
  Home,
  Star,
  Speed,
  Build,
  Engineering,
};

type IconSize = "small" | "medium" | number;

function sizeToPx(size: IconSize = "small"): number {
  if (typeof size === "number") return size;
  return size === "medium" ? 20 : 16;
}

export function BookmarkIconView({
  icon,
  fontSize = "small",
  sx,
  color = "primary.main",
}: {
  icon?: string | null;
  fontSize?: IconSize;
  sx?: object;
  color?: string;
}) {
  const px = sizeToPx(fontSize);
  const OsComp =
    icon && icon in OS_ICON_MAP ? OS_ICON_MAP[icon as OsIconId] : null;
  if (OsComp) {
    return (
      <Box
        component={OsComp}
        sx={{ width: px, height: px, flexShrink: 0, color, ...sx }}
      />
    );
  }
  return (
    <BookmarkBorderOutlined
      sx={{ fontSize: px, flexShrink: 0, color, ...sx }}
    />
  );
}

export function GroupIconView({
  icon,
  fontSize = "small",
  sx,
  color = "warning.main",
}: {
  icon?: string | null;
  fontSize?: IconSize;
  sx?: object;
  color?: string;
}) {
  const px = sizeToPx(fontSize);
  const Comp =
    icon && icon in GROUP_ICON_MAP
      ? GROUP_ICON_MAP[icon as GroupIconId]
      : FolderOutlined;
  return <Comp sx={{ fontSize: px, flexShrink: 0, color, ...sx }} />;
}

function PickerGrid({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: "repeat(6, 1fr)",
        gap: 0.5,
        p: 1,
        maxWidth: 280,
        maxHeight: 320,
        overflowY: "auto",
      }}
    >
      {children}
    </Box>
  );
}

function PickerCell({
  selected,
  title,
  onClick,
  children,
}: {
  selected: boolean;
  title: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip title={title}>
      <IconButton
        size="small"
        onClick={onClick}
        sx={{
          borderRadius: 1,
          border: 1,
          borderColor: selected ? "primary.main" : "transparent",
          bgcolor: (theme) =>
            selected ? alpha(theme.palette.primary.main, 0.12) : "transparent",
        }}
      >
        {children}
      </IconButton>
    </Tooltip>
  );
}

type PickerProps = {
  value: string;
  onChange: (icon: string) => void;
  anchorEl: HTMLElement | null;
  open: boolean;
  onClose: () => void;
};

export function OsIconPicker({
  value,
  onChange,
  anchorEl,
  open,
  onClose,
}: PickerProps) {
  const pick = (id: string) => {
    onChange(id);
    onClose();
  };
  return (
    <Popover
      open={open}
      anchorEl={anchorEl}
      onClose={onClose}
      anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
    >
      <PickerGrid>
        <PickerCell
          selected={!value}
          title={t("common.default")}
          onClick={() => pick("")}
        >
          <BookmarkBorderOutlined sx={{ fontSize: 18 }} />
        </PickerCell>
        {OS_ICON_IDS.map((id) => {
          const Comp = OS_ICON_MAP[id];
          return (
            <PickerCell
              key={id}
              selected={value === id}
              title={id}
              onClick={() => pick(id)}
            >
              <Box component={Comp} sx={{ width: 18, height: 18 }} />
            </PickerCell>
          );
        })}
      </PickerGrid>
    </Popover>
  );
}

export function GroupIconPicker({
  value,
  onChange,
  anchorEl,
  open,
  onClose,
}: PickerProps) {
  const pick = (id: string) => {
    onChange(id);
    onClose();
  };
  return (
    <Popover
      open={open}
      anchorEl={anchorEl}
      onClose={onClose}
      anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
    >
      <PickerGrid>
        <PickerCell
          selected={!value}
          title={t("common.default")}
          onClick={() => pick("")}
        >
          <FolderOutlined sx={{ fontSize: 18 }} />
        </PickerCell>
        {GROUP_ICON_IDS.map((id) => {
          const Comp = GROUP_ICON_MAP[id];
          return (
            <PickerCell
              key={id}
              selected={value === id}
              title={id}
              onClick={() => pick(id)}
            >
              <Comp sx={{ fontSize: 18 }} />
            </PickerCell>
          );
        })}
      </PickerGrid>
    </Popover>
  );
}
