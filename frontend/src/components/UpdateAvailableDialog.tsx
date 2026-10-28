import React, { useEffect, useRef, useState } from "react";
import { Browser, Events } from "@wailsio/runtime";
import {
  Button,
  LinearProgress,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Box,
} from "@mui/material";
import Markdown from "markdown-to-jsx";
import { NewVersion } from "../../bindings/github.com/ilaziness/vela/services";
import { InstallUpdate } from "../../bindings/github.com/ilaziness/vela/services/appservice";
import { useMessageStore } from "../stores/message";
import { t } from "../i18n";
import { formatFileSize, translateServiceError } from "../func/service";

const EventDownloadProgress = "wails:updater:download-progress";
const EventVerifying = "wails:updater:verifying";
const EventInstalling = "wails:updater:installing";

type InstallPhase = "download" | "verify" | "install";

interface DownloadProgress {
  written: number;
  total: number;
}

function readProgress(event: { data?: DownloadProgress }): DownloadProgress | null {
  const data = event?.data;
  if (!data || typeof data.written !== "number") {
    return null;
  }
  return {
    written: data.written,
    total: typeof data.total === "number" ? data.total : 0,
  };
}
interface UpdateAvailableDialogProps {
  open: boolean;
  onClose: () => void;
  newVersion: NewVersion | null;
}

function openExternalURL(url: string | undefined | null) {
  if (!url) {
    return;
  }
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return;
    }
    Browser.OpenURL(parsed.toString());
  } catch {
    // ignore invalid URLs
  }
}

const markdownOptions = {
  disableParsingRawHTML: true,
  overrides: {
    h1: {
      component: Typography,
      props: { variant: "h6", gutterBottom: true, sx: { fontWeight: 700, mt: 1 } },
    },
    h2: {
      component: Typography,
      props: {
        variant: "subtitle1",
        gutterBottom: true,
        sx: { fontWeight: 700, mt: 1 },
      },
    },
    h3: {
      component: Typography,
      props: {
        variant: "subtitle2",
        gutterBottom: true,
        sx: { fontWeight: 600, mt: 1 },
      },
    },
    p: {
      component: Typography,
      props: { variant: "body2", paragraph: true },
    },
    li: {
      component: Typography,
      props: { component: "li", variant: "body2" },
    },
    a: {
      component: ({
        href,
        children,
        ...rest
      }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
        <Box
          component="a"
          href={href}
          {...rest}
          onClick={(e: React.MouseEvent) => {
            e.preventDefault();
            openExternalURL(href);
          }}
          sx={{
            color: "primary.main",
            textDecoration: "underline",
            cursor: "pointer",
          }}
        >
          {children}
        </Box>
      ),
    },
    code: {
      component: ({ children }: { children?: React.ReactNode }) => (
        <Box
          component="code"
          sx={{
            fontFamily: "monospace",
            fontSize: "0.85em",
            px: 0.5,
            py: 0.25,
            borderRadius: 0.5,
            bgcolor: "action.hover",
          }}
        >
          {children}
        </Box>
      ),
    },
    pre: {
      component: ({ children }: { children?: React.ReactNode }) => (
        <Box
          component="pre"
          sx={{
            fontFamily: "monospace",
            fontSize: "0.8rem",
            p: 1,
            borderRadius: 1,
            bgcolor: "action.hover",
            overflow: "auto",
            my: 1,
          }}
        >
          {children}
        </Box>
      ),
    },
  },
};

export default function UpdateAvailableDialog({
  open,
  onClose,
  newVersion,
}: UpdateAvailableDialogProps) {
  const { errorMessage } = useMessageStore();
  const [installing, setInstalling] = useState(false);
  const [phase, setPhase] = useState<InstallPhase>("download");
  const [progress, setProgress] = useState<DownloadProgress | null>(null);
  const installingRef = useRef(false);

  useEffect(() => {
    const offProgress = Events.On(EventDownloadProgress, (event: { data?: DownloadProgress }) => {
      if (!installingRef.current) {
        return;
      }
      const next = readProgress(event);
      if (next) {
        setProgress(next);
      }
    });
    const offVerify = Events.On(EventVerifying, () => {
      if (installingRef.current) {
        setPhase("verify");
      }
    });
    const offInstall = Events.On(EventInstalling, () => {
      if (installingRef.current) {
        setPhase("install");
      }
    });
    return () => {
      offProgress();
      offVerify();
      offInstall();
    };
  }, []);

  const handleInstall = async () => {
    if (installingRef.current) {
      return;
    }
    installingRef.current = true;
    setPhase("download");
    setProgress(null);
    setInstalling(true);
    try {
      await InstallUpdate();
      // Restart 成功后进程会退出，保持加载直到窗口关闭。
    } catch (err) {
      installingRef.current = false;
      setInstalling(false);
      errorMessage(translateServiceError(err));
    }
  };

  return (
    <Dialog
      open={open}
      onClose={() => {
        if (!installing) {
          onClose();
        }
      }}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>{t("update.title")}</DialogTitle>
      <DialogContent>
        <Typography sx={{ fontWeight: 600 }}>{newVersion?.Version}</Typography>
        {newVersion?.Notes ? (
          <Box
            sx={{
              mt: 1,
              maxHeight: 360,
              overflow: "auto",
              "& ul, & ol": { pl: 2.5, m: 0 },
            }}
          >
            <Markdown options={markdownOptions}>{newVersion.Notes}</Markdown>
          </Box>
        ) : null}
        {installing ? (
          <Box sx={{ mt: 2 }}>
            <LinearProgress
              variant={
                phase === "download" && progress && progress.total > 0
                  ? "determinate"
                  : "indeterminate"
              }
              value={
                progress && progress.total > 0
                  ? Math.min(100, (progress.written / progress.total) * 100)
                  : undefined
              }
            />
            <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5, display: "block" }}>
              {phase === "verify"
                ? t("update.verifying")
                : phase === "install"
                  ? t("update.installing")
                  : progress && progress.total > 0
                    ? `${formatFileSize(progress.written)} / ${formatFileSize(progress.total)}`
                    : t("update.downloading")}
            </Typography>
          </Box>
        ) : null}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={installing}>
          {t("common.close")}
        </Button>
        <Button
          onClick={() => openExternalURL(newVersion?.URL)}
          disabled={installing}
        >
          {t("update.open_download_page")}
        </Button>
        <Button
          variant="contained"
          onClick={handleInstall}
          loading={installing}
        >
          {t("update.install")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
