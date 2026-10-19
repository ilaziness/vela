import { Window } from "@wailsio/runtime";
import { Box, IconButton, Stack } from "@mui/material";
import { alpha } from "@mui/material/styles";
import { Close, CropSquare, FilterNone, Remove } from "@mui/icons-material";
import { useEffect, useState } from "react";
import {
  toggleWindowMaximise,
  windowMaximisedEvent,
} from "../hooks/useFrameless";

interface OpBarProps {
  onClose?: () => void;
  draggable?: boolean;
  showBorder?: boolean;
}

const minimizeColor = "#f7b424";
const maximizeColor = "#28C940";
const closeColor = "#FF5F57";

function controlButtonSx(color: string) {
  return {
    bgcolor: alpha(color, 0.18),
    "&:hover": {
      bgcolor: color,
      "& svg": { color: "#1c1c1c" },
    },
  };
}

const OpBar = ({
  onClose,
  draggable = true,
  showBorder = true,
}: OpBarProps) => {
  const [isMaximised, setIsMaximised] = useState(false);

  useEffect(() => {
    let active = true;
    Window.IsMaximised()
      .then((maximised) => {
        if (active) {
          setIsMaximised(maximised);
        }
      })
      .catch(() => {});
    const onMaximised = (event: Event) => {
      setIsMaximised(Boolean((event as CustomEvent<boolean>).detail));
    };
    window.addEventListener(windowMaximisedEvent, onMaximised);
    return () => {
      active = false;
      window.removeEventListener(windowMaximisedEvent, onMaximised);
    };
  }, []);

  const handleMinimize = () => {
    Window.Minimise().then(() => {});
  };

  const handleClose = () => {
    if (onClose) {
      onClose();
    } else {
      Window.Close().then(() => {});
    }
  };

  return (
    <Box
      onDoubleClick={(event) => {
        if ((event.target as HTMLElement).closest("button")) {
          return;
        }
        toggleWindowMaximise();
      }}
      sx={{
        "--wails-draggable": draggable ? "drag" : "no-drag",
        height: "100%",
        display: "flex",
        alignItems: "center",
        justifyContent: "flex-end",
        pr: 1,
        borderBottom: showBorder ? 1 : 0,
        borderColor: "divider",
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{ "--wails-draggable": "no-drag" }}
      >
        <IconButton
          size="small"
          onClick={handleMinimize}
          sx={controlButtonSx(minimizeColor)}
        >
          <Remove fontSize="small" sx={{ color: minimizeColor }} />
        </IconButton>
        <IconButton
          size="small"
          onClick={toggleWindowMaximise}
          sx={controlButtonSx(maximizeColor)}
        >
          {isMaximised ? (
            <FilterNone fontSize="small" sx={{ color: maximizeColor }} />
          ) : (
            <CropSquare fontSize="small" sx={{ color: maximizeColor }} />
          )}
        </IconButton>
        <IconButton
          size="small"
          onClick={handleClose}
          sx={controlButtonSx(closeColor)}
        >
          <Close fontSize="small" sx={{ color: closeColor }} />
        </IconButton>
      </Stack>
    </Box>
  );
};

export default OpBar;
