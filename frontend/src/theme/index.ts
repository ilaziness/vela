import { createColorScheme, createTheme } from "@mui/material/styles";
import type { Theme } from "@mui/material/styles";
import type {} from "@mui/x-chat/themeAugmentation";
import type {} from "@mui/x-charts/themeAugmentation";
import light from "./light";
import dark from "./dark";
import eyeCare from "./eyeCare";
import { darkTokens, eyeCareTokens, lightTokens } from "./paletteTokens";
import { CODE_FONT_FAMILY, CODE_TYPOGRAPHY } from "./typography";
import { AppTheme } from "../types/ssh";

declare module "@mui/material/styles" {
  interface ColorSchemeOverrides {
    eyeCare: true;
  }
}

/** MUI X Chat markdown 使用原生 <a>，需显式设色（暗色底上浏览器默认蓝几乎不可见） */
function chatMarkdownLinkStyles(theme: Theme, onPrimaryBubble = false) {
  if (onPrimaryBubble) {
    return {
      color: "#FFFFFF",
      textDecoration: "underline",
      textDecorationColor: "rgba(255, 255, 255, 0.5)",
      "&:hover": { color: "rgba(255, 255, 255, 0.9)" },
      "&:visited": { color: "rgba(255, 255, 255, 0.85)" },
    };
  }

  const isDark = theme.palette.mode === "dark";
  const isEyeCare = theme.palette.background.default === eyeCareTokens.bgDefault;
  const linkColor = isDark
    ? theme.palette.primary.light
    : isEyeCare
      ? eyeCareTokens.secondary
      : theme.palette.primary.main;
  const hoverColor = isDark
    ? "#A5D6FF"
    : isEyeCare
      ? eyeCareTokens.secondaryLight
      : theme.palette.primary.dark;

  return {
    color: linkColor,
    textDecoration: "underline",
    textDecorationColor: isDark
      ? "rgba(121, 192, 255, 0.45)"
      : isEyeCare
        ? "rgba(13, 148, 136, 0.35)"
        : "rgba(37, 99, 235, 0.35)",
    "&:hover": { color: hoverColor },
    "&:visited": { color: hoverColor },
  };
}

/**
 * 主题选项配置
 */
export interface ThemeOption {
  value: AppTheme;
  label: string;
  mode: "light" | "dark";
  color: string;
}

export const ThemeOptions: ThemeOption[] = [
  {
    value: "light",
    label: "日间",
    mode: "light",
    color: lightTokens.primary,
  },
  {
    value: "dark",
    label: "夜间",
    mode: "dark",
    color: darkTokens.secondary,
  },
  {
    value: "eyeCare",
    label: "护眼",
    mode: "light",
    color: eyeCareTokens.primary,
  },
];

const SYSTEM_FONT_FAMILY = [
  "-apple-system",
  "BlinkMacSystemFont",
  '"Segoe UI"',
  "Roboto",
  '"Helvetica Neue"',
  "Arial",
  "sans-serif",
  '"Apple Color Emoji"',
  '"Segoe UI Emoji"',
  '"Segoe UI Symbol"',
].join(",");

export function sanitizeAppFont(font: string): string {
  return font.replaceAll(/[;{}<>\\'"\u0000-\u001f]/g, "").trim();
}

function quoteFontName(name: string): string {
  if (/\s/.test(name)) {
    return `"${name}"`;
  }
  return name;
}

export function appFontFamily(font: string): string {
  const trimmed = sanitizeAppFont(font);
  if (!trimmed) {
    return SYSTEM_FONT_FAMILY;
  }
  const families = trimmed
    .split(",")
    .map((part) => quoteFontName(part.trim()))
    .filter((part) => part !== "");
  if (families.length === 0) {
    return SYSTEM_FONT_FAMILY;
  }
  return [...families, SYSTEM_FONT_FAMILY].join(",");
}

/** Theme-token scrollbars (CSS vars switch with color scheme). */
function appScrollbarColors(theme: Theme) {
  const palette = (theme.vars || theme).palette;
  return {
    thumb: palette.text.disabled,
    track: palette.background.default,
  };
}

/**
 * SSH GUI 客户端主题配置
 */
export function createAppTheme(font = "", fontSize = 14) {
  return createTheme({
  colorSchemes: {
    light,
    dark: createColorScheme(dark),
    eyeCare: createColorScheme(eyeCare),
  },

  components: {
    MuiCssBaseline: {
      styleOverrides: (theme: Theme) => {
        const { thumb, track } = appScrollbarColors(theme);
        return {
          html: {
            fontSize,
            scrollbarWidth: "thin",
            scrollbarColor: `${thumb} ${track}`,
          },
        };
      },
    },
    MuiChatMessageList: {
      styleOverrides: {
        scroller: ({ theme: t }) => {
          const { thumb, track } = appScrollbarColors(t);
          return {
            scrollbarColor: `${thumb} ${track}`,
          };
        },
      },
    },
    MuiChatBox: {
      styleOverrides: {
        threadPane: ({ theme: t }) => ({
          backgroundColor: t.palette.background.default,
        }),
      },
    },
    MuiChatCodeBlock: {
      styleOverrides: {
        languageLabel: {
          fontFamily: CODE_FONT_FAMILY,
        },
        pre: {
          fontFamily: CODE_FONT_FAMILY,
          lineHeight: CODE_TYPOGRAPHY.lineHeight,
        },
        code: {
          fontFamily: CODE_FONT_FAMILY,
          fontSize: "0.8125rem",
          lineHeight: CODE_TYPOGRAPHY.lineHeight,
        },
      },
    },
    MuiChatMessage: {
      styleOverrides: {
        bubble: ({ theme: t, ownerState }) => ({
          borderRadius: 12,
          ...(ownerState?.role === "assistant" &&
            t.palette.background.default === eyeCareTokens.bgDefault && {
              border: `1px solid ${eyeCareTokens.assistantBubbleBorder}`,
            }),
          "& a": chatMarkdownLinkStyles(t, ownerState?.role === "user"),
          "& code": CODE_TYPOGRAPHY,
          "& pre": {
            fontFamily: CODE_FONT_FAMILY,
            lineHeight: CODE_TYPOGRAPHY.lineHeight,
          },
        }),
        content: ({ theme: t }) => ({
          "& a": chatMarkdownLinkStyles(t),
          "& code": CODE_TYPOGRAPHY,
          "& pre": {
            fontFamily: CODE_FONT_FAMILY,
            lineHeight: CODE_TYPOGRAPHY.lineHeight,
          },
        }),
      },
    },
  },

    typography: {
      fontFamily: appFontFamily(font),
      fontSize,
      htmlFontSize: fontSize,
    },
  });
}
