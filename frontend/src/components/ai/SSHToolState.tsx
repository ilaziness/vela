import React, { useEffect, useState } from "react";
import { Box, Button, CircularProgress } from "@mui/material";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services/index";
import { parseCallServiceError } from "../../func/service";
import { useAIAssistantStore } from "../../stores/aiAssistant";
import { useMessageStore } from "../../stores/message";

/**
 * run_ssh_command state slot: keep the status label, and while Exec is running
 * (approval-responded) show a Stop that cancels only this command — not the agent run.
 */
const SSHToolState = React.forwardRef<HTMLSpanElement, any>(
  function SSHToolState({ ownerState, children, ...other }, ref) {
    const showStop = ownerState?.state === "approval-responded";
    const activeSessionId = useAIAssistantStore((s) => s.activeSessionId);
    const [stopping, setStopping] = useState(false);

    useEffect(() => {
      if (!showStop) {
        setStopping(false);
      }
    }, [showStop]);

    const { sx: otherSx, ...rest } = other ?? {};

    return (
      <Box
        ref={ref}
        component="span"
        role="status"
        aria-label={typeof children === "string" ? children : undefined}
        {...rest}
        sx={[
          {
            display: "inline-flex",
            alignItems: "center",
            gap: 0.75,
            flexShrink: 0,
            fontSize: "0.75rem",
            color: "text.secondary",
            minWidth: 0,
          },
          ...(Array.isArray(otherSx) ? otherSx : otherSx ? [otherSx] : []),
        ]}
      >
        {showStop ? <CircularProgress size={12} color="inherit" /> : null}
        {typeof children === "string" ? children : null}
        {showStop ? (
          <Button
            size="small"
            color="warning"
            variant="outlined"
            disabled={stopping}
            sx={{
              py: 0,
              px: 0.75,
              minWidth: 0,
              fontSize: "0.7rem",
              lineHeight: 1.5,
              textTransform: "none",
            }}
            onMouseDown={(e) => {
              e.preventDefault();
              e.stopPropagation();
            }}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              if (!activeSessionId || stopping) return;
              setStopping(true);
              void AIService.CancelSSHExec(activeSessionId).catch((err) => {
                setStopping(false);
                useMessageStore.getState().errorMessage(parseCallServiceError(err));
              });
            }}
          >
            停止命令
          </Button>
        ) : null}
      </Box>
    );
  },
);

export default SSHToolState;
