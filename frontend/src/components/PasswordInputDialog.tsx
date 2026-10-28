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
import * as ConfigService from "../../bindings/github.com/ilaziness/vela/services/configservice";
import { t, tf } from "../i18n";

const PasswordInputDialog: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    let alive = true;
    const unsubscribe = Events.On("eventInputPassword", (event: any) => {
      void (async () => {
        let focused = true;
        try {
          focused = await Window.IsFocused();
        } catch (error) {
          console.error("window focus check failed", error);
        }
        if (!alive || !focused) {
          return;
        }
        setMessage(
          event?.data || t("password.encrypt_decrypt_required"),
        );
        setPassword("");
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
  };

  const handleClose = () => {
    close();
    ConfigService.SetUserPassword("").then(() => {});
  };

  const handleSubmit = async () => {
    if (!password.trim()) {
      setMessage(t("password.required"));
      return;
    }

    try {
      await ConfigService.SetUserPassword(password);
      console.log("Password set successfully");
      close();
    } catch (error) {
      console.error("Failed to set password:", error);
      setMessage(tf("password.set_failed", String(error)));
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Enter") {
      handleSubmit();
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
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>{t("common.cancel")}</Button>
        <Button onClick={handleSubmit} variant="contained">
          {t("common.ok")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default PasswordInputDialog;
