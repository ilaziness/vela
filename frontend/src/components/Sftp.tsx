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
  Divider,
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
  Tooltip,
  Typography,
} from "@mui/material";
import {
  CloudDownload as DownloadIcon,
  CreateNewFolder as CreateFolderIcon,
  Delete as DeleteIcon,
  Description as FileIcon,
  Edit as EditIcon,
  DriveFileRenameOutline as RenameIcon,
  Folder as FolderIcon,
  FolderOpen as FolderOpenIcon,
  NoteAdd as CreateFileIcon,
  Security as ChmodIcon,
  Person as ChownIcon,
  UploadFile as UploadIcon,
} from "@mui/icons-material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import ArrowForwardIcon from "@mui/icons-material/ArrowForward";
import { Events } from "@wailsio/runtime";
import {
  LogService,
  SftpService,
  SSHService,
  ToolService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { Entry } from "../../bindings/github.com/ilaziness/vela/internal/localfs/models";
import { ProgressData } from "../../bindings/github.com/ilaziness/vela/services/models";
import { formatFileSize, translateServiceError } from "../func/service";
import { t, tf } from "../i18n";
import { sortFileList } from "../func/ftp";
import {
  PaneSide,
  allowPaneDragOver,
  dragStartsOnControl,
  readPaneDrag,
  writePaneDrag,
} from "../func/sftpPane";
import { useMessageStore } from "../stores/message";
import { useTransferStore } from "../stores/transfer";
import LocalFilePane from "./LocalFilePane";
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
  const [localDir, setLocalDir] = useState("");
  const [localParent, setLocalParent] = useState("");
  const [localAtRoot, setLocalAtRoot] = useState(false);
  const [localEntries, setLocalEntries] = useState<Entry[]>([]);
  const [localLoading, setLocalLoading] = useState(false);
  const [localShowHidden, setLocalShowHidden] = useState(false);
  const [localSelected, setLocalSelected] = useState<Set<string>>(new Set());
  const initStartedRef = useRef(false);
  // Gate terminal-cwd follow until StartSftp + first listing finished.
  const remoteReadyRef = useRef(false);
  const pendingCwdRef = useRef<string | null>(null);
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
  const localDirRef = useRef(localDir);
  localDirRef.current = localDir;
  const localShowHiddenRef = useRef(localShowHidden);
  localShowHiddenRef.current = localShowHidden;
  const localListSeq = useRef(0);
  const remoteListSeq = useRef(0);
  const remoteListInFlight = useRef(false);
  const remoteNavTarget = useRef<string | null>(null);
  const localNavTarget = useRef<string | null>(null);
  const [remoteNavigating, setRemoteNavigating] = useState(false);
  const [localNavigating, setLocalNavigating] = useState(false);

  const refreshFileList = async (path?: string, showHidden?: boolean) => {
    const targetPath =
      path !== undefined && path !== "" ? path : currentPathRef.current;
    const showHiddenAll = showHidden ?? showHiddenFiles;
    const seq = ++remoteListSeq.current;
    const changing = targetPath !== currentPathRef.current;
    remoteListInFlight.current = true;
    if (changing) {
      remoteNavTarget.current = targetPath;
      setRemoteNavigating(true);
    }
    setLoading(true);
    try {
      const files = await SftpService.ListFiles(
        linkID,
        targetPath,
        showHiddenAll,
      );
      if (seq !== remoteListSeq.current) return false;
      const sorted = sortFileList(files);
      setFileList(sorted);
      setSelectedNames((prev) => {
        if (prev.size === 0) return prev;
        const names = new Set(sorted.map((f) => f.name));
        const next = new Set([...prev].filter((n) => names.has(n)));
        return next.size === prev.size ? prev : next;
      });
      return true;
    } catch (err: any) {
      if (seq !== remoteListSeq.current) return false;
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to list files: ${err.message || err}`);
      throw err;
    } finally {
      if (seq === remoteListSeq.current) {
        remoteListInFlight.current = false;
        remoteNavTarget.current = null;
        setRemoteNavigating(false);
        setLoading(false);
      }
    }
  };

  const refreshFileListRef = useRef(refreshFileList);
  refreshFileListRef.current = refreshFileList;

  const refreshLocal = async (dir?: string, showHidden?: boolean) => {
    const target = dir !== undefined ? dir : localDirRef.current;
    const showHiddenAll = showHidden ?? localShowHiddenRef.current;
    const seq = ++localListSeq.current;
    const changing = dir !== undefined && dir !== localDirRef.current;
    if (changing) {
      localNavTarget.current = target;
      setLocalNavigating(true);
    }
    setLocalLoading(true);
    try {
      const listing = await SftpService.ListLocal(target, showHiddenAll);
      if (seq !== localListSeq.current) return false;
      const entries = listing.entries ?? [];
      localDirRef.current = listing.dir;
      setLocalDir(listing.dir);
      setLocalParent(listing.parent);
      setLocalAtRoot(listing.atRoot);
      setLocalEntries(entries);
      setLocalSelected((prev) => {
        if (prev.size === 0) return prev;
        const paths = new Set(entries.map((entry) => entry.path));
        const next = new Set([...prev].filter((path) => paths.has(path)));
        return next.size === prev.size ? prev : next;
      });
      return true;
    } catch (err: any) {
      if (seq !== localListSeq.current) return false;
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to list local files: ${err.message || err}`);
      return false;
    } finally {
      if (seq === localListSeq.current) {
        localNavTarget.current = null;
        setLocalNavigating(false);
        setLocalLoading(false);
      }
    }
  };

  const refreshLocalRef = useRef(refreshLocal);
  refreshLocalRef.current = refreshLocal;
  const enqueueUploadRef = useRef<(paths: string[]) => void>(() => {});

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

  const followTerminalCwd = useEffectEvent(async (path: string) => {
    if (!path) return;
    if (!remoteReadyRef.current) {
      pendingCwdRef.current = path;
      return;
    }
    if (path === currentPathRef.current) return;
    try {
      const applied = await refreshFileListRef.current(path);
      if (!applied) return;
      currentPathRef.current = path;
      setCurrentPath(path);
      setSelectedNames(new Set());
    } catch {
      // Keep current listing; refreshFileList already surfaced the error.
    }
  });

  const initSftp = useEffectEvent(async () => {
    if (initStartedRef.current || sftpLoaded) return;
    initStartedRef.current = true;
    remoteReadyRef.current = false;
    pendingCwdRef.current = null;
    try {
      await SSHService.StartSftp(linkID, ownerKey);
      LogService.Debug("SFTP connection established");
      let startPath = "";
      try {
        startPath = (await SSHService.TerminalCwd(linkID)) || "";
      } catch (err) {
        LogService.Error(`TerminalCwd: ${err}`);
      }
      if (!startPath) {
        startPath = await SftpService.GetWd(linkID);
      }
      currentPathRef.current = startPath;
      setCurrentPath(startPath);
      await refreshFileList(startPath);
      await loadPending();
      setSftpLoaded(true);
      remoteReadyRef.current = true;
      onReady?.();
      // Apply cwd reported while StartSftp / first listing ran (no-op if unchanged).
      const pending = pendingCwdRef.current;
      pendingCwdRef.current = null;
      if (pending) {
        void followTerminalCwd(pending);
      }
    } catch (err: any) {
      initStartedRef.current = false;
      remoteReadyRef.current = false;
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to initialize SFTP: ${err.message || err}`);
    }
  });

  useEffect(() => {
    void initSftp();
    void refreshLocalRef.current("");
  }, [linkID]);

  useEffect(() => {
    const unsubCwd = Events.On("eventTerminalCwd", (event: any) => {
      try {
        const raw =
          typeof event.data === "string"
            ? JSON.parse(event.data)
            : event.data;
        const id = raw?.id as string | undefined;
        const path = raw?.path as string | undefined;
        if (!id || id !== linkID || !path) return;
        void followTerminalCwd(path);
      } catch (e) {
        console.error("Invalid terminal cwd payload", e);
      }
    });
    return unsubCwd;
  }, [linkID]);

  useEffect(() => {
    const unsubDrop = Events.On("eventSftpFilesDropped", (event) => {
      const { sessionID, localPaths } = event.data;
      if (sessionID !== linkID || !localPaths?.length) return;
      enqueueUploadRef.current(localPaths);
    });
    return unsubDrop;
  }, [linkID]);

  useEffect(() => {
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    let refreshRemote = false;
    let refreshLocalPane = false;
    const unsubProgress = Events.On("eventProgress", (event) => {
      const data = event.data as ProgressData;
      if (!data.done || data.sessionID !== linkID) {
        return;
      }
      if (data.transferType === "upload") {
        refreshRemote = true;
      } else if (data.transferType === "download") {
        refreshLocalPane = true;
      } else {
        return;
      }
      if (refreshTimer) clearTimeout(refreshTimer);
      refreshTimer = setTimeout(() => {
        if (refreshRemote && !remoteNavTarget.current) {
          void refreshFileListRef.current();
        }
        if (refreshLocalPane && !localNavTarget.current) {
          void refreshLocalRef.current();
        }
        refreshRemote = false;
        refreshLocalPane = false;
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
      const applied = await refreshFileList(newPath);
      if (!applied) return;
      currentPathRef.current = newPath;
      setCurrentPath(newPath);
      clearSelection();
    } finally {
      if (!remoteListInFlight.current) setFullScreenLoading(false);
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

  const handleEdit = async (file: FileInfo) => {
    handleCloseMenu();
    if (file.isDir) return;
    try {
      await SftpService.EditRemoteFile(linkID, remoteJoin(currentPath, file.name));
    } catch (err: any) {
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to edit: ${err.message || err}`);
    }
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
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to download: ${err.message || err}`);
    }
  };

  const handleUpload = async (type: "file" | "directory") => {
    handleCloseBlankMenu();
    handleCloseMenu();
    const remotePath = currentPath;
    const uploadPromise =
      type === "file"
        ? SftpService.UploadFileDialog(linkID, remotePath)
        : SftpService.UploadDirectoryDialog(linkID, remotePath);

    uploadPromise.catch((err) => {
      showMessageError(translateServiceError(err));
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
      showMessageError(translateServiceError(err));
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
      showMessageError(translateServiceError(err));
      LogService.Error(`Failed to rename: ${err.message || err}`);
    } finally {
      if (!remoteListInFlight.current) setFullScreenLoading(false);
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
      showMessageError(t("sftp.invalid_chmod"));
      return;
    }
    try {
      const path = remoteJoin(currentPath, chmodTarget.name);
      await SftpService.Chmod(linkID, path, bits);
      setChmodOpen(false);
      setChmodTarget(null);
      await refreshFileList();
    } catch (err: any) {
      showMessageError(translateServiceError(err));
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
      showMessageError(t("sftp.invalid_uid_gid"));
      return;
    }
    try {
      const path = remoteJoin(currentPath, chownTarget.name);
      await SftpService.Chown(linkID, path, uid, gid);
      setChownOpen(false);
      setChownTarget(null);
      await refreshFileList();
    } catch (err: any) {
      showMessageError(translateServiceError(err));
    }
  };

  const handleCreateFile = () => {
    setCreateFileDialogOpen(true);
    setNewFileName("");
    handleCloseBlankMenu();
    handleCloseMenu();
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
      showMessageError(translateServiceError(err));
    } finally {
      if (!remoteListInFlight.current) setFullScreenLoading(false);
      setNewFileName("");
    }
  };

  const handleCreateDirectory = () => {
    setCreateDirDialogOpen(true);
    setNewFileName("");
    handleCloseBlankMenu();
    handleCloseMenu();
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
      showMessageError(translateServiceError(err));
    } finally {
      if (!remoteListInFlight.current) setFullScreenLoading(false);
      setNewFileName("");
    }
  };

  const navigateToParent = async () => {
    if (currentPath === "/") return;
    const parentPath = currentPath.substring(0, currentPath.lastIndexOf("/"));
    const newPath = parentPath || "/";
    try {
      setFullScreenLoading(true);
      const applied = await refreshFileList(newPath);
      if (!applied) return;
      currentPathRef.current = newPath;
      setCurrentPath(newPath);
      clearSelection();
    } finally {
      if (!remoteListInFlight.current) setFullScreenLoading(false);
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

  const selectedLocalPaths = (): string[] =>
    localEntries
      .filter((entry) => localSelected.has(entry.path))
      .map((entry) => entry.path);

  const pathsForLocalDrag = (entry: Entry): string[] => {
    if (localSelected.has(entry.path)) return selectedLocalPaths();
    return [entry.path];
  };

  const pathsForRemoteDrag = (file: FileInfo): string[] => {
    const targets = selectedNames.has(file.name) ? selectedFiles() : [file];
    return targets.map((item) => remoteJoin(currentPath, item.name));
  };

  const enqueueUpload = (paths: string[]) => {
    if (paths.length === 0) return;
    if (remoteNavTarget.current) {
      showMessageError(t("sftp.remote_dir_switching"));
      return;
    }
    SftpService.UploadPaths(linkID, currentPathRef.current, paths).catch(
      (err) => {
        showMessageError(translateServiceError(err));
        LogService.Error(`Failed to upload paths: ${err.message || err}`);
      },
    );
  };
  enqueueUploadRef.current = enqueueUpload;

  const enqueueDownload = (paths: string[]) => {
    if (paths.length === 0 || !localDirRef.current) return;
    if (localNavTarget.current) {
      showMessageError(t("sftp.local_dir_switching"));
      return;
    }
    SftpService.DownloadPaths(linkID, localDirRef.current, paths).catch(
      (err) => {
        showMessageError(translateServiceError(err));
        LogService.Error(`Failed to download paths: ${err.message || err}`);
      },
    );
  };

  const handlePaneDrop = (target: PaneSide, event: React.DragEvent) => {
    const data = readPaneDrag(event.dataTransfer);
    if (!data || data.side === target) return;
    event.preventDefault();
    if (target === PaneSide.Remote) {
      enqueueUpload(data.paths);
      return;
    }
    enqueueDownload(data.paths);
  };

  const selectionCount = selectedNames.size;
  const allSelected =
    fileList.length > 0 && selectedNames.size === fileList.length;
  const menuTargetCount = contextMenu ? menuActionTargets().length : 0;
  const editTarget =
    menuTargetCount === 1 && !menuActionTargets()[0].isDir
      ? menuActionTargets()[0]
      : null;

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        position: "relative",
      }}
    >
      {!sftpLoaded ? (
        <Box
          sx={{
            flex: 1,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          <CircularProgress />
        </Box>
      ) : (
        <>
          <Box
            sx={{
              display: "flex",
              alignItems: "stretch",
              borderBottom: 1,
              borderColor: "divider",
            }}
          >
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <SftpNavbar
                currentPath={localDir}
                onPathChange={(path: string) => {
                  void refreshLocal(path);
                }}
                onRefresh={() => {
                  void refreshLocal();
                }}
                onNavigateToParent={() => {
                  if (localAtRoot) return;
                  void refreshLocal(localParent);
                }}
                disableParentButton={localAtRoot}
                showHiddenFiles={localShowHidden}
                onToggleShowHidden={() => {
                  const next = !localShowHidden;
                  setLocalShowHidden(next);
                  void refreshLocal(localDir, next);
                }}
              />
            </Box>
            <Box
              sx={{
                width: 40,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <Tooltip title={t("sftp.upload_to_remote")}>
                <span>
                  <IconButton
                    size="small"
                    sx={{ p: 0.25 }}
                    disabled={localSelected.size === 0 || remoteNavigating}
                    onClick={() => enqueueUpload(selectedLocalPaths())}
                  >
                    <ArrowForwardIcon fontSize="small" />
                  </IconButton>
                </span>
              </Tooltip>
              <Tooltip title={t("sftp.download_to_local")}>
                <span>
                  <IconButton
                    size="small"
                    sx={{ p: 0.25 }}
                    disabled={selectionCount === 0 || !localDir || localNavigating}
                    onClick={() =>
                      enqueueDownload(
                        selectedFiles().map((file) =>
                          remoteJoin(currentPath, file.name),
                        ),
                      )
                    }
                  >
                    <ArrowBackIcon fontSize="small" />
                  </IconButton>
                </span>
              </Tooltip>
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <SftpNavbar
                currentPath={currentPath}
                onPathChange={async (path: string) => {
                  try {
                    setFullScreenLoading(true);
                    const applied = await refreshFileList(path);
                    if (!applied) return;
                    currentPathRef.current = path;
                    setCurrentPath(path);
                    clearSelection();
                  } finally {
                    if (!remoteListInFlight.current) setFullScreenLoading(false);
                  }
                }}
                onRefresh={async () => {
                  try {
                    setFullScreenLoading(true);
                    await refreshFileList();
                  } finally {
                    if (!remoteListInFlight.current) setFullScreenLoading(false);
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
            </Box>
          </Box>

          <Box sx={{ flex: 1, minHeight: 0, display: "flex" }}>
            <LocalFilePane
              entries={localEntries}
              loading={localLoading}
              selectedPaths={localSelected}
              onToggle={(path) => {
                setLocalSelected((prev) => {
                  const next = new Set(prev);
                  if (next.has(path)) next.delete(path);
                  else next.add(path);
                  return next;
                });
              }}
              onToggleAll={() => {
                if (
                  localEntries.length > 0 &&
                  localSelected.size === localEntries.length
                ) {
                  setLocalSelected(new Set());
                  return;
                }
                setLocalSelected(new Set(localEntries.map((entry) => entry.path)));
              }}
              onOpen={(entry) => {
                if (!entry.isDir) return;
                void refreshLocal(entry.path);
              }}
              pathsForDrag={pathsForLocalDrag}
              onDragOver={allowPaneDragOver}
              onDrop={(event) => handlePaneDrop(PaneSide.Local, event)}
            />
            <Box
              sx={{ width: 40, borderLeft: 1, borderRight: 1, borderColor: "divider" }}
            />
            <Box
              sx={{
                flex: 1,
                minWidth: 0,
                minHeight: 0,
                display: "flex",
                flexDirection: "column",
                position: "relative",
              }}
            >
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
                    {tf("sftp.selected_n", selectionCount)}
                  </Typography>
                  <Button
                    size="small"
                    startIcon={<DownloadIcon />}
                    onClick={() => {
                      void handleDownload(selectedFiles());
                    }}
                  >
                    {t("sftp.download")}
                  </Button>
                  <Button
                    size="small"
                    color="error"
                    startIcon={<DeleteIcon />}
                    onClick={() => openDeleteConfirm(selectedFiles())}
                  >
                    {t("common.delete")}
                  </Button>
                  <Button size="small" onClick={clearSelection}>
                    {t("sftp.deselect")}
                  </Button>
                </Toolbar>
              )}
              <Box
                component="div"
                id={`sftp-drop-${linkID}`}
                data-file-drop-target=""
                className="sftp-drop-zone"
                onContextMenu={(e) => handleBlankContextMenu(e)}
                onDragOver={allowPaneDragOver}
                onDrop={(event) => handlePaneDrop(PaneSide.Remote, event)}
                sx={{ flex: 1, overflow: "auto", position: "relative" }}
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
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      minHeight: "100%",
                    }}
                  >
                    <TableContainer component={Paper} sx={{ boxShadow: "none" }}>
                      <Table stickyHeader size="small">
                        <TableHead>
                          <TableRow>
                            <TableCell padding="checkbox">
                              <Checkbox
                                size="small"
                                indeterminate={
                                  selectionCount > 0 && !allSelected
                                }
                                checked={allSelected}
                                onChange={toggleSelectAll}
                              />
                            </TableCell>
                            <TableCell>{t("sftp.type")}</TableCell>
                            <TableCell>{t("sftp.name")}</TableCell>
                            <TableCell align="center">{t("sftp.size")}</TableCell>
                            <TableCell align="center">{t("sftp.modified")}</TableCell>
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
                                draggable
                                sx={{
                                  cursor: file.isDir ? "pointer" : "default",
                                }}
                                onContextMenu={(e) =>
                                  handleContextMenu(e, file)
                                }
                                onDoubleClick={() => handleItemClick(file)}
                                onDragStart={(event) => {
                                  if (dragStartsOnControl(event.target)) {
                                    event.preventDefault();
                                    return;
                                  }
                                  writePaneDrag(event.dataTransfer, {
                                    side: PaneSide.Remote,
                                    paths: pathsForRemoteDrag(file),
                                  });
                                }}
                              >
                                <TableCell padding="checkbox">
                                  <Checkbox
                                    size="small"
                                    checked={selected}
                                    onChange={() => {
                                      setSelectedNames((prev) => {
                                        const next = new Set(prev);
                                        if (next.has(file.name))
                                          next.delete(file.name);
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
                                  <Box
                                    sx={{
                                      display: "flex",
                                      alignItems: "center",
                                    }}
                                  >
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
                                  {file.isDir
                                    ? "-"
                                    : formatFileSize(file.size)}
                                </TableCell>
                                <TableCell align="center">
                                  {new Date(file.modTime).toLocaleString()}
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    </TableContainer>
                    <Box sx={{ flex: 1, minHeight: 48 }} />
                  </Box>
                )}
              </Box>
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
                      {t("sftp.please_wait")}
                    </Typography>
                  </Box>
                </Box>
              )}
            </Box>
          </Box>
        </>
      )}

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
        {editTarget && (
          <MenuItem onClick={() => void handleEdit(editTarget)}>
            <ListItemIcon>
              <EditIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>{t("common.edit")}</ListItemText>
          </MenuItem>
        )}
        <MenuItem onClick={() => void handleDownload(menuActionTargets())}>
          <ListItemIcon>
            <DownloadIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>
            {menuTargetCount > 1 ? tf("sftp.download_with_count", menuTargetCount) : t("sftp.download")}
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
            {menuTargetCount > 1 ? tf("sftp.delete_with_count", menuTargetCount) : t("common.delete")}
          </ListItemText>
        </MenuItem>
        {menuTargetCount <= 1 && (
          <MenuItem onClick={handleRenameClick}>
            <ListItemIcon>
              <RenameIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>{t("common.rename")}</ListItemText>
          </MenuItem>
        )}
        {menuTargetCount <= 1 && (
          <MenuItem onClick={() => void openChmod()}>
            <ListItemIcon>
              <ChmodIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>{t("sftp.chmod")}</ListItemText>
          </MenuItem>
        )}
        {menuTargetCount <= 1 && (
          <MenuItem onClick={openChown}>
            <ListItemIcon>
              <ChownIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>{t("sftp.chown")}</ListItemText>
          </MenuItem>
        )}
        <Divider />
        <MenuItem onClick={handleCreateFile}>
          <ListItemIcon>
            <CreateFileIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.new_file")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={handleCreateDirectory}>
          <ListItemIcon>
            <CreateFolderIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.new_folder")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => void handleUpload("file")}>
          <ListItemIcon>
            <UploadIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.upload_file")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => void handleUpload("directory")}>
          <ListItemIcon>
            <FolderOpenIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.upload_folder")}</ListItemText>
        </MenuItem>
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
          <ListItemText>{t("sftp.new_file")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={handleCreateDirectory}>
          <ListItemIcon>
            <CreateFolderIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.new_folder")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => void handleUpload("file")}>
          <ListItemIcon>
            <UploadIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.upload_file")}</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => void handleUpload("directory")}>
          <ListItemIcon>
            <FolderOpenIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>{t("sftp.upload_folder")}</ListItemText>
        </MenuItem>
      </Menu>

      <Dialog
        open={deleteConfirmOpen}
        onClose={() => setDeleteConfirmOpen(false)}
      >
        <DialogTitle>{t("sftp.confirm_delete_title")}</DialogTitle>
        <DialogContent>
          <Typography>
            {filesToDelete.length === 1
              ? tf("sftp.confirm_delete_one", filesToDelete[0]?.name ?? "") : tf("sftp.confirm_delete_many", filesToDelete.length)}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteConfirmOpen(false)}>{t("common.cancel")}</Button>
          <Button onClick={() => void handleDelete()} color="error">
                    {t("common.delete")}
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
        <DialogTitle>{t("sftp.chmod_title")}</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {chmodTarget?.name}
          </Typography>
          <TextField
            autoFocus
            margin="dense"
            label={t("sftp.octal_permission")}
            fullWidth
            value={chmodOctal}
            onChange={(e) => void onChmodOctalChange(e.target.value)}
            helperText={
              chmodSymbolic ? tf("sftp.symbolic_prefix", chmodSymbolic) : t("sftp.chmod_example")
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
            {t("common.cancel")}
          </Button>
          <Button onClick={() => void confirmChmod()} variant="contained">
            {t("common.ok")}
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
        <DialogTitle>{t("sftp.chown_title")}</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {tf("sftp.chown_requires_privilege", chownTarget?.name ?? "")}
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
            {t("common.cancel")}
          </Button>
          <Button onClick={() => void confirmChown()} variant="contained">
            {t("common.ok")}
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
        <DialogTitle>{t("sftp.new_file")}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label={t("sftp.filename")}
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
            {t("common.cancel")}
          </Button>
          <Button onClick={() => void handleCreateFileConfirm()} variant="contained">
            {t("sftp.create")}
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
        <DialogTitle>{t("sftp.new_folder")}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label={t("sftp.folder_name")}
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
            {t("common.cancel")}
          </Button>
          <Button
            onClick={() => void handleCreateDirectoryConfirm()}
            variant="contained"
          >
            {t("sftp.create")}
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
        <DialogTitle>{t("common.rename")}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label={t("sftp.new_name")}
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
            {t("common.cancel")}
          </Button>
          <Button onClick={() => void handleRename()} variant="contained">
            {t("common.rename")}
          </Button>
        </DialogActions>
      </Dialog>

    </Box>
  );
};

export default Sftp;
