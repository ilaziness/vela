import { Box } from "@mui/material";
import React from "react";
import CloseIcon from "@mui/icons-material/Close";
import EllipsisTooltip from "./EllipsisTooltip";

interface SSHTabTextProps {
  text: string;
  onClose?: (event: React.MouseEvent) => void;
}

const SSHTabText: React.FC<SSHTabTextProps> = ({ text, onClose }) => {
  return (
    <Box component="span" sx={{ display: "flex", alignItems: "center" }}>
      <EllipsisTooltip text={text} sx={{ maxWidth: 90 }} />
      {onClose && (
        <CloseIcon
          sx={{
            fontSize: 16,
            ml: 0.5,
            borderRadius: "50%",
            "&:hover": {
              bgcolor: "action.hover",
            },
          }}
          onClick={onClose}
        />
      )}
    </Box>
  );
};

export default SSHTabText;
