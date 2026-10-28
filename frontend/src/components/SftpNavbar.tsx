import React, { useEffect, useState } from "react";
import {
  Box,
  IconButton,
  InputAdornment,
  TextField,
  Tooltip,
} from "@mui/material";
import {
  ArrowForward as ArrowForwardIcon,
  Refresh as RefreshIcon,
  Visibility as VisibilityIcon,
  VisibilityOff as VisibilityOffIcon,
} from "@mui/icons-material";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import { t } from "../i18n";

interface SftpNavbarProps {
  currentPath: string;
  onPathChange: (path: string) => void;
  onRefresh: () => void;
  onNavigateToParent: () => void;
  disableParentButton: boolean;
  showHiddenFiles: boolean;
  onToggleShowHidden: () => void;
}

const SftpNavbar: React.FC<SftpNavbarProps> = ({
  currentPath,
  onPathChange,
  onRefresh,
  onNavigateToParent,
  disableParentButton,
  showHiddenFiles,
  onToggleShowHidden,
}) => {
  const [pathInput, setPathInput] = useState<string>(currentPath);

  // 当currentPath变化时，同步更新输入框的值
  useEffect(() => {
    setPathInput(currentPath);
  }, [currentPath]);

  const handlePathSubmit = (e: React.SubmitEvent) => {
    e.preventDefault();
    if (pathInput.trim() && pathInput !== currentPath) {
      onPathChange(pathInput);
    }
  };

  const handleNavigateToInputPath = () => {
    if (pathInput.trim() && pathInput !== currentPath) {
      onPathChange(pathInput);
    }
  };

  return (
    <Box
      component="form"
      onSubmit={handlePathSubmit}
      sx={{
        p: 1,
        display: "flex",
        alignItems: "center",
        gap: 1,
      }}
    >
      <Tooltip
        title={
          showHiddenFiles ? t("sftp.hide_dotfiles") : t("sftp.show_dotfiles")
        }
      >
        <IconButton size="small" onClick={onToggleShowHidden}>
          {showHiddenFiles ? (
            <VisibilityIcon fontSize="small" />
          ) : (
            <VisibilityOffIcon fontSize="small" />
          )}
        </IconButton>
      </Tooltip>
      <Tooltip title={t("sftp.go_up")}>
        <IconButton
          size="small"
          onClick={onNavigateToParent}
          disabled={disableParentButton}
        >
          <ArrowUpwardIcon fontSize="small" />
        </IconButton>
      </Tooltip>

      <TextField
        value={pathInput}
        onChange={(e) => setPathInput(e.target.value)}
        fullWidth
        size="small"
        variant="outlined"
        slotProps={{
          input: {
            endAdornment: (
              <InputAdornment position="end">
                <Tooltip title={t("sftp.navigate_to_path")}>
                  <IconButton size="small" onClick={handleNavigateToInputPath}>
                    <ArrowForwardIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t("common.refresh")}>
                  <IconButton size="small" onClick={onRefresh}>
                    <RefreshIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          },
        }}
      />
    </Box>
  );
};

export default SftpNavbar;
