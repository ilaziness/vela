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
import AddIcon from "@mui/icons-material/Add";
import ClearIcon from "@mui/icons-material/Clear";
import { Provider } from "../../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { SearchProvider } from "../../../bindings/github.com/ilaziness/vela/internal/ai/tool/models";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services";
import type {
  MCPServerInfo,
  SkillInfo,
} from "../../../bindings/github.com/ilaziness/vela/internal/ai/models";
import { useAIConfigStore } from "../../stores/aiConfig";
import { useMessageStore } from "../../stores/message";
import { parseCallServiceError } from "../../func/service";
import FormRow from "../FormRow";

const MSG_TITLE = "AI 助手设置";
const MSG_TAB_GENERAL = "常规";
const MSG_TAB_TOOLS = "工具";
const MSG_SAVE_OK = "AI 配置保存成功";
const MSG_SAVE_FAIL = "AI 配置保存失败";
const MSG_RESET_OK = "已重置为默认配置";
const MSG_RESET_TOOLS_OK = "已重置工具配置";
const MSG_RESET_FAIL = "重置配置失败";
const MSG_ENABLE_AI = "启用 AI 助手";
const MSG_ENABLED = "已启用";
const MSG_DISABLED = "已禁用";
const MSG_PROVIDER = "提供商";
const MSG_SELECT_PROVIDER = "选择提供商";
const MSG_MODEL = "模型";
const MSG_ENDPOINT = "API 端点";
const MSG_MAX_TOKENS = "最大 Token 数";
const MSG_MAX_TOKENS_HELP =
  "限制单次回复的最大生成长度；数值越大可输出越长，但响应更慢、占用更多资源。";
const MSG_ECHO = "命令回显到终端";
const MSG_ECHO_HELP =
  "将 AI 批准执行的命令注解显示到当前 SSH 终端（默认关闭；不会重复执行）";
const MSG_RESET = "重置为默认";
const MSG_RESET_TOOLS = "重置工具配置";
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
const MSG_FS_TITLE = "本地文件系统";
const MSG_FS_HELP =
  "配置允许访问的目录后，AI 才可使用读写/列表工具；每次操作都需要批准。";
const MSG_FS_EMPTY = "未配置允许目录时，本地文件工具不可用。";
const MSG_FS_ADD = "添加";
const MSG_FS_DIR = "允许目录";
const MSG_FS_DIR_REQUIRED = "请先输入绝对路径的目录";
const MSG_SEARCH_TITLE = "网络搜索";
const MSG_SEARCH_HELP =
  "选择提供商并填写 API Key 后启用 web_search；未配置则工具不可用";
const MSG_SEARCH_PROVIDER = "搜索提供商";
const MSG_SEARCH_NONE = "未配置（禁用）";

/** MUI Select 对空字符串 value 支持不佳，用哨兵表示「未配置」。 */
const SEARCH_PROVIDER_NONE = "__none__";

/** Shown in password fields when a key is already stored (plaintext is never returned). */
const API_KEY_MASK = "••••••••";

function SecretKeyField({
  value,
  encrypted,
  onChange,
}: {
  value?: string;
  encrypted?: string;
  onChange: (next: string) => void;
}) {
  const hasSaved = Boolean(encrypted);
  const display = value || (hasSaved ? API_KEY_MASK : "");
  return (
    <TextField
      fullWidth
      size="small"
      type="password"
      autoComplete="new-password"
      value={display}
      placeholder={hasSaved ? API_KEY_MASK : "API Key"}
      onFocus={(e) => {
        if (!value && hasSaved) {
          e.target.select();
        }
      }}
      onChange={(e) => {
        const next = e.target.value;
        if (next === API_KEY_MASK) {
          return;
        }
        onChange(next);
      }}
    />
  );
}

enum AISettingsTab {
  General = "general",
  Tools = "tools",
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
    webSearchProviders,
    loadConfig,
    saveConfig,
    resetConfig,
    resetToolsConfig,
    loadProviders,
    loadWebSearchProviders,
    updatePartialConfig,
    clearError,
  } = useAIConfigStore();
  const { successMessage, errorMessage } = useMessageStore();

  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [tab, setTab] = useState<AISettingsTab>(AISettingsTab.General);
  const [mcpVisited, setMcpVisited] = useState(false);
  const [skillsVisited, setSkillsVisited] = useState(false);
  const [newAllowDir, setNewAllowDir] = useState("");

  useEffect(() => {
    if (tab === AISettingsTab.MCP) setMcpVisited(true);
    if (tab === AISettingsTab.Skills) setSkillsVisited(true);
  }, [tab]);

  useEffect(() => {
    loadConfig();
    loadProviders();
    loadWebSearchProviders();
  }, [loadConfig, loadProviders, loadWebSearchProviders]);

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

  const handleResetTools = async () => {
    setResetting(true);
    try {
      await resetToolsConfig();
      successMessage(MSG_RESET_TOOLS_OK);
    } catch (err) {
      errorMessage(parseCallServiceError(err) || MSG_RESET_FAIL);
    } finally {
      setResetting(false);
    }
  };

  const currentProvider = providers.find((p) => p.name === config?.provider);
  const currentSearchProvider = webSearchProviders.find(
    (p) => p.name === config?.web_search?.provider,
  );
  const allowedDirs = config?.filesystem?.allowed_dirs ?? [];

  const setAllowedDirs = (dirs: string[]) => {
    updatePartialConfig({
      filesystem: { allowed_dirs: dirs },
    });
  };

  const addAllowedDir = async () => {
    const dir = newAllowDir.trim();
    if (!dir) {
      errorMessage(MSG_FS_DIR_REQUIRED);
      return;
    }
    try {
      const abs = await AIService.ValidateAllowedDir(dir);
      const normalized = (abs || "").trim();
      if (!normalized) {
        errorMessage(MSG_FS_DIR_REQUIRED);
        return;
      }
      const win = navigator.userAgent.includes("Windows");
      const already = allowedDirs.some((d) =>
        win ? d.toLowerCase() === normalized.toLowerCase() : d === normalized,
      );
      if (already) {
        setNewAllowDir("");
        return;
      }
      setAllowedDirs([...allowedDirs, normalized]);
      setNewAllowDir("");
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const removeAllowedDir = (dir: string) => {
    setAllowedDirs(allowedDirs.filter((d) => d !== dir));
  };

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
          <Tab label={MSG_TAB_TOOLS} value={AISettingsTab.Tools} />
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
                        onChange={(e) =>
                          updatePartialConfig({ enabled: e.target.checked })
                        }
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
                        updatePartialConfig({
                          provider: e.target.value as Provider,
                        })
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
                    onChange={(e) =>
                      updatePartialConfig({ model: e.target.value })
                    }
                    placeholder="llama3.2, gpt-4"
                  />
                </FormRow>

                {currentProvider?.needs_endpoint && (
                  <FormRow label={MSG_ENDPOINT}>
                    <TextField
                      fullWidth
                      size="small"
                      value={config.endpoint}
                      onChange={(e) =>
                        updatePartialConfig({ endpoint: e.target.value })
                      }
                      placeholder="http://localhost:11434"
                    />
                  </FormRow>
                )}

                {currentProvider?.needs_api_key && (
                  <FormRow label="API Key">
                    <SecretKeyField
                      value={config.api_key}
                      encrypted={config.api_key_encrypted}
                      onChange={(api_key) => updatePartialConfig({ api_key })}
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
                      updatePartialConfig({
                        max_tokens: isNaN(value) ? 0 : value,
                      });
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
                          updatePartialConfig({
                            echo_ssh_commands: e.target.checked,
                          })
                        }
                      />
                    }
                    label={MSG_ECHO_HELP}
                  />
                </FormRow>

                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "flex-end",
                    gap: 2,
                    mt: 3,
                  }}
                >
                  <Button
                    variant="outlined"
                    onClick={handleReset}
                    loading={resetting}
                  >
                    {MSG_RESET}
                  </Button>
                  <Button
                    variant="contained"
                    onClick={handleSave}
                    loading={saving}
                    disabled={!config}
                  >
                    {MSG_SAVE}
                  </Button>
                </Box>
              </Stack>
            </>
          )}

          {tab === AISettingsTab.Tools && (
            <Stack spacing={2}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {MSG_FS_TITLE}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {MSG_FS_HELP}
              </Typography>
              {allowedDirs.length === 0 && (
                <Alert severity="info">{MSG_FS_EMPTY}</Alert>
              )}
              {allowedDirs.length > 0 && (
                <List dense disablePadding>
                  {allowedDirs.map((dir) => (
                    <ListItem
                      key={dir}
                      secondaryAction={
                        <IconButton
                          edge="end"
                          aria-label="delete"
                          onClick={() => removeAllowedDir(dir)}
                        >
                          <ClearIcon fontSize="small" />
                        </IconButton>
                      }
                    >
                      <ListItemText
                        primary={dir}
                        slotProps={{
                          primary: { sx: { wordBreak: "break-all" } },
                        }}
                      />
                    </ListItem>
                  ))}
                </List>
              )}
              <Box sx={{ display: "flex", gap: 1, width: "100%" }}>
                <TextField
                  size="small"
                  label={MSG_FS_DIR}
                  value={newAllowDir}
                  onChange={(e) => setNewAllowDir(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addAllowedDir();
                    }
                  }}
                  placeholder="绝对路径，如 D:\\work 或 /home/user/projects"
                  sx={{ flex: 1, minWidth: 0 }}
                />
                <Button
                  type="button"
                  variant="outlined"
                  startIcon={<AddIcon />}
                  onClick={addAllowedDir}
                  sx={{
                    flexShrink: 0,
                    whiteSpace: "nowrap",
                    alignSelf: "center",
                  }}
                >
                  {MSG_FS_ADD}
                </Button>
              </Box>

              <Divider sx={{ my: 1 }} />

              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {MSG_SEARCH_TITLE}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {MSG_SEARCH_HELP}
              </Typography>

              <FormRow label={MSG_SEARCH_PROVIDER}>
                <FormControl fullWidth size="small">
                  <InputLabel>{MSG_SELECT_PROVIDER}</InputLabel>
                  <Select
                    value={
                      config.web_search?.provider
                        ? String(config.web_search.provider)
                        : SEARCH_PROVIDER_NONE
                    }
                    label={MSG_SELECT_PROVIDER}
                    onChange={(e) => {
                      const raw = String(e.target.value);
                      const provider =
                        raw === SEARCH_PROVIDER_NONE
                          ? SearchProvider.$zero
                          : (raw as SearchProvider);
                      // Provider-scoped keys: never keep another vendor's ciphertext.
                      updatePartialConfig({
                        web_search: {
                          provider,
                          api_key: "",
                          api_key_encrypted: undefined,
                        },
                      });
                    }}
                  >
                    <MenuItem value={SEARCH_PROVIDER_NONE}>
                      {MSG_SEARCH_NONE}
                    </MenuItem>
                    {webSearchProviders.map((provider) => (
                      <MenuItem key={provider.name} value={provider.name}>
                        {provider.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </FormRow>

              {currentSearchProvider && (
                <FormRow label="API Key">
                  <SecretKeyField
                    value={config.web_search?.api_key}
                    encrypted={config.web_search?.api_key_encrypted}
                    onChange={(api_key) =>
                      updatePartialConfig({
                        web_search: {
                          ...config.web_search,
                          provider:
                            config.web_search?.provider ?? SearchProvider.$zero,
                          api_key,
                          api_key_encrypted:
                            config.web_search?.api_key_encrypted,
                        },
                      })
                    }
                  />
                </FormRow>
              )}

              <Box
                sx={{
                  display: "flex",
                  justifyContent: "flex-end",
                  gap: 2,
                  mt: 3,
                }}
              >
                <Button
                  variant="outlined"
                  onClick={handleResetTools}
                  loading={resetting}
                >
                  {MSG_RESET_TOOLS}
                </Button>
                <Button
                  variant="contained"
                  onClick={handleSave}
                  loading={saving}
                  disabled={!config}
                >
                  {MSG_SAVE}
                </Button>
              </Box>
            </Stack>
          )}

          {mcpVisited && (
            <Box sx={{ display: tab === AISettingsTab.MCP ? "block" : "none" }}>
              <MCPSettingsPanel />
            </Box>
          )}
          {skillsVisited && (
            <Box
              sx={{ display: tab === AISettingsTab.Skills ? "block" : "none" }}
            >
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
        <Button
          startIcon={<OpenInNewIcon />}
          variant="outlined"
          onClick={() => void onOpen()}
        >
          {MSG_OPEN_MCP}
        </Button>
        <Button
          startIcon={
            loading ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <RefreshIcon />
            )
          }
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
              <AvailabilityDot
                enabled={s.enabled}
                available={s.available}
                error={s.error}
              />
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
        <Button
          startIcon={<FolderOpenIcon />}
          variant="outlined"
          onClick={() => void onOpenDir()}
        >
          {MSG_OPEN_SKILLS}
        </Button>
        <Button
          startIcon={
            loading ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <RefreshIcon />
            )
          }
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
                  <IconButton
                    edge="end"
                    aria-label="open"
                    onClick={() => void onOpenSkill(s.name)}
                  >
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
              <ListItemText
                primary={s.name}
                secondary={s.description || undefined}
              />
            </ListItem>
          ))}
        </List>
      )}
    </>
  );
}

export default AISettings;
