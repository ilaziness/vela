import React, { useState } from "react";
import { Browser } from "@wailsio/runtime";
import { Button, Typography, Box, Stack } from "@mui/material";
import { GitHub as GitHubIcon } from "@mui/icons-material";
import { CheckUpdate } from "../../../bindings/github.com/ilaziness/vela/services/appservice";
import {
  AppInfo,
  NewVersion,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { useMessageStore } from "../../stores/message";
import { t, tf } from "../../i18n";
import { translateServiceError } from "../../func/service";

const UpdateAvailableDialog = React.lazy(
  () => import("../UpdateAvailableDialog"),
);

interface AboutProps {
  appinfo: AppInfo;
}

const About: React.FC<AboutProps> = ({ appinfo }) => {
  const { errorMessage, infoMessage } = useMessageStore();
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [newVersion, setNewVersion] = useState<NewVersion | null>(null);
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false);
  const [updateDialogMounted, setUpdateDialogMounted] = useState(false);

  React.useEffect(() => {
    if (updateDialogOpen) {
      setUpdateDialogMounted(true);
    }
  }, [updateDialogOpen]);

  const handleCheckUpdate = async () => {
    try {
      setCheckingUpdate(true);
      const [ok, ver] = await CheckUpdate();
      if (ok) {
        setNewVersion(ver);
        setUpdateDialogOpen(true);
      } else {
        infoMessage(t("settings.about.up_to_date"));
      }
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setCheckingUpdate(false);
    }
  };

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 3, fontWeight: 600 }}>
        {t("settings.about.title")}
      </Typography>
      <Box>
        <Stack spacing={1} sx={{ textAlign: "center", alignItems: "center" }}>
          <img
            src="/appicon.png"
            alt="Logo"
            style={{ width: "64px", height: "64px" }}
          />
          <Typography variant="h4" sx={{ fontWeight: 800 }}>
            Vela
          </Typography>

          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <GitHubIcon
              sx={{ "&:hover": { cursor: "pointer" } }}
              onClick={() => {
                Browser.OpenURL(appinfo.HomeURL);
              }}
            />
            <Typography
              component="span"
              variant="body1"
              sx={{ fontWeight: 500, ml: 1 }}
            >
              {appinfo.Version}
            </Typography>
            <Button
              size="small"
              sx={{ ml: 1 }}
              onClick={handleCheckUpdate}
              loading={checkingUpdate}
              variant="outlined"
            >
              {t("settings.about.check_update")}
            </Button>
          </Box>

          {updateDialogMounted && (
            <React.Suspense fallback={null}>
              <UpdateAvailableDialog
                open={updateDialogOpen}
                onClose={() => setUpdateDialogOpen(false)}
                newVersion={newVersion}
              />
            </React.Suspense>
          )}

          <Typography
            color="text.secondary"
            variant="body2"
            sx={{ fontWeight: 500, ml: 1 }}
          >
            {t("settings.about.tagline")}
          </Typography>

          <Typography
            color="text.secondary"
            variant="body2"
            sx={{ fontSize: "12px", ml: 1 }}
          >
            {tf("settings.about.build_time", appinfo.BuildTime)}
          </Typography>

          <Typography
            color="text.secondary"
            variant="body2"
            sx={{ fontSize: "12px", ml: 1 }}
          >
            {tf("settings.about.git", appinfo.GitInfo)}
          </Typography>
        </Stack>
      </Box>
    </Box>
  );
};

export default About;
