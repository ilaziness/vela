import { Box, Typography } from "@mui/material";
import {
  Description as FileIcon,
  Folder as FolderIcon,
} from "@mui/icons-material";
import React from "react";
import EllipsisTooltip from "../common/EllipsisTooltip";

interface SftpEntryLabelProps {
  isDir: boolean;
  name: string;
  /** Remote listings only — e.g. drwxr-xr-x */
  mode?: string;
}

/** Icon + ellipsized name (+ optional mode) for SFTP file rows. */
const SftpEntryLabel: React.FC<SftpEntryLabelProps> = ({
  isDir,
  name,
  mode,
}) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
      gap: 1,
      minWidth: 0,
    }}
  >
    {isDir ? (
      <FolderIcon sx={{ color: "#FFB74D", flexShrink: 0 }} fontSize="small" />
    ) : (
      <FileIcon sx={{ color: "#81C784", flexShrink: 0 }} fontSize="small" />
    )}
    <EllipsisTooltip text={name} sx={{ flex: 1 }} />
    {mode && (
      <Typography
        noWrap
        variant="subtitle2"
        color="text.secondary"
        sx={{ flexShrink: 0 }}
      >
        {mode}
      </Typography>
    )}
  </Box>
);

export default SftpEntryLabel;
