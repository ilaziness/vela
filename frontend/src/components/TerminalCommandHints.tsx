import { useLayoutEffect, useRef, useState } from "react";
import { List, ListItemButton, ListItemText, Paper } from "@mui/material";

export const TERMINAL_HINTS_MAX = 8;

export type HintAnchor = {
  left: number;
  top: number;
  cellHeight: number;
};

interface TerminalCommandHintsProps {
  items: string[];
  activeIndex: number;
  anchor: HintAnchor | null;
  onSelect: (command: string) => void;
  onHover: (index: number) => void;
}

export default function TerminalCommandHints({
  items,
  activeIndex,
  anchor,
  onSelect,
  onHover,
}: TerminalCommandHintsProps) {
  const paperRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ left: 0, top: 0, ready: false });

  useLayoutEffect(() => {
    if (!anchor || items.length === 0 || !paperRef.current) {
      setPos((p) => ({ ...p, ready: false }));
      return;
    }
    const el = paperRef.current;
    const parent = el.offsetParent as HTMLElement | null;
    const pw = parent?.clientWidth ?? window.innerWidth;
    const ph = parent?.clientHeight ?? window.innerHeight;
    const h = el.offsetHeight;
    const w = el.offsetWidth;

    let left = anchor.left;
    // Prefer just below the cursor row.
    let top = anchor.top;
    if (top + h > ph - 4) {
      // Flip above the current line so the popup does not cover input.
      top = anchor.top - anchor.cellHeight - h - 2;
      if (top < 4) {
        top = 4;
      }
    }
    if (left + w > pw - 4) {
      left = Math.max(4, pw - w - 4);
    }
    if (left < 4) {
      left = 4;
    }
    setPos({ left, top, ready: true });
  }, [anchor, items, activeIndex]);

  if (items.length === 0 || !anchor) {
    return null;
  }

  return (
    <Paper
      ref={paperRef}
      elevation={4}
      onMouseDown={(e) => e.preventDefault()}
      sx={{
        position: "absolute",
        left: pos.left,
        top: pos.top,
        zIndex: 10,
        visibility: pos.ready ? "visible" : "hidden",
        width: "max-content",
        maxWidth: 420,
        maxHeight: 220,
        overflow: "auto",
        bgcolor: "background.paper",
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
      }}
    >
      <List dense disablePadding>
        {items.map((cmd, index) => (
          <ListItemButton
            key={`${index}-${cmd}`}
            selected={index === activeIndex}
            onMouseEnter={() => onHover(index)}
            onClick={() => onSelect(cmd)}
            sx={{ py: 0.25, px: 1 }}
          >
            <ListItemText
              primary={cmd}
              slotProps={{
                primary: {
                  sx: {
                    fontFamily: "monospace",
                    fontSize: "0.8125rem",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  },
                },
              }}
            />
          </ListItemButton>
        ))}
      </List>
    </Paper>
  );
}
