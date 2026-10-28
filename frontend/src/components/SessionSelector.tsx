import React from "react";
import { Autocomplete, Box, TextField, Checkbox, IconButton } from "@mui/material";
import { Refresh } from "@mui/icons-material";
import { SSHTunnelSession } from "../types/command";
import { t } from "../i18n";

interface SessionSelectorProps {
  sessions: SSHTunnelSession[];
  selectedSessions: SSHTunnelSession[];
  onChange: (newValue: SSHTunnelSession[]) => void;
  onRefresh?: () => void; // 可选的刷新回调
}

const SessionSelector: React.FC<SessionSelectorProps> = ({
  sessions,
  selectedSessions,
  onChange,
  onRefresh,
}) => {
  return (
    <Box sx={{ display: "flex", alignItems: "flex-start", gap: 0.5, my: 1 }}>
      <Autocomplete
        multiple
        options={sessions}
        getOptionLabel={(option) => option.clientKey}
        isOptionEqualToValue={(option, value) => option.id === value.id}
        value={selectedSessions}
        onChange={(_, newValue) => onChange(newValue)}
        renderInput={(params) => (
          <TextField
            {...params}
            label={t("command.select_session_label")}
            placeholder={t("command.select_session_placeholder")}
            size="small"
          />
        )}
        renderOption={(props, option) => {
          const { key, ...optionProps } = props;
          const checked = selectedSessions.some((session) => session.id === option.id);
          return (
            <li key={key} {...optionProps}>
              <Checkbox checked={checked} />
              {option.clientKey}
            </li>
          );
        }}
        sx={{ flex: 1 }}
      />
      {onRefresh && (
        <IconButton
          onClick={onRefresh}
          size="small"
          title={t("command.refresh_sessions")}
          sx={{ mt: 1 }}
        >
          <Refresh fontSize="small" />
        </IconButton>
      )}
    </Box>
  );
};

export default SessionSelector;
