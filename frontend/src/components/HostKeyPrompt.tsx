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
import { t, tf } from "../i18n";

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
        {mismatch ? t("hostkey.mismatch_title") : t("hostkey.new_title")}
      </DialogTitle>
      <DialogContent>
        {payload && (
          <Stack spacing={2}>
            {mismatch && (
              <Alert severity="warning">{t("hostkey.mismatch_warning")}</Alert>
            )}
            <Typography variant="body2" color="text.secondary">
              {tf("hostkey.host_line", payload.host, payload.address)}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {tf("hostkey.type_line", payload.key_type || "—")}
            </Typography>
            {mismatch ? (
              <>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  {tf(
                    "hostkey.old_fingerprint",
                    payload.old_fingerprint || "—",
                  )}
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  {tf("hostkey.new_fingerprint", payload.fingerprint)}
                </Typography>
              </>
            ) : (
              <>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ wordBreak: "break-all" }}
                >
                  {tf("hostkey.fingerprint", payload.fingerprint)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {t("hostkey.trust_hint")}
                </Typography>
              </>
            )}
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => handleClose(false)}>
          {t("hostkey.reject")}
        </Button>
        <Button onClick={() => handleClose(true)} variant="contained">
          {mismatch
            ? t("hostkey.update_continue")
            : t("hostkey.trust_continue")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default HostKeyPrompt;
