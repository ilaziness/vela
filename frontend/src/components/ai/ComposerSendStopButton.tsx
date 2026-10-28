import React from "react";
import { IconButton, Tooltip, type IconButtonProps } from "@mui/material";
import StopIcon from "@mui/icons-material/Stop";
import { useChatActions, useChatComposer } from "@mui/x-chat/headless";
import { t } from "../../i18n";

const sendStopButtonSx: IconButtonProps["sx"] = {
  width: 36,
  height: 36,
  flexShrink: 0,
  marginInlineStart: "auto",
  bgcolor: "primary.main",
  color: "primary.contrastText",
  "&:hover": {
    bgcolor: "primary.dark",
  },
  "&.Mui-disabled": {
    bgcolor: "action.disabledBackground",
    color: "action.disabled",
  },
  "& svg": {
    fontSize: "1.25rem",
  },
};

function mergeSx(sx: IconButtonProps["sx"]): IconButtonProps["sx"] {
  return [sendStopButtonSx, ...(Array.isArray(sx) ? sx : sx ? [sx] : [])];
}

/** 空闲发送；生成中切换为停止并调用 stopStreaming。 */
const ComposerSendStopButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  function ComposerSendStopButton({ children, onClick, "aria-label": ariaLabel, sx, ...other }, ref) {
    const { isSubmitting, value } = useChatComposer();
    const actions = useChatActions();
    const isEmpty = value.trim() === "";

    if (isSubmitting) {
      const stopLabel = t("ai.chat.stop_generation");
      return (
        <Tooltip title={stopLabel}>
          <IconButton
            ref={ref}
            {...other}
            type="button"
            disabled={false}
            aria-label={stopLabel}
            onClick={(event) => {
              event.preventDefault();
              onClick?.(event);
              actions?.stopStreaming();
            }}
            sx={mergeSx(sx)}
          >
            <StopIcon fontSize="inherit" />
          </IconButton>
        </Tooltip>
      );
    }

    return (
      <IconButton
        ref={ref}
        {...other}
        type="submit"
        aria-label={ariaLabel}
        disabled={isEmpty}
        onClick={onClick}
        sx={mergeSx(sx)}
      >
        {children}
      </IconButton>
    );
  }
);

export default ComposerSendStopButton;
