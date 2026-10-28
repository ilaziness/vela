import React from "react";
import {
  Box,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Divider,
  Typography,
} from "@mui/material";
import { t, useLanguage } from "../../i18n";

export type SettingsTab =
  | "general"
  | "terminal"
  | "ssh"
  | "sync"
  | "ai"
  | "keys"
  | "knownHosts"
  | "keybindings"
  | "about";

interface NavItem {
  key: SettingsTab;
  labelKey: string;
}

const navItems: NavItem[] = [
  { key: "general", labelKey: "settings.nav.general" },
  { key: "terminal", labelKey: "settings.nav.terminal" },
  { key: "ssh", labelKey: "settings.nav.ssh" },
  { key: "sync", labelKey: "settings.nav.sync" },
  { key: "ai", labelKey: "settings.nav.ai" },
  { key: "keys", labelKey: "settings.nav.keys" },
  { key: "knownHosts", labelKey: "settings.nav.known_hosts" },
  { key: "keybindings", labelKey: "settings.nav.keybindings" },
  { key: "about", labelKey: "settings.nav.about" },
];

interface SettingsNavProps {
  activeTab: SettingsTab;
  onTabChange: (tab: SettingsTab) => void;
}

const SettingsNav: React.FC<SettingsNavProps> = ({ activeTab, onTabChange }) => {
  useLanguage();
  return (
    <Box>
      <Box sx={{ p: 2, pb: 1.5 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, fontSize: "1.2rem" }}>
          {t("window.settings")}
        </Typography>
      </Box>
      <Divider />
      <List sx={{ py: 1 }}>
        {navItems.map((item) => (
          <ListItem key={item.key} disablePadding>
            <ListItemButton
              selected={activeTab === item.key}
              onClick={() => onTabChange(item.key)}
              sx={{
                py: 1,
                px: 2,
                "&.Mui-selected": {
                  backgroundColor: "primary.main",
                  color: "primary.contrastText",
                  "&:hover": {
                    backgroundColor: "primary.dark",
                  },
                },
              }}
            >
              <ListItemText primary={t(item.labelKey)} />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Box>
  );
};

export default SettingsNav;
