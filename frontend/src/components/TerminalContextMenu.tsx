import {
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
  Typography,
} from "@mui/material";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import ContentPasteIcon from "@mui/icons-material/ContentPaste";
import ClearAllIcon from "@mui/icons-material/ClearAll";
import ChatIcon from "@mui/icons-material/Chat";
import SearchIcon from "@mui/icons-material/Search";
import FiberManualRecordIcon from "@mui/icons-material/FiberManualRecord";
import StopCircleIcon from "@mui/icons-material/StopCircle";
import { terminalInstances } from "../stores/terminalInstances";
import { useAIAssistantStore } from "../stores/aiAssistant";
import { useKeybindingsStore } from "../stores/keybindings";
import { ShortcutAction, toShellCodeBlock } from "../func/shortcuts";
import { t } from "../i18n";

interface TerminalContextMenuProps {
  contextMenu: {
    mouseX: number;
    mouseY: number;
  } | null;
  onClose: () => void;
  linkID: string;
  onFind: () => void;
  logging: boolean;
  onStartLogging: () => void;
  onStopLogging: () => void;
}

function ShortcutHint({ action }: { action: string }) {
  const keys = useKeybindingsStore(
    (s) => s.bindings.find((b) => b.action === action)?.keys ?? "",
  );
  if (!keys) {
    return null;
  }
  return (
    <Typography variant="body2" color="text.secondary" sx={{ ml: 3, pl: 1 }}>
      {keys}
    </Typography>
  );
}

export default function TerminalContextMenu({
  contextMenu,
  onClose,
  linkID,
  onFind,
  logging,
  onStartLogging,
  onStopLogging,
}: TerminalContextMenuProps) {
  const selection = terminalInstances.get(linkID)?.getSelection() ?? "";
  const hasSelection = selection.length > 0;
  const canLog = linkID.length > 0;

  const handleCopy = () => {
    if (selection) {
      void navigator.clipboard.writeText(selection);
    }
    onClose();
  };

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (!text) {
        onClose();
        return;
      }
      const term = terminalInstances.get(linkID);
      term?.paste(text);
      term?.focus();
    } catch (err) {
      console.error("Failed to read clipboard contents: ", err);
    }
    onClose();
  };

  const handleClear = () => {
    const term = terminalInstances.get(linkID);
    term?.clear();
    onClose();
  };

  const handleAddToChat = () => {
    if (!selection) {
      onClose();
      return;
    }
    useAIAssistantStore.getState().appendToComposer(toShellCodeBlock(selection));
    onClose();
  };

  const handleFind = () => {
    onFind();
    onClose();
  };

  const handleToggleLogging = () => {
    if (logging) {
      onStopLogging();
    } else {
      onStartLogging();
    }
    onClose();
  };

  return (
    <Menu
      open={contextMenu !== null}
      onClose={onClose}
      anchorReference="anchorPosition"
      anchorPosition={
        contextMenu !== null
          ? { top: contextMenu.mouseY, left: contextMenu.mouseX }
          : undefined
      }
    >
      <MenuItem onClick={handleCopy}>
        <ListItemIcon>
          <ContentCopyIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{t("common.copy")}</ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalCopy} />
      </MenuItem>
      <MenuItem onClick={handlePaste}>
        <ListItemIcon>
          <ContentPasteIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{t("terminal.paste")}</ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalPaste} />
      </MenuItem>
      <MenuItem onClick={handleFind}>
        <ListItemIcon>
          <SearchIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{t("terminal.find")}</ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalFind} />
      </MenuItem>
      <MenuItem onClick={handleAddToChat} disabled={!hasSelection}>
        <ListItemIcon>
          <ChatIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{t("terminal.add_to_chat")}</ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalAddToChat} />
      </MenuItem>
      <MenuItem onClick={handleToggleLogging} disabled={!logging && !canLog}>
        <ListItemIcon>
          {logging ? (
            <StopCircleIcon fontSize="small" />
          ) : (
            <FiberManualRecordIcon fontSize="small" color="error" />
          )}
        </ListItemIcon>
        <ListItemText>
          {logging ? t("terminal.stop_logging") : t("terminal.start_logging")}
        </ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalToggleLog} />
      </MenuItem>
      <MenuItem onClick={handleClear}>
        <ListItemIcon>
          <ClearAllIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText>{t("terminal.clear_screen")}</ListItemText>
        <ShortcutHint action={ShortcutAction.TerminalClear} />
      </MenuItem>
    </Menu>
  );
}
