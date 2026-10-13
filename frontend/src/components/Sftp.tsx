import React, { useEffect, useEffectEvent, useRef, useState } from "react";
import {
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Toolbar,
  Typography,
} from "@mui/material";
import {
  CloudDownload as DownloadIcon,
  CreateNewFolder as CreateFolderIcon,
  Delete as DeleteIcon,
  Description as FileIcon,
  DriveFileRenameOutline as RenameIcon,
  Folder as FolderIcon,
  FolderOpen as FolderOpenIcon,
  NoteAdd as CreateFileIcon,
  Security as ChmodIcon,
  Person as ChownIcon,
  UploadFile as UploadIcon,
} from "@mui/icons-material";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import { Events } from "@wailsio/runtime";
import {
  LogService,
  SftpService,
  SSHService,
  ToolService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { ProgressData } from "../../bindings/github.com/ilaziness/vela/services/models";
import { formatFileSize, parseCallServiceError } from "../func/service";
import { sortFileList } from "../func/ftp";
import { useMessageStore } from "../stores/message";
import { useTransferStore } from "../stores/transfer";
import SftpNavbar from "./SftpNavbar";
import {
  FileInfo,
  modeBitsToOctal,
  parseOctalMode,
} from "../func/types";

interface SftpProps {
  linkID: string;
  ownerKey: string;
  onReady?: () => void;
}

function remoteJoin(dir: string, name: string): string {
  return dir === "/" ? `/${name}` : `${dir}/${name}`;
}

const Sftp: React.FC<SftpProps> = ({ linkID, ownerKey, onReady }) => {
  const [sftpLoaded, setSftpLoaded] = useState(false);
  const [currentPath, setCurrentPath] = useState<string>("/tmp");
  const [fileList, setFileList] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [showHiddenFiles, setShowHiddenFiles] = useState<boolean>(false);
  const initStartedRef = useRef(false);
  const { errorMessage: showMessageError } = useMessageStore();
  const addProgress = useTransferStore((s) => s.addProgress);

  const [contextMenu, setContextMenu] = useState<{
    mouseX: number;
    mouseY: number;
    file: FileInfo;
  } | null>(null);
  const [blankContextMenu, setBlankContextMenu] = useState<{
    mouseX: number;
    mouseY: number;
  } | null>(null);

  const [selectedNames, setSelectedNames] = useState<Set<string>>(new Set());

  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [filesToDelete, setFilesToDelete] = useState<FileInfo[]>([]);
  const [renameDialogOpen, setRenameDialogOpen] = useState(false);
  const [renamingItem, setRenamingItem] = useState<FileInfo | null>(null);
  const [renamingName, setRenamingName] = useState("");
  const [fullScreenLoading, setFullScreenLoading] = useState(false);
  const [createFileDialogOpen, setCreateFileDialogOpen] = useState(false);
  const [createDirDialogOpen, setCreateDirDialogOpen] = useState(false);
  const [newFileName, setNewFileName] = useState("");

  const [chmodOpen, setChmodOpen] = useState(false);
  const [chmodTarget, setChmodTarget] = useState<FileInfo | null>(null);
  const [chmodOctal, setChmodOctal] = useState("");
  const [chmodSymbolic, setChmodSymbolic] = useState("");

  const [chownOpen, setChownOpen] = useState(false);
  const [chownTarget, setChownTarget] = useState<FileInfo | null>(null);
  const [chownUid, setChownUid] = useState("");
  const [chownGid, setChownGid] = useState("");

  const currentPathRef = useRef(currentPath);
  currentPathRef.current = currentPath;

  const refreshFileList = async (path?: string, showHidden?: boolean) => {
    const targetPath = path !== undefined && path !== "" ? path : currentPath;
    const showHiddenAll = showHidden ?? showHiddenFiles;
    setLoading(true);
    try {
      const files = await SftpService.ListFiles(
        linkID,
        targetPath,
        showHiddenAll,
      );
      const sorted = sortFileList(files);
      setFileList(sorted);
      setSelectedNames((prev) => {
        if (prev.size === 0) return prev;
        const names = new Set(sorted.map((f) => f.name));
        const next = new Set([...prev].filter((n) => names.has(n)));
        return next.size === prev.size ? prev : next;
      });
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to list files: ${err.message || err}`);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  const refreshFileListRef = useRef(refreshFileList);
  refreshFileListRef.current = refreshFileList;

  const loadPending = async () => {
    if (!ownerKey) return;
    try {
      const pending = await SftpService.ListPendingTransfers(ownerKey);
      for (const item of pending) {
        addProgress({ ...item, sessionID: linkID } as ProgressData);
      }
    } catch (err) {
      LogService.Error(`ListPendingTransfers: ${err}`);
    }
  };

  const initSftp = useEffectEvent(async () => {
    if (initStartedRef.current || sftpLoaded) return;
    initStartedRef.current = true;
    try {
      await SSHService.StartSftp(linkID, ownerKey);
      LogService.Debug("SFTP connection established");
      const homePath = await SftpService.GetWd(linkID);
      setCurrentPath(homePath);
      await refreshFileList(homePath);
      await loadPending();
      setSftpLoaded(true);
      onReady?.();
    } catch (err: any) {
      initStartedRef.current = false;
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to initialize SFTP: ${err.message || err}`);
    }
  });

  useEffect(() => {
    void initSftp();
  }, [linkID]);

  useEffect(() => {
    const unsubDrop = Events.On("eventSftpFilesDropped", (event) => {
      const { sessionID, localPaths } = event.data;
      if (sessionID !== linkID || !localPaths?.length) return;

      SftpService.UploadPaths(linkID, currentPathRef.current, localPaths).catch(
        (err) => showMessageError(parseCallServiceError(err)),
      );
    });
    return unsubDrop;
  }, [linkID, showMessageError]);

  useEffect(() => {
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    const unsubProgress = Events.On("eventProgress", (event) => {
      const data = event.data as ProgressData;
      if (
        !data.done ||
        data.sessionID !== linkID ||
        data.transferType !== "upload"
      ) {
        return;
      }
      if (refreshTimer) clearTimeout(refreshTimer);
      refreshTimer = setTimeout(() => {
        void refreshFileListRef.current();
      }, 400);
    });
    return () => {
      unsubProgress();
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [linkID]);

  const clearSelection = () => {
    setSelectedNames(new Set());
  };

  const handleItemClick = async (file: FileInfo) => {
    if (!file.isDir) return;
    const newPath = remoteJoin(currentPath, file.name);
    try {
      setFullScreenLoading(true);
      await refreshFileList(newPath);
      setCurrentPath(newPath);
      clearSelection();
    } finally {
      setFullScreenLoading(false);
    }
  };

  const toggleSelectAll = () => {
    if (selectedNames.size === fileList.length && fileList.length > 0) {
      clearSelection();
    } else {
      setSelectedNames(new Set(fileList.map((f) => f.name)));
    }
  };

  const handleContextMenu = (event: React.MouseEvent, file: FileInfo) => {
    event.preventDefault();
    event.stopPropagation();
    setContextMenu({
      mouseX: event.clientX,
      mouseY: event.clientY,
      file,
    });
    setBlankContextMenu(null);
  };

  const handleBlankContextMenu = (event: React.MouseEvent) => {
    event.preventDefault();
    setBlankContextMenu({
      mouseX: event.clientX,
      mouseY: event.clientY,
    });
    setContextMenu(null);
  };

  const handleCloseMenu = () => setContextMenu(null);
  const handleCloseBlankMenu = () => setBlankContextMenu(null);

  const selectedFiles = (): FileInfo[] =>
    fileList.filter((f) => selectedNames.has(f.name));

  // 菜单操作目标：未勾选的右键/三点文件单独操作；已勾选则按多选批量
  const menuActionTargets = (): FileInfo[] => {
    if (
      contextMenu?.file &&
      (selectedNames.size === 0 || !selectedNames.has(contextMenu.file.name))
    ) {
      return [contextMenu.file];
    }
    return selectedFiles();
  };

  const handleDownload = async (targets: FileInfo[]) => {
    handleCloseMenu();
    if (targets.length === 0) return;

    try {
      if (targets.length === 1) {
        const file = targets[0];
        const remotePath = remoteJoin(currentPath, file.name);
        if (file.isDir) {
          await SftpService.DownloadDirectoryDialog(linkID, remotePath);
        } else {
          await SftpService.DownloadFileDialog(linkID, remotePath);
        }
      } else {
        const paths = targets.map((f) => remoteJoin(currentPath, f.name));
        await SftpService.DownloadPathsDialog(linkID, paths);
      }
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to download: ${err.message || err}`);
    }
  };

  const handleUpload = async (type: "file" | "directory") => {
    handleCloseBlankMenu();
    const remotePath = currentPath;
    const uploadPromise =
      type === "file"
        ? SftpService.UploadFileDialog(linkID, remotePath)
        : SftpService.UploadDirectoryDialog(linkID, remotePath);

    uploadPromise.catch((err) => {
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to upload ${type}: ${err.message || err}`);
    });
  };

  const openDeleteConfirm = (files: FileInfo[]) => {
    if (files.length === 0) return;
    setFilesToDelete(files);
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (filesToDelete.length === 0) return;
    const paths = filesToDelete.map((f) => remoteJoin(currentPath, f.name));
    setDeleteConfirmOpen(false);
    try {
      await SftpService.DeleteFiles(linkID, paths);
      clearSelection();
      await refreshFileList();
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to delete: ${err.message || err}`);
      await refreshFileList();
    } finally {
      setFilesToDelete([]);
    }
  };

  const handleRenameClick = () => {
    if (!contextMenu?.file) return;
    setRenamingItem(contextMenu.file);
    setRenamingName(contextMenu.file.name);
    setRenameDialogOpen(true);
    handleCloseMenu();
  };

  const handleRename = async () => {
    if (!renamingItem || !renamingName.trim()) return;
    try {
      setFullScreenLoading(true);
      setRenameDialogOpen(false);
      const oldPath = remoteJoin(currentPath, renamingItem.name);
      const newPath = remoteJoin(currentPath, renamingName.trim());
      if (oldPath !== newPath) {
        await SftpService.RenameFile(linkID, oldPath, newPath);
        await refreshFileList();
      }
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
      LogService.Error(`Failed to rename: ${err.message || err}`);
    } finally {
      setFullScreenLoading(false);
      setRenamingItem(null);
      setRenamingName("");
    }
  };

  const openChmod = async () => {
    const file = contextMenu?.file;
    handleCloseMenu();
    if (!file) return;
    setChmodTarget(file);
    const octal = modeBitsToOctal(file.modeBits || 0);
    setChmodOctal(octal);
    try {
      const res = await ToolService.ConvertChmod(octal, "toSymbolic");
      setChmodSymbolic(res.success ? res.symbolic || "" : "");
    } catch {
      setChmodSymbolic("");
    }
    setChmodOpen(true);
  };

  const confirmChmod = async () => {
    if (!chmodTarget) return;
    const bits = parseOctalMode(chmodOctal);
    if (bits === null) {
      showMessageError("请输入有效的八进制权限，例如 755 或 4755");
      return;
    }
    try {
      const path = remoteJoin(currentPath, chmodTarget.name);
      await SftpService.Chmod(linkID, path, bits);
      setChmodOpen(false);
      setChmodTarget(null);
      await refreshFileList();
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
    }
  };

  const openChown = () => {
    const file = contextMenu?.file;
    handleCloseMenu();
    if (!file) return;
    setChownTarget(file);
    setChownUid(String(file.uid ?? 0));
    setChownGid(String(file.gid ?? 0));
    setChownOpen(true);
  };

  const confirmChown = async () => {
    if (!chownTarget) return;
    const uid = Number.parseInt(chownUid, 10);
    const gid = Number.parseInt(chownGid, 10);
    if (Number.isNaN(uid) || Number.isNaN(gid) || uid < 0 || gid < 0) {
      showMessageError("UID / GID 须为非负整数");
      return;
    }
    try {
      const path = remoteJoin(currentPath, chownTarget.name);
      await SftpService.Chown(linkID, path, uid, gid);
      setChownOpen(false);
      setChownTarget(null);
      await refreshFileList();
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
    }
  };

  const handleCreateFile = () => {
    setCreateFileDialogOpen(true);
    setNewFileName("");
    handleCloseBlankMenu();
  };

  const handleCreateFileConfirm = async () => {
    if (!newFileName.trim()) return;
    try {
      setFullScreenLoading(true);
      setCreateFileDialogOpen(false);
      await SftpService.CreateFile(
        linkID,
        remoteJoin(currentPath, newFileName.trim()),
      );
      await refreshFileList();
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
    } finally {
      setFullScreenLoading(false);
      setNewFileName("");
    }
  };

  const handleCreateDirectory = () => {
    setCreateDirDialogOpen(true);
    setNewFileName("");
    handleCloseBlankMenu();
  };

  const handleCreateDirectoryConfirm = async () => {
    if (!newFileName.trim()) return;
    try {
      setFullScreenLoading(true);
      setCreateDirDialogOpen(false);
      await SftpService.CreateDirectory(
        linkID,
        remoteJoin(currentPath, newFileName.trim()),
      );
      await refreshFileList();
    } catch (err: any) {
      showMessageError(parseCallServiceError(err));
    } finally {
      setFullScreenLoading(false);
      setNewFileName("");
    }
  };

  const navigateToParent = async () => {
    if (currentPath === "/") return;
    const parentPath = currentPath.substring(0, currentPath.lastIndexOf("/"));
    const newPath = parentPath || "/";
    try {
      setFullScreenLoading(true);
      await refreshFileList(newPath);
      setCurrentPath(newPath);
      clearSelection();
    } finally {
      setFullScreenLoading(false);
    }
  };

  const onChmodOctalChange = async (value: string) => {
    setChmodOctal(value);
    const bits = parseOctalMode(value);
    if (bits === null) {
      setChmodSymbolic("");
      return;
    }
    try {
      const res = await ToolService.ConvertChmod(value.trim(), "toSymbolic");
      setChmodSymbolic(res.success ? res.symbolic || "" : "");
    } catch {
      setChmodSymbolic("");
    }
  };

  const selectionCount = selectedNames.size;
  const allSelected =
    fileList.length > 0 && selectedNames.size === fileList.length;
  const menuTargetCount = contextMenu ? menuActionTargets().length : 0;

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        position: "relative",
      }}
    >
      {sftpLoaded && (
        <SftpNavbar
          currentPath={currentPath}
          onPathChange={async (path: string) => {
            try {
              setFullScreenLoading(true);
              await refreshFileList(path);
              setCurrentPath(path);
              clearSelection();
            } finally {
              setFullScreenLoading(false);
            }
          }}
          onRefresh={async () => {
            try {
              setFullScreenLoading(true);
              await refreshFileList();
            } finally {
              setFullScreenLoading(false);
            }
          }}
          onNavigateToParent={navigateToParent}
          disableParentButton={currentPath === "/"}
          showHiddenFiles={showHiddenFiles}
          onToggleShowHidden={() => {
            const newShowHiddenFiles = !showHiddenFiles;
            setShowHiddenFiles(newShowHiddenFiles);
            refreshFileList(undefined, newShowHiddenFiles).then(() => {});
          }}
        />
      )}

      {selectionCount > 0 && (
        <Toolbar
          variant="dense"
          sx={{
            minHeight: 40,
            gap: 1,
            borderBottom: 1,
            borderColor: "divider",
            bgcolor: "action.selected",
          }}
        >
          <Typography variant="body2" sx={{ flex: 1 }}>
            已选 {selectionCount} 项
          </Typography>
          <Button
            size="small"
            startIcon={<DownloadIcon />}
            onClick={() => {
              void handleDownload(selectedFiles());
            }}
          >
            下载
          </Button>
          <Button
            size="small"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={() => openDeleteConfirm(selectedFiles())}
          >
            删除
          </Button>
          <Button size="small" onClick={clearSelection}>
            取消选择
          </Button>
        </Toolbar>
      )}

      <Box
        component="div"
        id={`sftp-drop-${linkID}`}
        data-file-drop-target=""
        className="sftp-drop-zone"
        onContextMenu={(e) => handleBlankContextMenu(e)}
        sx={{ flex: 1, overflow: "auto", position: "relative" }}
      >
        {!sftpLoaded || loading ? (
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
                      indeterminate={selectionCount > 0 && !allSelected}
                      checked={allSelected}
                      onChange={toggleSelectAll}
                    />
                  </TableCell>
                  <TableCell>类型</TableCell>
                  <TableCell>名称</TableCell>
                  <TableCell align="center">大小</TableCell>
                  <TableCell align="center">修改时间</TableCell>
                  <TableCell align="center">操作</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {fileList.map((file) => {
                  const selected = selectedNames.has(file.name);
                  return (
                    <TableRow
                      hover
                      key={file.name}
                      selected={selected}
                      sx={{
                        cursor: file.isDir ? "pointer" : "default",
                      }}
                      onContextMenu={(e) => handleContextMenu(e, file)}
                      onDoubleClick={() => handleItemClick(file)}
                    >
                      <TableCell padding="checkbox">
                        <Checkbox
                          size="small"
                          checked={selected}
                          onChange={() => {
                            setSelectedNames((prev) => {
                              const next = new Set(prev);
                              if (next.has(file.name)) next.delete(file.name);
                              else next.add(file.name);
                              return next;
                            });
                          }}
                        />
                      </TableCell>
                      <TableCell component="th" scope="row">
                        {file.isDir ? (
                          <FolderIcon sx={{ color: "#FFB74D" }} />
                        ) : (
                          <FileIcon sx={{ color: "#81C784" }} />
                        )}
                      </TableCell>
                      <TableCell>
                        <Box sx={{ display: "flex", alignItems: "center" }}>
                          <Typography noWrap sx={{ flex: 1 }}>
                            {file.name}
                          </Typography>
                          <Typography
                            noWrap
                            variant="subtitle2"
                            color="textSecondary"
                          >
                            {file.mode}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell align="center">
                        {file.isDir ? "-" : formatFileSize(file.size)}
                      </TableCell>
                      <TableCell align="center">
                        {new Date(file.modTime).toLocaleString()}
                      </TableCell>
                      <TableCell align="center">
                        <IconButton
                          onClick={(e) => {
                            e.stopPropagation();
                            handleContextMenu(e, file);
                          }}
                        >
                          <MoreVertIcon />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Box>

      <Menu
        open={contextMenu !== null}
        onClose={handleCloseMenu}
        anchorReference="anchorPosition"
        anchorPosition={
          contextMenu === null
            ? undefined
            : { top: contextMenu.mouseY, left: contextMenu.mouseX }
        }
      >
        <MenuItem onClick={() => void handleDownload(menuActionTargets())}>
          <ListItemIcon>
            <DownloadIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>
            {menuTargetCount > 1 ? `下载 (${menuTargetCount})` : "下载"}
          </ListItemText>
        </MenuItem>
        <MenuItem
          onClick={() => {
            openDeleteConfirm(menuActionTargets());
            handleCloseMenu();
          }}
        >
          <ListItemIcon>
            <DeleteIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>
            {menuTargetCount > 1 ? `删除 (${menuTargetCount})` : "删除"}
          </ListItemText>
        </MenuItem>
        {menuTargetCount <= 1 && (
          <MenuItem onClick={handleRenameClick}>
            <ListItemIcon>
              <RenameIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>重命名</ListItemText>
          </MenuItem>
        )}
        {menuTargetCount <= 1 && (
          <MenuItem onClick={() => void openChmod()}>
            <ListItemIcon>
              <ChmodIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>修改权限</ListItemText>
          </MenuItem>
        )}
        {menuTargetCount <= 1 && (
          <MenuItem onClick={openChown}>
            <ListItemIcon>
              <ChownIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>修改属主</ListItemText>
          </MenuItem>
        )}
      </Menu>

      <Menu
        open={blankContextMenu !== null}
        onClose={handleCloseBlankMenu}
        anchorReference="anchorPosition"
        anchorPosition={
          blankContextMenu === null
            ? undefined
            : { top: blankContextMenu.mouseY, left: blankContextMenu.mouseX }
        }
      >
        <MenuItem onClick={handleCreateFile}>
          <ListItemIcon>
            <CreateFileIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>新建文件</ListItemText>
        </MenuItem>
        <MenuItem onClick={handleCreateDirectory}>
          <ListItemIcon>
            <CreateFolderIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>新建文件夹</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => handleUpload("file")}>
          <ListItemIcon>
            <UploadIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>上传文件</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => handleUpload("directory")}>
          <ListItemIcon>
            <FolderOpenIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>上传文件夹</ListItemText>
        </MenuItem>
      </Menu>

      <Dialog
        open={deleteConfirmOpen}
        onClose={() => setDeleteConfirmOpen(false)}
      >
        <DialogTitle>确认删除</DialogTitle>
        <DialogContent>
          <Typography>
            {filesToDelete.length === 1
              ? `确定要删除 "${filesToDelete[0]?.name}" 吗?此操作不可撤销。`
              : `确定要删除选中的 ${filesToDelete.length} 项吗?此操作不可撤销。`}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteConfirmOpen(false)}>取消</Button>
          <Button onClick={() => void handleDelete()} color="error">
            删除
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={chmodOpen}
        onClose={() => {
          setChmodOpen(false);
          setChmodTarget(null);
        }}
      >
        <DialogTitle>修改权限</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {chmodTarget?.name}
          </Typography>
          <TextField
            autoFocus
            margin="dense"
            label="八进制权限"
            fullWidth
            value={chmodOctal}
            onChange={(e) => void onChmodOctalChange(e.target.value)}
            helperText={
              chmodSymbolic ? `符号: ${chmodSymbolic}` : "例如 755 或 4755"
            }
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setChmodOpen(false);
              setChmodTarget(null);
            }}
          >
            取消
          </Button>
          <Button onClick={() => void confirmChmod()} variant="contained">
            确定
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={chownOpen}
        onClose={() => {
          setChownOpen(false);
          setChownTarget(null);
        }}
      >
        <DialogTitle>修改属主</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {chownTarget?.name}（需要足够权限）
          </Typography>
          <TextField
            autoFocus
            margin="dense"
            label="UID"
            type="number"
            fullWidth
            value={chownUid}
            onChange={(e) => setChownUid(e.target.value)}
          />
          <TextField
            margin="dense"
            label="GID"
            type="number"
            fullWidth
            value={chownGid}
            onChange={(e) => setChownGid(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setChownOpen(false);
              setChownTarget(null);
            }}
          >
            取消
          </Button>
          <Button onClick={() => void confirmChown()} variant="contained">
            确定
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={createFileDialogOpen}
        onClose={() => {
          setCreateFileDialogOpen(false);
          setNewFileName("");
        }}
      >
        <DialogTitle>新建文件</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="文件名"
            fullWidth
            value={newFileName}
            onChange={(e) => setNewFileName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleCreateFileConfirm();
            }}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setCreateFileDialogOpen(false);
              setNewFileName("");
            }}
          >
            取消
          </Button>
          <Button onClick={() => void handleCreateFileConfirm()} variant="contained">
            创建
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={createDirDialogOpen}
        onClose={() => {
          setCreateDirDialogOpen(false);
          setNewFileName("");
        }}
      >
        <DialogTitle>新建文件夹</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="文件夹名"
            fullWidth
            value={newFileName}
            onChange={(e) => setNewFileName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleCreateDirectoryConfirm();
            }}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setCreateDirDialogOpen(false);
              setNewFileName("");
            }}
          >
            取消
          </Button>
          <Button
            onClick={() => void handleCreateDirectoryConfirm()}
            variant="contained"
          >
            创建
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={renameDialogOpen}
        onClose={() => {
          setRenameDialogOpen(false);
          setRenamingItem(null);
          setRenamingName("");
        }}
      >
        <DialogTitle>重命名</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="新名称"
            fullWidth
            value={renamingName}
            onChange={(e) => setRenamingName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setRenameDialogOpen(false);
              setRenamingItem(null);
              setRenamingName("");
            }}
          >
            取消
          </Button>
          <Button onClick={() => void handleRename()} variant="contained">
            重命名
          </Button>
        </DialogActions>
      </Dialog>

      {fullScreenLoading && (
        <Box
          sx={{
            position: "absolute",
            top: 0,
            left: 0,
            width: "100%",
            height: "100%",
            backgroundColor: "rgba(0, 0, 0, 0.5)",
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            zIndex: 9999,
          }}
        >
          <Box sx={{ width: "80%", maxWidth: 400 }}>
            <LinearProgress />
            <Typography
              variant="h6"
              color="white"
              align="center"
              sx={{ mt: 2 }}
            >
              请稍候...
            </Typography>
          </Box>
        </Box>
      )}
    </Box>
  );
};

export default Sftp;
