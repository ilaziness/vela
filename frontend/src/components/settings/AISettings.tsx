import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Paper,
  TextField,
  Button,
  Stack,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Switch,
  FormControlLabel,
  Slider,
  Divider,
  Alert,
  CircularProgress,
  Tabs,
  Tab,
  List,
  ListItem,
  ListItemText,
  Tooltip,
  IconButton,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import FolderOpenIcon from "@mui/icons-material/FolderOpen";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import { Provider } from "../../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services";
import type { MCPServerInfo, SkillInfo } from "../../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { useAIConfigStore } from "../../stores/aiConfig";
import { useMessageStore } from "../../stores/message";
import { parseCallServiceError } from "../../func/service";
import FormRow from "../FormRow";

const MSG_TITLE = "AI 助手设置";
const MSG_TAB_GENERAL = "常规";
const MSG_SAVE_OK = "AI 配置保存成功";
const MSG_SAVE_FAIL = "AI 配置保存失败";
const MSG_RESET_OK = "已重置为默认配置";
const MSG_RESET_FAIL = "重置配置失败";
const MSG_ENABLE_AI = "启用 AI 助手";
const MSG_ENABLED = "已启用";
const MSG_DISABLED = "已禁用";
const MSG_PROVIDER = "提供商";
const MSG_SELECT_PROVIDER = "选择提供商";
const MSG_MODEL = "模型";
const MSG_ENDPOINT = "API 端点";
const MSG_MAX_TOKENS = "最大 Token 数";
const MSG_MAX_TOKENS_HELP = "限制单次回复的最大生成长度；数值越大可输出越长，但响应更慢、占用更多资源。";
const MSG_ECHO = "命令回显到终端";
const MSG_ECHO_HELP = "将 AI 批准执行的命令注解显示到当前 SSH 终端（默认关闭；不会重复执行）";
const MSG_RESET = "重置为默认";
const MSG_SAVE = "保存";
const MSG_AVAILABLE = "可用";
const MSG_UNAVAILABLE = "不可用";
const MSG_OPEN_MCP = "打开 mcp.json";
const MSG_REFRESH = "刷新";
const MSG_MCP_EMPTY = "暂无 MCP 服务器，请编辑 mcp.json 后刷新。";
const MSG_MCP_ON = "已启用 MCP";
const MSG_MCP_OFF = "已禁用 MCP";
const MSG_OPEN_SKILLS = "打开 skills 目录";
const MSG_SKILLS_EMPTY = "暂无 Skill，请在 skills 目录添加后刷新。";
const MSG_SKILL_ON = "已启用 Skill";
const MSG_SKILL_OFF = "已禁用 Skill";

enum AISettingsTab {
  General = "general",
  MCP = "mcp",
  Skills = "skills",
}

const AISettings: React.FC = () => {
  const {
    config,
    isLoading,
    error,
    providers,
    isLoadingProviders,
    loadConfig,
    saveConfig,
    resetConfig,
    loadProviders,
    updatePartialConfig,
    clearError,
  } = useAIConfigStore();
  const { successMessage, errorMessage } = useMessageStore();

  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [tab, setTab] = useState<AISettingsTab>(AISettingsTab.General);
  const [mcpVisited, setMcpVisited] = useState(false);
  const [skillsVisited, setSkillsVisited] = useState(false);

  useEffect(() => {
    if (tab === AISettingsTab.MCP) setMcpVisited(true);
    if (tab === AISettingsTab.Skills) setSkillsVisited(true);
  }, [tab]);

  useEffect(() => {
    loadConfig();
    loadProviders();
  }, [loadConfig, loadProviders]);

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await saveConfig(config);
      successMessage(MSG_SAVE_OK);
    } catch (err) {
      errorMessage(parseCallServiceError(err) || MSG_SAVE_FAIL);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    setResetting(true);
    try {
      await resetConfig();
      successMessage(MSG_RESET_OK);
    } catch (err) {
      errorMessage(parseCallServiceError(err) || MSG_RESET_FAIL);
    } finally {
      setResetting(false);
    }
  };

  const currentProvider = providers.find((p) => p.name === config?.provider);

  if (isLoading || !config) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Typography variant="h5" gutterBottom sx={{ mb: 2, fontWeight: 600 }}>
        {MSG_TITLE}
      </Typography>

      <Paper elevation={1}>
        <Tabs
          value={tab}
          onChange={(_, v: AISettingsTab) => setTab(v)}
          sx={{ px: 1, borderBottom: 1, borderColor: "divider" }}
        >
          <Tab label={MSG_TAB_GENERAL} value={AISettingsTab.General} />
          <Tab label="MCP" value={AISettingsTab.MCP} />
          <Tab label="Skills" value={AISettingsTab.Skills} />
        </Tabs>

        <Box sx={{ px: 3, pt: 1.5, pb: 3 }}>
          {tab === AISettingsTab.General && (
            <>
              {error && (
                <Alert severity="error" sx={{ mb: 2 }} onClose={clearError}>
                  {error}
                </Alert>
              )}

              <Stack spacing={2}>
                <FormRow label={MSG_ENABLE_AI}>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={config.enabled}
                        onChange={(e) => updatePartialConfig({ enabled: e.target.checked })}
                      />
                    }
                    label={config.enabled ? MSG_ENABLED : MSG_DISABLED}
                  />
                </FormRow>

                <Divider sx={{ my: 1 }} />

                <FormRow label={MSG_PROVIDER}>
                  <FormControl fullWidth size="small">
                    <InputLabel>{MSG_SELECT_PROVIDER}</InputLabel>
                    <Select
                      value={config.provider}
                      label={MSG_SELECT_PROVIDER}
                      onChange={(e) =>
                        updatePartialConfig({ provider: e.target.value as Provider })
                      }
                      disabled={isLoadingProviders}
                    >
                      {providers.map((provider) => (
                        <MenuItem key={provider.name} value={provider.name}>
                          {provider.label}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </FormRow>

                <FormRow label={MSG_MODEL}>
                  <TextField
                    fullWidth
                    size="small"
                    value={config.model}
                    onChange={(e) => updatePartialConfig({ model: e.target.value })}
                    placeholder="llama3.2, gpt-4"
                  />
                </FormRow>

                {currentProvider?.needs_endpoint && (
                  <FormRow label={MSG_ENDPOINT}>
                    <TextField
                      fullWidth
                      size="small"
                      value={config.endpoint}
                      onChange={(e) => updatePartialConfig({ endpoint: e.target.value })}
                      placeholder="http://localhost:11434"
                    />
                  </FormRow>
                )}

                {currentProvider?.needs_api_key && (
                  <FormRow label="API Key">
                    <TextField
                      fullWidth
                      size="small"
                      type="password"
                      value={config.api_key}
                      onChange={(e) => updatePartialConfig({ api_key: e.target.value })}
                      placeholder="API Key"
                    />
                  </FormRow>
                )}

                <FormRow label="Temperature">
                  <Box sx={{ width: "100%", px: 1 }}>
                    <Slider
                      value={config.temperature}
                      onChange={(_, value) =>
                        updatePartialConfig({ temperature: value as number })
                      }
                      min={0}
                      max={2}
                      step={0.1}
                      marks={[
                        { value: 0, label: "0" },
                        { value: 1, label: "1" },
                        { value: 2, label: "2" },
                      ]}
                      valueLabelDisplay="auto"
                    />
                  </Box>
                </FormRow>

                <FormRow label={MSG_MAX_TOKENS}>
                  <TextField
                    fullWidth
                    size="small"
                    type="number"
                    value={config.max_tokens}
                    onChange={(e) => {
                      const value = parseInt(e.target.value, 10);
                      updatePartialConfig({ max_tokens: isNaN(value) ? 0 : value });
                    }}
                    slotProps={{ htmlInput: { min: 100, max: 8192 } }}
                    helperText={MSG_MAX_TOKENS_HELP}
                  />
                </FormRow>

                <Divider />

                <FormRow label={MSG_ECHO}>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={Boolean(config.echo_ssh_commands)}
                        onChange={(e) =>
                          updatePartialConfig({ echo_ssh_commands: e.target.checked })
                        }
                      />
                    }
                    label={MSG_ECHO_HELP}
                  />
                </FormRow>

                <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 2, mt: 3 }}>
                  <Button variant="outlined" onClick={handleReset} loading={resetting}>
                    {MSG_RESET}
                  </Button>
                  <Button variant="contained" onClick={handleSave} loading={saving} disabled={!config}>
                    {MSG_SAVE}
                  </Button>
                </Box>
              </Stack>
            </>
          )}

          {mcpVisited && (
            <Box sx={{ display: tab === AISettingsTab.MCP ? "block" : "none" }}>
              <MCPSettingsPanel />
            </Box>
          )}
          {skillsVisited && (
            <Box sx={{ display: tab === AISettingsTab.Skills ? "block" : "none" }}>
              <SkillsSettingsPanel />
            </Box>
          )}
        </Box>
      </Paper>
    </Box>
  );
};

function AvailabilityDot({
  enabled,
  available,
  error,
}: {
  enabled: boolean;
  available: boolean;
  error?: string;
}) {
  if (!enabled) return null;
  // Unprobed cache hydrate: available=false and no error — hide the dot.
  if (!available && !error) return null;
  const color = available ? "success.main" : "error.main";
  const tip = available ? MSG_AVAILABLE : error || MSG_UNAVAILABLE;
  return (
    <Tooltip title={tip}>
      <Box
        component="span"
        sx={{
          width: 10,
          height: 10,
          borderRadius: "50%",
          bgcolor: color,
          display: "inline-block",
          mr: 1,
          flexShrink: 0,
        }}
      />
    </Tooltip>
  );
}

function MCPSettingsPanel() {
  const { successMessage, errorMessage } = useMessageStore();
  const [loading, setLoading] = useState(false);
  const [servers, setServers] = useState<MCPServerInfo[]>([]);
  const [busyName, setBusyName] = useState("");
  const [ready, setReady] = useState(false);

  const applyList = (list: (MCPServerInfo | null)[] | null) => {
    setServers((list || []).filter((s): s is MCPServerInfo => s != null));
  };

  // First open: show cached status only (no MCP probe).
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await AIService.ListMCPServers();
        if (!cancelled) applyList(list);
      } catch (err) {
        if (!cancelled) errorMessage(parseCallServiceError(err));
      } finally {
        if (!cancelled) setReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [errorMessage]);

  const onRefresh = async () => {
    setLoading(true);
    try {
      const list = await AIService.ReloadMCPServers();
      applyList(list);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setLoading(false);
    }
  };

  const onToggle = async (name: string, enabled: boolean) => {
    setBusyName(name);
    try {
      const list = await AIService.SetMCPServerEnabled(name, enabled);
      applyList(list);
      successMessage(enabled ? MSG_MCP_ON : MSG_MCP_OFF);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setBusyName("");
    }
  };

  const onOpen = async () => {
    try {
      await AIService.OpenMCPConfig();
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const toggling = busyName !== "";

  return (
    <>
      <Box sx={{ display: "flex", gap: 1, mb: 2 }}>
        <Button startIcon={<OpenInNewIcon />} variant="outlined" onClick={() => void onOpen()}>
          {MSG_OPEN_MCP}
        </Button>
        <Button
          startIcon={loading ? <CircularProgress size={16} color="inherit" /> : <RefreshIcon />}
          variant="outlined"
          onClick={() => void onRefresh()}
          disabled={loading || toggling}
        >
          {MSG_REFRESH}
        </Button>
      </Box>
      {!ready ? null : servers.length === 0 ? (
        <Typography color="text.secondary">{MSG_MCP_EMPTY}</Typography>
      ) : (
        <List disablePadding>
          {servers.map((s) => (
            <ListItem
              key={s.name}
              divider
              sx={{ px: 0, pr: 8 }}
              secondaryAction={
                <Switch
                  edge="end"
                  checked={s.enabled}
                  disabled={toggling || loading}
                  onChange={(e) => void onToggle(s.name, e.target.checked)}
                />
              }
            >
              <AvailabilityDot enabled={s.enabled} available={s.available} error={s.error} />
              <ListItemText
                primary={s.name}
                secondary={[s.transport, s.summary].filter(Boolean).join(" · ")}
              />
            </ListItem>
          ))}
        </List>
      )}
    </>
  );
}

function SkillsSettingsPanel() {
  const { successMessage, errorMessage } = useMessageStore();
  const [loading, setLoading] = useState(true);
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [busyName, setBusyName] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const list = await AIService.ListSkills();
      setSkills((list || []).filter((s): s is SkillInfo => s != null));
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setLoading(false);
    }
  }, [errorMessage]);

  useEffect(() => {
    void load();
  }, [load]);

  const onToggle = async (name: string, enabled: boolean) => {
    setBusyName(name);
    try {
      await AIService.SetSkillEnabled(name, enabled);
      const list = await AIService.ListSkills();
      setSkills((list || []).filter((s): s is SkillInfo => s != null));
      successMessage(enabled ? MSG_SKILL_ON : MSG_SKILL_OFF);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    } finally {
      setBusyName("");
    }
  };

  const onOpenDir = async () => {
    try {
      await AIService.OpenSkillsDir();
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const onOpenSkill = async (name: string) => {
    try {
      await AIService.OpenSkillDir(name);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  return (
    <>
      <Box sx={{ display: "flex", gap: 1, mb: 2 }}>
        <Button startIcon={<FolderOpenIcon />} variant="outlined" onClick={() => void onOpenDir()}>
          {MSG_OPEN_SKILLS}
        </Button>
        <Button
          startIcon={loading ? <CircularProgress size={16} color="inherit" /> : <RefreshIcon />}
          variant="outlined"
          onClick={() => void load()}
          disabled={loading || busyName !== ""}
        >
          {MSG_REFRESH}
        </Button>
      </Box>
      {loading && skills.length === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
          <CircularProgress size={28} />
        </Box>
      ) : skills.length === 0 ? (
        <Typography color="text.secondary">{MSG_SKILLS_EMPTY}</Typography>
      ) : (
        <List disablePadding>
          {skills.map((s) => (
            <ListItem
              key={s.name}
              divider
              sx={{ px: 0, pr: 12 }}
              secondaryAction={
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                  <IconButton edge="end" aria-label="open" onClick={() => void onOpenSkill(s.name)}>
                    <FolderOpenIcon fontSize="small" />
                  </IconButton>
                  <Switch
                    edge="end"
                    checked={s.enabled}
                    disabled={busyName !== ""}
                    onChange={(e) => void onToggle(s.name, e.target.checked)}
                  />
                </Box>
              }
            >
              <ListItemText primary={s.name} secondary={s.description || undefined} />
            </ListItem>
          ))}
        </List>
      )}
    </>
  );
}

export default AISettings;
