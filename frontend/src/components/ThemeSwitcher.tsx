import React, { useState } from "react";
import { IconButton, Menu, ToggleButton, ToggleButtonGroup, Tooltip } from "@mui/material";
import { useColorScheme } from "@mui/material/styles";
import PaletteIcon from "@mui/icons-material/Palette";
import useTerminalStore from "../stores/terminal";
import { AppTheme } from "../types/ssh";
import { ThemeOptions } from "../theme";
import { railIconButtonSx } from "./railIconButtonSx";
import { SetTheme } from "../../bindings/github.com/ilaziness/vela/services/configservice";
import { LogService } from "../../bindings/github.com/ilaziness/vela/services";

const VALID_THEMES: AppTheme[] = ["light", "dark", "eyeCare"];

function isValidTheme(value: string): value is AppTheme {
  return VALID_THEMES.includes(value as AppTheme);
}

export default function ThemeSwitcher() {
  const {
    mode,
    setMode,
    colorScheme,
    setColorScheme,
    lightColorScheme,
    darkColorScheme,
  } = useColorScheme();

  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const getCurrentThemeValue = (): AppTheme => {
    let current: string;

    if (mode === "dark" && !colorScheme) {
      current = "dark";
    } else {
      current = (mode === "dark" ? darkColorScheme : lightColorScheme) || "dark";
    }

    return isValidTheme(current) ? current : "dark";
  };

  const handleThemeSelect = (value: AppTheme) => {
    // 保存主题配置
    SetTheme(value).catch((err) => {
      LogService.Error("Failed to save theme:" + err.message);
    });

    // 同步终端主题
    useTerminalStore.getState().syncWithGlobalTheme(value);

    const themeConfig = ThemeOptions.find((t) => t.value === value);
    if (!themeConfig) return;

    setMode(themeConfig.mode);
    if (themeConfig.mode === "light") {
      setColorScheme({ light: value });
    } else {
      setColorScheme({ dark: value });
    }

    handleMenuClose();
  };

  const currentTheme = getCurrentThemeValue();

  return (
    <>
      <Tooltip title="主题切换">
        <IconButton sx={railIconButtonSx} onClick={handleMenuOpen}>
          <PaletteIcon />
        </IconButton>
      </Tooltip>
      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleMenuClose}
        onClick={(e) => e.stopPropagation()}
        slotProps={{
          paper: {
            elevation: 8,
            sx: {
              overflow: "visible",
              mt: 1,
              p: 1,
            },
          },
        }}
      >
        <ToggleButtonGroup
          value={currentTheme}
          exclusive
          size="small"
          sx={{
            gap: 0.5,
            "& .MuiToggleButton-root": {
              border: "1px solid",
              borderColor: "divider",
              borderRadius: "4px !important",
              px: 1.5,
              py: 0.5,
              minWidth: "auto",
              fontSize: "0.875rem",
            },
            "& .Mui-selected": {
              borderColor: (theme) => theme.palette.primary.main,
              backgroundColor: (theme) => theme.palette.primary.main,
              color: (theme) => theme.palette.primary.contrastText,
              "&:hover": {
                backgroundColor: (theme) => theme.palette.primary.dark,
              },
            },
          }}
        >
          {ThemeOptions.map((option) => (
            <ToggleButton
              key={option.value}
              value={option.value}
              onClick={() => handleThemeSelect(option.value)}
            >
              {option.label}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Menu>
    </>
  );
}

