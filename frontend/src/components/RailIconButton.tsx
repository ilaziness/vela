import { Box, Tooltip, type BoxProps } from "@mui/material";
import { alpha } from "@mui/material/styles";
import type { MouseEventHandler, ReactNode } from "react";
import { useKeybindingsStore } from "../stores/keybindings";

export function formatRailTooltip(title: string, keys?: string): string {
  return keys ? `${title} (${keys})` : title;
}

export interface RailIconButtonProps
  extends Omit<BoxProps<"button">, "component" | "children"> {
  children: ReactNode;
  active?: boolean;
}

/**
 * 左侧栏紧凑图标按钮。不用 MUI IconButton，避免默认 ~40px 点击区。
 * 图标尺寸锁像素，避免随应用字号 pxToRem 缩放。
 */
export default function RailIconButton({
  children,
  active = false,
  sx,
  ...rest
}: RailIconButtonProps) {
  return (
    <Box
      component="button"
      type="button"
      aria-pressed={active ? true : undefined}
      sx={[
        (theme) => ({
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          width: 30,
          height: 30,
          padding: 0,
          margin: 0,
          border: "none",
          borderRadius: "6px",
          appearance: "none",
          font: "inherit",
          cursor: "pointer",
          backgroundColor: active
            ? alpha(theme.palette.primary.main, 0.22)
            : "transparent",
          // Idle: text color with a light primary tint; active stays full primary.
          color: active
            ? theme.palette.primary.main
            : `color-mix(in srgb, ${theme.palette.text.primary} 62%, ${theme.palette.primary.main} 38%)`,
          "&:hover": {
            backgroundColor: alpha(
              theme.palette.primary.main,
              active ? 0.32 : 0.16,
            ),
            color: theme.palette.primary.main,
          },
          "&:focus-visible": {
            outline: `2px solid ${theme.palette.primary.main}`,
            outlineOffset: 1,
          },
          "& .MuiSvgIcon-root": {
            fontSize: "20px",
            width: "20px",
            height: "20px",
          },
        }),
        ...(Array.isArray(sx) ? sx : sx != null ? [sx] : []),
      ]}
      {...rest}
    >
      {children}
    </Box>
  );
}

interface RailMenuButtonProps {
  title: string;
  shortcutAction?: string;
  active?: boolean;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  children: ReactNode;
}

/** Tooltip + 可选快捷键括号提示，统一 placement=right。 */
export function RailMenuButton({
  title,
  shortcutAction,
  active,
  onClick,
  children,
}: RailMenuButtonProps) {
  const keys = useKeybindingsStore((s) =>
    shortcutAction
      ? (s.bindings.find((b) => b.action === shortcutAction)?.keys ?? "")
      : "",
  );
  const label = formatRailTooltip(title, keys || undefined);
  return (
    <Tooltip title={label} placement="right">
      <RailIconButton onClick={onClick} active={active} aria-label={label}>
        {children}
      </RailIconButton>
    </Tooltip>
  );
}
