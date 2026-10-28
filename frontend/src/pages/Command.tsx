import React, { useState, useEffect, useCallback } from "react";
import {
  Box,
  Typography,
  Paper,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
} from "@mui/material";
import { Events } from "@wailsio/runtime";
import { useMessageStore } from "../stores/message";
import { translateServiceError } from "../func/service";
import { t, tf } from "../i18n";
import {
  CommandHistory,
  SSHTunnelSession,
  SendCommandRequest,
  CommandInfo,
  UserCommand,
} from "../types/command";
import { SSHService, CommandService } from "../../bindings/github.com/ilaziness/vela/services";
import CommandTree from "../components/CommandTree";
import CommandHistoryList from "../components/CommandHistory";
import SessionSelector from "../components/SessionSelector";
import CommandInput from "../components/CommandInput";
import AddCommandDialog from "../components/AddCommandDialog";
import OpBar from "../components/OpBar";
import { useFrameless } from "../hooks/useFrameless";

const Command: React.FC = () => {
  const { errorMessage, successMessage } = useMessageStore();
  const frameless = useFrameless() === true;

  // 命令数据
  const [commandsByCategory, setCommandsByCategory] = useState<{
    [key: string]: CommandInfo[];
  }>({});
  const [expandedCategories, setExpandedCategories] = useState<{
    [key: string]: boolean;
  }>({});

  // SSH 会话列表
  const [sessions, setSessions] = useState<SSHTunnelSession[]>([]);
  const [selectedSessions, setSelectedSessions] = useState<SSHTunnelSession[]>(
    [],
  );

  // 命令历史
  const [history, setHistory] = useState<CommandHistory[]>([]);

  // 命令输入
  const [inputCommand, setInputCommand] = useState("");

  // 添加自定义命令对话框
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [newCategory, setNewCategory] = useState("");
  const [newName, setNewName] = useState("");
  const [newCommand, setNewCommand] = useState("");
  const [newDescription, setNewDescription] = useState("");

  // 删除确认对话框
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [commandToDelete, setCommandToDelete] = useState<{
    category: string;
    name: string;
  } | null>(null);

  // 清空历史确认对话框
  const [clearHistoryConfirmOpen, setClearHistoryConfirmOpen] = useState(false);

  const loadCommands = useCallback(async () => {
    try {
      const result = await CommandService.GetAllCommands();
      setCommandsByCategory(result as { [key: string]: CommandInfo[] });
    } catch (error) {
      errorMessage(tf("command.load_failed", translateServiceError(error)));
    }
  }, [errorMessage]);

  const loadSessions = useCallback(async () => {
    try {
      const sessionsData = (await SSHService.GetActiveSessions()) as SSHTunnelSession[];
      const next = sessionsData ?? [];
      setSessions(next);
      setSelectedSessions((prev) =>
        prev.flatMap((selected) => {
          const current = next.find((session) => session.id === selected.id);
          return current ? [current] : [];
        }),
      );
    } catch (error) {
      errorMessage(tf("command.sessions_failed", translateServiceError(error)));
    }
  }, [errorMessage]);

  const loadHistory = useCallback(async () => {
    try {
      const historyData = await CommandService.GetCommandHistory();
      setHistory(historyData as CommandHistory[]);
    } catch (error) {
      errorMessage(tf("command.history_failed", translateServiceError(error)));
    }
  }, [errorMessage]);

  useEffect(() => {
    void loadCommands();
    void loadSessions();
    void loadHistory();
  }, [loadCommands, loadSessions, loadHistory]);

  // Builtin command categories are language-specific; refetch after SetLanguage.
  useEffect(() => {
    const unsub = Events.On("eventLanguage", () => {
      setExpandedCategories({});
      void loadCommands();
    });
    return () => {
      unsub();
    };
  }, [loadCommands]);

  // 刷新会话列表
  const handleRefreshSessions = () => {
    void loadSessions();
  };

  // 切换分类展开/折叠
  const toggleCategory = (category: string) => {
    setExpandedCategories((prev) => ({
      ...prev,
      [category]: !prev[category],
    }));
  };

  // 点击命令，填充到输入框
  const handleCommandClick = (command: string) => {
    setInputCommand(command);
  };

  // 发送命令
  const handleSendCommand = async () => {
    if (!inputCommand.trim()) {
      errorMessage(t("command.empty"));
      return;
    }

    if (selectedSessions.length === 0) {
      errorMessage(t("command.select_session"));
      return;
    }

    try {
      const request: SendCommandRequest = {
        command: inputCommand,
        session_ids: selectedSessions.map((s) => s.id),
      };

      await CommandService.SendCommand(request);

      // 添加到 UI 历史列表
      const newHistory: CommandHistory = {
        timestamp: Date.now(),
        command: inputCommand,
      };
      setHistory((prev) => [...prev, newHistory]);

      successMessage(t("command.send_success"));
      setInputCommand("");
    } catch (error) {
      errorMessage(t("command.send_failed", translateServiceError(error)));
    }
  };

  // 键盘事件处理
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendCommand();
    }
    // Shift+Enter 默认行为（换行）
  };

  // 添加自定义命令
  const handleAddCommand = async () => {
    if (!newCategory.trim() || !newName.trim() || !newCommand.trim()) {
      errorMessage(t("command.fields_required"));
      return;
    }

    try {
      const userCmd: UserCommand = {
        category: newCategory,
        name: newName,
        command: newCommand,
        description: newDescription,
        created_at: Date.now(),
      };

      await CommandService.SaveUserCommand(userCmd);
      await loadCommands();
      setAddDialogOpen(false);
      setNewCategory("");
      setNewName("");
      setNewCommand("");
      setNewDescription("");
      successMessage(t("command.add_success"));
    } catch (error) {
      errorMessage(t("command.add_failed", translateServiceError(error)));
    }
  };

  // 删除用户命令
  const handleDeleteCommand = async (category: string, name: string) => {
    setCommandToDelete({ category, name });
    setDeleteConfirmOpen(true);
  };

  // 确认删除用户命令
  const confirmDeleteCommand = async () => {
    if (!commandToDelete) return;
    try {
      await CommandService.DeleteUserCommand(
        commandToDelete.category,
        commandToDelete.name,
      );
      await loadCommands();
      successMessage(t("command.delete_success"));
    } catch (error) {
      errorMessage(t("command.delete_failed", translateServiceError(error)));
    } finally {
      setDeleteConfirmOpen(false);
      setCommandToDelete(null);
    }
  };

  // 清空历史
  const handleClearHistory = () => {
    setClearHistoryConfirmOpen(true);
  };

  // 确认清空历史
  const confirmClearHistory = async () => {
    try {
      await CommandService.ClearCommandHistory();
      setHistory([]);
      successMessage(t("command.clear_history_success"));
    } catch (error) {
      errorMessage(
        t("command.clear_history_failed", translateServiceError(error)),
      );
    } finally {
      setClearHistoryConfirmOpen(false);
    }
  };

  return (
    <>
      {frameless && (
        <Box
          sx={{
            height: 40,
            flexShrink: 0,
          }}
        >
          <OpBar />
        </Box>
      )}

      {/* 主内容区 */}
      <Box
        sx={{
          height: frameless ? "calc(100% - 40px)" : "100%",
          display: "flex",
          flexDirection: "row",
          overflow: "hidden",
        }}
      >
        {/* 左侧命令树容器 */}
        <Paper
          sx={{
            width: 300,
            flexShrink: 0,
            borderRight: 1,
            borderColor: "divider",
            display: "flex",
            flexDirection: "column",
            overflow: "auto",
          }}
          elevation={0}
          square
        >
          <Box sx={{ p: 2, pb: 1.5 }}>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {t("command.library")}
            </Typography>
          </Box>
          <CommandTree
            commandsByCategory={commandsByCategory}
            expandedCategories={expandedCategories}
            onToggleCategory={toggleCategory}
            onCommandClick={handleCommandClick}
            onDeleteCommand={handleDeleteCommand}
            onAddCommand={() => setAddDialogOpen(true)}
          />
        </Paper>

        {/* 右侧面板 */}
        <Box
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
            p: 2,
          }}
        >
          {/* 命令历史列表 */}
          <CommandHistoryList
            history={history}
            onItemClick={(command) => setInputCommand(command)}
            onClearHistory={handleClearHistory}
          />

          {/* SSH 会话选择器 */}
          <SessionSelector
            sessions={sessions}
            selectedSessions={selectedSessions}
            onChange={setSelectedSessions}
            onRefresh={handleRefreshSessions}
          />

          {/* 命令输入框 */}
          <CommandInput
            inputCommand={inputCommand}
            onChange={setInputCommand}
            onKeyDown={handleKeyDown}
            onSend={handleSendCommand}
          />
        </Box>
      </Box>

      {/* 删除用户命令确认对话框 */}
      <Dialog open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)}>
        <DialogTitle>{t("command.confirm_delete_title")}</DialogTitle>
        <DialogContent>
          <Typography>
            {tf(
              "command.confirm_delete_message",
              commandToDelete?.name ?? "",
            )}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteConfirmOpen(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={confirmDeleteCommand} color="error">
            {t("common.delete")}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 清空历史确认对话框 */}
      <Dialog
        open={clearHistoryConfirmOpen}
        onClose={() => setClearHistoryConfirmOpen(false)}
      >
        <DialogTitle>{t("command.confirm_clear_title")}</DialogTitle>
        <DialogContent>
          <Typography>{t("command.confirm_clear_message")}</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setClearHistoryConfirmOpen(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={confirmClearHistory} color="error">
            {t("common.clear")}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 添加自定义命令对话框 */}
      <AddCommandDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
        category={newCategory}
        name={newName}
        command={newCommand}
        description={newDescription}
        onCategoryChange={setNewCategory}
        onNameChange={setNewName}
        onCommandChange={setNewCommand}
        onDescriptionChange={setNewDescription}
        onSave={handleAddCommand}
      />
    </>
  );
};

export default Command;
