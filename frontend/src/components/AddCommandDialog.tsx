import React from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Box,
} from "@mui/material";
import { t } from "../i18n";

interface AddCommandDialogProps {
  open: boolean;
  onClose: () => void;
  category: string;
  name: string;
  command: string;
  description: string;
  onCategoryChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onCommandChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onSave: () => void;
}

const AddCommandDialog: React.FC<AddCommandDialogProps> = ({
  open,
  onClose,
  category,
  name,
  command,
  description,
  onCategoryChange,
  onNameChange,
  onCommandChange,
  onDescriptionChange,
  onSave,
}) => {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("command.add_title")}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField
            label={t("command.category")}
            value={category}
            onChange={(e) => onCategoryChange(e.target.value)}
            fullWidth
            required
          />
          <TextField
            label={t("command.name")}
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
            fullWidth
            required
          />
          <TextField
            label={t("command.description")}
            value={description}
            onChange={(e) => onDescriptionChange(e.target.value)}
            fullWidth
            multiline
            rows={2}
            minRows={2}
          />
          <TextField
            label={t("command.command")}
            value={command}
            onChange={(e) => onCommandChange(e.target.value)}
            fullWidth
            multiline
            rows={4}
            required
            minRows={4}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="outlined">
          {t("common.cancel")}
        </Button>
        <Button onClick={onSave} variant="contained">
          {t("common.save")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default AddCommandDialog;
