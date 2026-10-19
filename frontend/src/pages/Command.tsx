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
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";
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
      errorMessage(`加载命令失败：${parseCallServiceError(error)}`);
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
      errorMessage(`获取会话列表失败：${parseCallServiceError(error)}`);
    }
  }, [errorMessage]);

  const loadHistory = useCallback(async () => {
    try {
      const historyData = await CommandService.GetCommandHistory();
      setHistory(historyData as CommandHistory[]);
    } catch (error) {
      errorMessage(`加载历史失败：${parseCallServiceError(error)}`);
    }
  }, [errorMessage]);

  // 加载命令和会话
  useEffect(() => {
    void loadCommands();
    void loadSessions();
    void loadHistory();
  }, [loadCommands, loadSessions, loadHistory]);

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
      errorMessage("命令不能为空");
      return;
    }

    if (selectedSessions.length === 0) {
      errorMessage("请选择至少一个 SSH 会话");
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

      successMessage("命令发送成功");
      setInputCommand("");
    } catch (error) {
      errorMessage(`发送命令失败：${parseCallServiceError(error)}`);
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
      errorMessage("分类、名称和命令都不能为空");
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
      successMessage("命令添加成功");
    } catch (error) {
      errorMessage(`添加命令失败：${parseCallServiceError(error)}`);
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
      successMessage("命令删除成功");
    } catch (error) {
      errorMessage(`删除命令失败：${parseCallServiceError(error)}`);
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
      successMessage("历史已清空");
    } catch (error) {
      errorMessage(`清空历史失败：${parseCallServiceError(error)}`);
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
              命令库
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
        <DialogTitle>确认删除</DialogTitle>
        <DialogContent>
          <Typography>
            确定要删除命令 "{commandToDelete?.name}" 吗？此操作不可撤销。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteConfirmOpen(false)}>取消</Button>
          <Button onClick={confirmDeleteCommand} color="error">
            删除
          </Button>
        </DialogActions>
      </Dialog>

      {/* 清空历史确认对话框 */}
      <Dialog
        open={clearHistoryConfirmOpen}
        onClose={() => setClearHistoryConfirmOpen(false)}
      >
        <DialogTitle>确认清空</DialogTitle>
        <DialogContent>
          <Typography>
            确定要清空所有命令历史记录吗？此操作不可撤销。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setClearHistoryConfirmOpen(false)}>取消</Button>
          <Button onClick={confirmClearHistory} color="error">
            清空
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
