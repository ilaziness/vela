import React, { useState } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@mui/material";
import * as ConfigService from "../../bindings/github.com/ilaziness/vela/services/configservice";
import { AppService } from "../../bindings/github.com/ilaziness/vela/services";
import { t, tf } from "../i18n";
import { translateServiceError } from "../func/service";

type Props = {
  onUnlocked: () => void;
};

/** Startup hard gate: unlock or quit. Does not mount main app chrome behind it. */
const VaultUnlockGate: React.FC<Props> = ({ onUnlocked }) => {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const quit = () => {
    void AppService.MainWindowClose().catch(() => {});
  };

  const handleSubmit = async () => {
    if (!password.trim()) {
      setError(t("password.required"));
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await ConfigService.Unlock(password);
      onUnlocked();
    } catch (err) {
      setError(translateServiceError(err) || tf("password.unlock_failed", String(err)));
      setSubmitting(false);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Enter") {
      void handleSubmit();
    }
  };

  return (
    <Box
      sx={{
        height: "100%",
        width: "100%",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        bgcolor: "background.default",
      }}
    >
      <Dialog
        open
        maxWidth="sm"
        fullWidth
        onClose={(_, reason) => {
          // Hard gate: ignore backdrop/escape; only Cancel button quits.
          if (reason === "backdropClick" || reason === "escapeKeyDown") {
            return;
          }
        }}
      >
        <DialogTitle>{t("password.unlock_title")}</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {t("password.unlock_message")}
          </Typography>
          {error && (
            <Typography variant="body2" color="error" sx={{ mb: 2 }}>
              {error}
            </Typography>
          )}
          <TextField
            autoFocus
            margin="dense"
            label={t("password.label")}
            type="password"
            fullWidth
            variant="outlined"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={t("password.placeholder")}
            disabled={submitting}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={quit} disabled={submitting}>
            {t("common.quit")}
          </Button>
          <Button
            onClick={() => void handleSubmit()}
            variant="contained"
            disabled={submitting}
          >
            {t("common.ok")}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default VaultUnlockGate;
