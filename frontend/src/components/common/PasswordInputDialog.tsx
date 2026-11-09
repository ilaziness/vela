import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Typography,
} from "@mui/material";
import { Events, Window } from "@wailsio/runtime";
import * as ConfigService from "../../../bindings/github.com/ilaziness/vela/services/configservice";
import { t, tf } from "../../i18n";
import { translateServiceError } from "../../func/service";

/** Runtime set-password prompt (encrypt/decrypt waiters). Cancel aborts wait only. */
const PasswordInputDialog: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let alive = true;
    const unsubscribe = Events.On("eventInputPassword", (event: any) => {
      void (async () => {
        let focused = true;
        try {
          focused = await Window.IsFocused();
        } catch (err) {
          console.error("window focus check failed", err);
        }
        if (!alive || !focused) {
          return;
        }
        setMessage(event?.data || t("password.encrypt_decrypt_required"));
        setPassword("");
        setError("");
        setOpen(true);
      })();
    });

    const unsubscribe2 = Events.On("eventInputPasswordClose", () => {
      close();
    });

    return () => {
      alive = false;
      unsubscribe();
      unsubscribe2();
    };
  }, []);

  const close = () => {
    setOpen(false);
    setPassword("");
    setMessage("");
    setError("");
    setSubmitting(false);
  };

  const handleClose = () => {
    close();
    void ConfigService.AbortPasswordWait().catch(() => {});
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
      close();
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
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("password.title")}</DialogTitle>
      <DialogContent>
        {message && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {message}
          </Typography>
        )}
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
        <Button onClick={handleClose} disabled={submitting}>
          {t("common.cancel")}
        </Button>
        <Button onClick={() => void handleSubmit()} variant="contained" disabled={submitting}>
          {t("common.ok")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default PasswordInputDialog;
