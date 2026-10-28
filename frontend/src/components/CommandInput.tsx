import React from "react";
import { Box, TextField, Button } from "@mui/material";
import { t } from "../i18n";

interface CommandInputProps {
  inputCommand: string;
  onChange: (value: string) => void;
  onKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  onSend: () => void;
}

const CommandInput: React.FC<CommandInputProps> = ({
  inputCommand,
  onChange,
  onKeyDown,
  onSend,
}) => {
  return (
    <>
      <TextField
        multiline
        rows={3}
        value={inputCommand}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => onKeyDown(e as unknown as React.KeyboardEvent<HTMLTextAreaElement>)}
        placeholder={t("command.input_placeholder")}
        helperText={t("command.input_helper")}
        fullWidth
        variant="outlined"
        slotProps={{
          htmlInput: {
            autoComplete: "off",
          },
        }}
      />
      <Box sx={{ mt: 1, display: "flex", justifyContent: "flex-end" }}>
        <Button sx={{ px: 5 }} variant="contained" onClick={onSend}>
          {t("command.send")}
        </Button>
      </Box>
    </>
  );
};

export default CommandInput;
