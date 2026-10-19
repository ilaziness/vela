import React from "react";
import { Autocomplete, Box, TextField, Checkbox, IconButton } from "@mui/material";
import { Refresh } from "@mui/icons-material";
import { SSHTunnelSession } from "../types/command";

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
            label="选择要发送到的 SSH 会话"
            placeholder="选择会话"
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
          title="刷新会话列表"
          sx={{ mt: 1 }}
        >
          <Refresh fontSize="small" />
        </IconButton>
      )}
    </Box>
  );
};

export default SessionSelector;
