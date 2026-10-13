import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Alert,
  Stack,
} from "@mui/material";
import { Events } from "@wailsio/runtime";
import { SSHService } from "../../bindings/github.com/ilaziness/vela/services";

interface Payload {
  host: string;
  address: string;
  fingerprint: string;
  key_type?: string;
  mismatch?: boolean;
  old_fingerprint?: string;
}

const HostKeyPrompt: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [payload, setPayload] = useState<Payload | null>(null);

  useEffect(() => {
    const unsubscribe = Events.On("eventHostKeyPrompt", (event: any) => {
      try {
        const data =
          typeof event.data === "string" ? JSON.parse(event.data) : event.data;
        setPayload(data as Payload);
        setOpen(true);
      } catch (e) {
        console.error("Invalid host key prompt payload", e);
      }
    });

    return () => {
      unsubscribe();
    };
  }, []);

  const handleClose = async (trust: boolean) => {
    if (payload) {
      try {
        await SSHService.SetHostKeyDecision(payload.host, trust);
      } catch (err) {
        console.error("Failed to send host key decision", err);
      }
    }
    setOpen(false);
    setPayload(null);
  };

  const mismatch = Boolean(payload?.mismatch);

  return (
    <Dialog
      open={open}
      onClose={() => handleClose(false)}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>
        {mismatch ? "主机密钥已变更" : "新的主机密钥"}
      </DialogTitle>
      <DialogContent>
        {payload && (
          <Stack spacing={2}>
            {mismatch && (
              <Alert severity="warning">
                服务器主机密钥与本地已信任记录不一致。若你刚重装系统或更换了机器，可选择更新；否则请拒绝连接，以防中间人攻击。
              </Alert>
            )}
            <Typography variant="body2" color="text.secondary">
              主机: {payload.host} ({payload.address})
            </Typography>
            <Typography variant="body2" color="text.secondary">
              类型: {payload.key_type || "—"}
            </Typography>
            {mismatch ? (
              <>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  旧指纹: {payload.old_fingerprint || "—"}
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  新指纹: {payload.fingerprint}
                </Typography>
              </>
            ) : (
              <>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  指纹: {payload.fingerprint}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  若信任此主机，选择“信任并继续”，将会把该主机密钥保存到文件中。
                </Typography>
              </>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => handleClose(false)}>拒绝</Button>
        <Button onClick={() => handleClose(true)} variant="contained">
          {mismatch ? "更新并继续" : "信任并继续"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default HostKeyPrompt;
