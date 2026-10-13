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
        setMessage(event?.data || "需要密码来加密/解密密码");
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
      setMessage("密码不能为空");
      return;
    }

    try {
      // 调用ConfigService.SetUserPassword保存密码
      await ConfigService.SetUserPassword(password);
      console.log("Password set successfully");
      close();
    } catch (error) {
      console.error("Failed to set password:", error);
      setMessage("设置密码失败: " + error);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Enter") {
      handleSubmit();
    }
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>输入密码</DialogTitle>
      <DialogContent>
        {message && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {message}
          </Typography>
        )}
        <TextField
          autoFocus
          margin="dense"
          label="密码"
          type="password"
          fullWidth
          variant="outlined"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="请输入密码"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>取消</Button>
        <Button onClick={handleSubmit} variant="contained">
          确定
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default PasswordInputDialog;
