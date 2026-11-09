import React from "react";
import {
  Box,
  Checkbox,
  CircularProgress,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import { Entry } from "../../../bindings/github.com/ilaziness/vela/internal/localfs/models";
import { formatFileSize } from "../../func/service";
import { t } from "../../i18n";
import {
  PaneSide,
  dragStartsOnControl,
  sftpCheckboxSx,
  sftpCol,
  sftpTableContainerSx,
  sftpTableSx,
  writePaneDrag,
} from "../../func/sftpPane";
import EllipsisTooltip from "../common/EllipsisTooltip";
import SftpEntryLabel from "../sftp/SftpEntryLabel";

interface LocalFilePaneProps {
  entries: Entry[];
  loading: boolean;
  selectedPaths: Set<string>;
  onToggle: (path: string) => void;
  onToggleAll: () => void;
  onOpen: (entry: Entry) => void;
  pathsForDrag: (entry: Entry) => string[];
  onDragOver: (event: React.DragEvent) => void;
  onDrop: (event: React.DragEvent) => void;
}

const LocalFilePane: React.FC<LocalFilePaneProps> = ({
  entries,
  loading,
  selectedPaths,
  onToggle,
  onToggleAll,
  onOpen,
  pathsForDrag,
  onDragOver,
  onDrop,
}) => {
  const allSelected = entries.length > 0 && selectedPaths.size === entries.length;

  return (
    <Box
      onDragOver={onDragOver}
      onDrop={onDrop}
      sx={{ flex: 1, minWidth: 0, overflow: "auto", position: "relative" }}
    >
      {loading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            height: "100%",
          }}
        >
          <CircularProgress />
        </Box>
      ) : (
        <TableContainer component={Paper} sx={sftpTableContainerSx}>
          <Table stickyHeader size="small" sx={sftpTableSx}>
            <TableHead>
              <TableRow>
                <TableCell sx={sftpCol.checkbox}>
                  <Checkbox
                    size="small"
                    sx={sftpCheckboxSx}
                    indeterminate={selectedPaths.size > 0 && !allSelected}
                    checked={allSelected}
                    onChange={onToggleAll}
                  />
                </TableCell>
                <TableCell>{t("sftp.name")}</TableCell>
                <TableCell align="center" sx={sftpCol.size}>
                  {t("sftp.size")}
                </TableCell>
                <TableCell align="center" sx={sftpCol.modified}>
                  {t("sftp.modified")}
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {entries.map((entry) => {
                const selected = selectedPaths.has(entry.path);
                const modTimeText = entry.modTime
                  ? new Date(entry.modTime).toLocaleString()
                  : "-";
                return (
                  <TableRow
                    hover
                    key={entry.path}
                    selected={selected}
                    draggable
                    sx={{ cursor: entry.isDir ? "pointer" : "default" }}
                    onDoubleClick={() => onOpen(entry)}
                    onDragStart={(event) => {
                      if (dragStartsOnControl(event.target)) {
                        event.preventDefault();
                        return;
                      }
                      writePaneDrag(event.dataTransfer, {
                        side: PaneSide.Local,
                        paths: pathsForDrag(entry),
                      });
                    }}
                  >
                    <TableCell>
                      <Checkbox
                        size="small"
                        sx={sftpCheckboxSx}
                        checked={selected}
                        onChange={() => onToggle(entry.path)}
                      />
                    </TableCell>
                    <TableCell sx={{ overflow: "hidden" }}>
                      <SftpEntryLabel isDir={entry.isDir} name={entry.name} />
                    </TableCell>
                    <TableCell align="center">
                      {entry.isDir ? "-" : formatFileSize(entry.size)}
                    </TableCell>
                    <TableCell align="center" sx={{ overflow: "hidden" }}>
                      <EllipsisTooltip text={modTimeText} />
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  );
};

export default LocalFilePane;
