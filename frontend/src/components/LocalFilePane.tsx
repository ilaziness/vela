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
  Typography,
} from "@mui/material";
import {
  Description as FileIcon,
  Folder as FolderIcon,
} from "@mui/icons-material";
import { Entry } from "../../bindings/github.com/ilaziness/vela/internal/localfs/models";
import { formatFileSize } from "../func/service";
import {
  PaneSide,
  dragStartsOnControl,
  writePaneDrag,
} from "../func/sftpPane";

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
        <TableContainer component={Paper} sx={{ boxShadow: "none" }}>
          <Table stickyHeader size="small">
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox
                    size="small"
                    indeterminate={selectedPaths.size > 0 && !allSelected}
                    checked={allSelected}
                    onChange={onToggleAll}
                  />
                </TableCell>
                <TableCell>名称</TableCell>
                <TableCell align="center">大小</TableCell>
                <TableCell align="center">修改时间</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {entries.map((entry) => {
                const selected = selectedPaths.has(entry.path);
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
                    <TableCell padding="checkbox">
                      <Checkbox
                        size="small"
                        checked={selected}
                        onChange={() => onToggle(entry.path)}
                      />
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                        {entry.isDir ? (
                          <FolderIcon sx={{ color: "#FFB74D" }} fontSize="small" />
                        ) : (
                          <FileIcon sx={{ color: "#81C784" }} fontSize="small" />
                        )}
                        <Typography noWrap>{entry.name}</Typography>
                      </Box>
                    </TableCell>
                    <TableCell align="center">
                      {entry.isDir ? "-" : formatFileSize(entry.size)}
                    </TableCell>
                    <TableCell align="center">
                      {entry.modTime
                        ? new Date(entry.modTime).toLocaleString()
                        : "-"}
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
