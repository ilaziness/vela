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
import FormRow from "../FormRow";
import { t } from "../../i18n";
import { translateServiceError } from "../../func/service";

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
      placeholder={hasSaved ? API_KEY_MASK : t("settings.ai.api_key")}
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
      successMessage(t("settings.ai.save_success"));
    } catch (err) {
      errorMessage(translateServiceError(err) || t("settings.ai.save_failed"));
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    setResetting(true);
    try {
      await resetConfig();
      successMessage(t("settings.ai.reset_success"));
    } catch (err) {
      errorMessage(translateServiceError(err) || t("settings.ai.reset_failed"));
    } finally {
      setResetting(false);
    }
  };

  const handleResetTools = async () => {
    setResetting(true);
    try {
      await resetToolsConfig();
      successMessage(t("settings.ai.reset_tools_success"));
    } catch (err) {
      errorMessage(translateServiceError(err) || t("settings.ai.reset_failed"));
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
      errorMessage(t("settings.ai.filesystem_dir_required"));
      return;
    }
    try {
      const abs = await AIService.ValidateAllowedDir(dir);
      const normalized = (abs || "").trim();
      if (!normalized) {
        errorMessage(t("settings.ai.filesystem_dir_required"));
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
      errorMessage(translateServiceError(err));
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
        {t("settings.ai.title")}
      </Typography>

      <Paper elevation={1}>
        <Tabs
          value={tab}
          onChange={(_, v: AISettingsTab) => setTab(v)}
          sx={{ px: 1, borderBottom: 1, borderColor: "divider" }}
        >
          <Tab label={t("settings.ai.tab_general")} value={AISettingsTab.General} />
          <Tab label={t("settings.ai.tab_tools")} value={AISettingsTab.Tools} />
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
                <FormRow label={t("settings.ai.enable")}>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={config.enabled}
                        onChange={(e) =>
                          updatePartialConfig({ enabled: e.target.checked })
                        }
                      />
                    }
                    label={config.enabled ? t("settings.ai.enabled") : t("settings.ai.disabled")}
                  />
                </FormRow>

                <Divider sx={{ my: 1 }} />

                <FormRow label={t("settings.ai.provider")}>
                  <FormControl fullWidth size="small">
                    <InputLabel>{t("settings.ai.select_provider")}</InputLabel>
                    <Select
                      value={config.provider}
                      label={t("settings.ai.select_provider")}
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

                <FormRow label={t("settings.ai.model")}>
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
                  <FormRow label={t("settings.ai.endpoint")}>
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
                  <FormRow label={t("settings.ai.api_key")}>
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

                <FormRow label={t("settings.ai.max_tokens")}>
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
                    helperText={t("settings.ai.max_tokens_help")}
                  />
                </FormRow>

                <Divider />

                <FormRow label={t("settings.ai.echo_commands")}>
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
                    label={t("settings.ai.echo_commands_help")}
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
                    {t("settings.ai.reset")}
                  </Button>
                  <Button
                    variant="contained"
                    onClick={handleSave}
                    loading={saving}
                    disabled={!config}
                  >
                    {t("common.save")}
                  </Button>
                </Box>
              </Stack>
            </>
          )}

          {tab === AISettingsTab.Tools && (
            <Stack spacing={2}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {t("settings.ai.filesystem_title")}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {t("settings.ai.filesystem_help")}
              </Typography>
              {allowedDirs.length === 0 && (
                <Alert severity="info">{t("settings.ai.filesystem_empty")}</Alert>
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
                  label={t("settings.ai.filesystem_dir")}
                  value={newAllowDir}
                  onChange={(e) => setNewAllowDir(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addAllowedDir();
                    }
                  }}
                  placeholder={t("settings.ai.filesystem_dir_placeholder")}
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
                  {t("settings.ai.filesystem_add")}
                </Button>
              </Box>

              <Divider sx={{ my: 1 }} />

              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {t("settings.ai.search_title")}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {t("settings.ai.search_help")}
              </Typography>

              <FormRow label={t("settings.ai.search_provider")}>
                <FormControl fullWidth size="small">
                  <InputLabel>{t("settings.ai.select_provider")}</InputLabel>
                  <Select
                    value={
                      config.web_search?.provider
                        ? String(config.web_search.provider)
                        : SEARCH_PROVIDER_NONE
                    }
                    label={t("settings.ai.select_provider")}
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
                      {t("settings.ai.search_none")}
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
                <FormRow label={t("settings.ai.api_key")}>
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
                  {t("settings.ai.reset_tools")}
                </Button>
                <Button
                  variant="contained"
                  onClick={handleSave}
                  loading={saving}
                  disabled={!config}
                >
                  {t("common.save")}
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
  const tip = available ? t("settings.ai.available") : error || t("settings.ai.unavailable");
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
        if (!cancelled) errorMessage(translateServiceError(err));
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
      errorMessage(translateServiceError(err));
    } finally {
      setLoading(false);
    }
  };

  const onToggle = async (name: string, enabled: boolean) => {
    setBusyName(name);
    try {
      const list = await AIService.SetMCPServerEnabled(name, enabled);
      applyList(list);
      successMessage(enabled ? t("settings.ai.mcp_on") : t("settings.ai.mcp_off"));
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setBusyName("");
    }
  };

  const onOpen = async () => {
    try {
      await AIService.OpenMCPConfig();
    } catch (err) {
      errorMessage(translateServiceError(err));
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
          {t("settings.ai.open_mcp")}
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
          {t("common.refresh")}
        </Button>
      </Box>
      {!ready ? null : servers.length === 0 ? (
        <Typography color="text.secondary">{t("settings.ai.mcp_empty")}</Typography>
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
      errorMessage(translateServiceError(err));
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
      successMessage(enabled ? t("settings.ai.skill_on") : t("settings.ai.skill_off"));
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setBusyName("");
    }
  };

  const onOpenDir = async () => {
    try {
      await AIService.OpenSkillsDir();
    } catch (err) {
      errorMessage(translateServiceError(err));
    }
  };

  const onOpenSkill = async (name: string) => {
    try {
      await AIService.OpenSkillDir(name);
    } catch (err) {
      errorMessage(translateServiceError(err));
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
          {t("settings.ai.open_skills")}
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
          {t("common.refresh")}
        </Button>
      </Box>
      {loading && skills.length === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", p: 3 }}>
          <CircularProgress size={28} />
        </Box>
      ) : skills.length === 0 ? (
        <Typography color="text.secondary">{t("settings.ai.skills_empty")}</Typography>
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
