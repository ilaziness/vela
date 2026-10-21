import React, { useState, useEffect, useRef, useCallback } from "react";
import {
  Box,
  TextField,
  Button,
  Typography,
  Paper,
  Stack,
  MenuItem,
  Select,
  FormControl,
  FormControlLabel,
  Checkbox,
  IconButton,
  InputAdornment,
  Autocomplete,
  createFilterOptions,
  Tooltip,
  Tabs,
  Tab,
  Divider,
  type SelectChangeEvent,
} from "@mui/material";
import ClearIcon from "@mui/icons-material/Clear";
import MoreHorizIcon from "@mui/icons-material/MoreHoriz";
import {
  SSHBookmark,
  SSHKeyInfo,
  AppService,
  KeyService,
  SSHService,
} from "../../bindings/github.com/ilaziness/vela/services";
import * as BookmarkService from "../../bindings/github.com/ilaziness/vela/services/bookmarkservice";
import { BookmarkListItem } from "../../bindings/github.com/ilaziness/vela/services/models";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";
import FormRow from "./FormRow";
import {
  BookmarkIconView,
  OsIconPicker,
} from "./icons/bookmarkIcons";
import { ProxyMode, ProxyType } from "../types/proxy";
import { envFromJSON, envToJSON, formatEnvLines, parseEnvLines } from "../func/envVars";
import {
  bookmarkColorHex,
  normalizeBookmarkColor,
  normalizeBookmarkTags,
} from "../func/bookmarkMeta";
import { BOOKMARK_COLOR_OPTIONS, BookmarkColor } from "../types/bookmark";

interface BookmarkFormProps {
  bookmark: SSHBookmark | null;
  groupNames: string[];
  tagSuggestions: string[];
  onSave: (bookmark: SSHBookmark) => Promise<SSHBookmark>;
  onTestConnection: (bookmark: SSHBookmark) => Promise<void>;
  onSaveAndConnect: (bookmark: SSHBookmark) => Promise<SSHBookmark>;
}

enum BookmarkFormTab {
  Basic = "basic",
  Connection = "connection",
  Auth = "auth",
  Session = "session",
}

const filterJumpOptions = createFilterOptions<BookmarkListItem>({
  limit: 20,
});

const TERM_OPTIONS = [
  "xterm-256color",
  "xterm",
  "vt100",
  "linux",
  "screen",
  "screen-256color",
  "tmux",
  "tmux-256color",
] as const;

const CHARSET_OPTIONS = [
  { value: "utf-8", label: "UTF-8" },
  { value: "gbk", label: "GBK" },
  { value: "gb18030", label: "GB18030" },
] as const;

const emptyBookmark = (): SSHBookmark => ({
  id: "",
  title: "",
  group_name: "默认书签",
  host: "",
  port: 22,
  private_key: "",
  private_key_password: "",
  proxy_jump_id: "",
  user: "",
  password: "",
  icon: "",
  use_agent: true,
  ssh_key_id: "",
  certificate: "",
  forward_agent: false,
  proxy_mode: ProxyMode.Inherit,
  proxy_type: ProxyType.None,
  proxy_host: "",
  proxy_port: 0,
  proxy_user: "",
  proxy_password: "",
  startup_cmd: "",
  env_vars: "",
  term: "",
  charset: "utf-8",
  tags: [],
  color: BookmarkColor.None,
});

const BookmarkForm: React.FC<BookmarkFormProps> = ({
  bookmark,
  groupNames,
  tagSuggestions,
  onSave,
  onTestConnection,
  onSaveAndConnect,
}) => {
  const { errorMessage } = useMessageStore();

  const [formData, setFormData] = useState<SSHBookmark>(emptyBookmark());
  const [envText, setEnvText] = useState("");

  const [isLoading, setIsLoading] = useState(false);
  const [allBookmarks, setAllBookmarks] = useState<BookmarkListItem[]>([]);
  const [storedKeys, setStoredKeys] = useState<SSHKeyInfo[]>([]);
  const [iconAnchor, setIconAnchor] = useState<HTMLElement | null>(null);
  const [tab, setTab] = useState(BookmarkFormTab.Basic);
  const prevBookmarkIdRef = useRef<string | undefined>(undefined);
  const keepTabAfterDraftSaveRef = useRef(false);

  const loadJumpTargets = useCallback(() => {
    BookmarkService.GetAllBookmarks()
      .then((res) => {
        setAllBookmarks(res.filter((b): b is BookmarkListItem => b !== null));
      })
      .catch((err) => {
        errorMessage(parseCallServiceError(err));
      });
  }, [errorMessage]);

  // 切换书签时回到基本信息；草稿首次保存拿到 id 时保持当前 Tab
  useEffect(() => {
    if (!bookmark) {
      prevBookmarkIdRef.current = undefined;
      return;
    }
    const id = bookmark.id;
    if (keepTabAfterDraftSaveRef.current) {
      keepTabAfterDraftSaveRef.current = false;
      prevBookmarkIdRef.current = id;
      return;
    }
    if (prevBookmarkIdRef.current !== id) {
      setTab(BookmarkFormTab.Basic);
      setIconAnchor(null);
    }
    prevBookmarkIdRef.current = id;
  }, [bookmark]);

  useEffect(() => {
    loadJumpTargets();
  }, [loadJumpTargets]);

  useEffect(() => {
    KeyService.List()
      .then((res) => {
        setStoredKeys((res ?? []).filter((item): item is SSHKeyInfo => item != null));
      })
      .catch((err) => {
        errorMessage(parseCallServiceError(err));
      });
  }, [bookmark, errorMessage]);

  useEffect(() => {
    if (bookmark) {
      const next = {
        ...emptyBookmark(),
        ...bookmark,
        icon: bookmark.icon || "",
        proxy_mode: bookmark.proxy_mode || ProxyMode.Inherit,
        startup_cmd: bookmark.startup_cmd || "",
        env_vars: bookmark.env_vars || "",
        term: bookmark.term || "",
        charset: bookmark.charset || "utf-8",
        tags: normalizeBookmarkTags(bookmark.tags),
        color: normalizeBookmarkColor(bookmark.color),
      };
      setFormData(next);
      setEnvText(formatEnvLines(envFromJSON(next.env_vars)));
    } else {
      setFormData(emptyBookmark());
      setEnvText("");
    }
  }, [bookmark]);

  const withSessionOptions = (data: SSHBookmark): SSHBookmark | null => {
    try {
      const env = parseEnvLines(envText);
      return {
        ...data,
        startup_cmd: (data.startup_cmd || "").trim(),
        term: (data.term || "").trim(),
        charset: data.charset || "utf-8",
        env_vars: envToJSON(env),
        tags: normalizeBookmarkTags(data.tags),
        color: normalizeBookmarkColor(data.color),
      };
    } catch (err) {
      errorMessage(err instanceof Error ? err.message : String(err));
      setTab(BookmarkFormTab.Session);
      return null;
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]:
        name === "port" || name === "proxy_port" ? Number(value) : value,
    }));
  };

  const validateForm = (): boolean => {
    if (!formData.title.trim()) {
      errorMessage("书签名称不能为空");
      setTab(BookmarkFormTab.Basic);
      return false;
    }
    if (!formData.host.trim()) {
      errorMessage("主机地址不能为空");
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (!formData.port || formData.port <= 0) {
      errorMessage("端口必须是有效的正整数");
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (!formData.user.trim()) {
      errorMessage("用户名不能为空");
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (formData.proxy_mode === ProxyMode.Custom) {
      if (!formData.proxy_type) {
        errorMessage("请选择代理类型");
        setTab(BookmarkFormTab.Connection);
        return false;
      }
      if (!formData.proxy_host?.trim()) {
        errorMessage("代理主机不能为空");
        setTab(BookmarkFormTab.Connection);
        return false;
      }
      const proxyPort = formData.proxy_port ?? 0;
      if (proxyPort < 1 || proxyPort > 65535) {
        errorMessage("代理端口必须在 1–65535 之间");
        setTab(BookmarkFormTab.Connection);
        return false;
      }
    }
    return true;
  };

  const handleSave = async () => {
    if (!validateForm()) {
      return;
    }
    const payload = withSessionOptions(formData);
    if (!payload) {
      return;
    }
    if (!formData.id) {
      keepTabAfterDraftSaveRef.current = true;
    }
    setIsLoading(true);
    try {
      const savedBookmark = await onSave(payload);
      setFormData(savedBookmark);
      setEnvText(formatEnvLines(envFromJSON(savedBookmark.env_vars)));
      loadJumpTargets();
    } catch {
      keepTabAfterDraftSaveRef.current = false;
    } finally {
      setIsLoading(false);
    }
  };

  const handleTestConnection = async () => {
    if (!validateForm()) {
      return;
    }
    const payload = withSessionOptions(formData);
    if (!payload) {
      return;
    }
    setIsLoading(true);
    try {
      await onTestConnection(payload);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSaveAndConnect = async () => {
    if (!validateForm()) {
      return;
    }
    const payload = withSessionOptions(formData);
    if (!payload) {
      return;
    }
    if (!formData.id) {
      keepTabAfterDraftSaveRef.current = true;
    }
    setIsLoading(true);
    try {
      const savedBookmark = await onSaveAndConnect(payload);
      setFormData(savedBookmark);
      setEnvText(formatEnvLines(envFromJSON(savedBookmark.env_vars)));
    } catch {
      keepTabAfterDraftSaveRef.current = false;
    } finally {
      setIsLoading(false);
    }
  };

  const handleGroupChange = (event: SelectChangeEvent) => {
    setFormData((prev) => ({
      ...prev,
      group_name: event.target.value,
    }));
  };

  const handleSelectFile = async () => {
    try {
      const selectedPath = await AppService.SelectFile();
      if (selectedPath) {
        setFormData((prev) => ({
          ...prev,
          private_key: selectedPath,
          ssh_key_id: "",
        }));
      }
    } catch (error) {
      errorMessage(parseCallServiceError(error));
    }
  };

  const handleSelectCertificate = async () => {
    try {
      const selectedPath = await SSHService.SelectCertificateFile();
      if (selectedPath) {
        setFormData((prev) => ({
          ...prev,
          certificate: selectedPath,
        }));
      }
    } catch (error) {
      errorMessage(parseCallServiceError(error));
    }
  };

  if (!bookmark) {
    return (
      <Box
        sx={{
          height: "100%",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          p: 3,
        }}
      >
        <Typography variant="h6" color="text.secondary" sx={{ mb: 1 }}>
          请选择一个书签进行编辑
        </Typography>
        <Typography variant="body2" color="text.disabled">
          从左侧列表中选择或创建一个新书签
        </Typography>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        minHeight: 0,
      }}
    >
      <Paper
        elevation={1}
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          minHeight: 0,
          m: 2,
          overflow: "hidden",
        }}
      >
        <Tabs
          value={tab}
          onChange={(_, v: BookmarkFormTab) => setTab(v)}
          sx={{ px: 1, flexShrink: 0, borderBottom: 1, borderColor: "divider" }}
        >
          <Tab label="基本信息" value={BookmarkFormTab.Basic} />
          <Tab label="连接信息" value={BookmarkFormTab.Connection} />
          <Tab label="认证信息" value={BookmarkFormTab.Auth} />
          <Tab label="会话选项" value={BookmarkFormTab.Session} />
        </Tabs>

        <Box sx={{ flex: 1, overflowY: "auto", minHeight: 0, px: 3, pt: 2, pb: 3 }}>
          {tab === BookmarkFormTab.Basic && (
            <Stack spacing={2}>
              <FormRow label="图标" labelWidth={120}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <IconButton
                    size="small"
                    onClick={(e) => setIconAnchor(e.currentTarget)}
                    sx={{
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 1,
                    }}
                  >
                    <BookmarkIconView icon={formData.icon} fontSize={20} />
                  </IconButton>
                  <Typography variant="body2" color="text.secondary">
                    {formData.icon || "默认"}
                  </Typography>
                  <OsIconPicker
                    value={formData.icon || ""}
                    onChange={(icon) =>
                      setFormData((prev) => ({ ...prev, icon }))
                    }
                    anchorEl={iconAnchor}
                    open={Boolean(iconAnchor)}
                    onClose={() => setIconAnchor(null)}
                  />
                </Box>
              </FormRow>
              <FormRow label="书签名称" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="title"
                  value={formData.title}
                  onChange={handleChange}
                  placeholder="请输入书签名称"
                />
              </FormRow>
              <FormRow label="分组" labelWidth={120}>
                <FormControl fullWidth size="small">
                  <Select
                    value={formData.group_name}
                    onChange={handleGroupChange}
                    displayEmpty
                  >
                    {groupNames.length === 0 && (
                      <MenuItem value="默认书签">默认书签</MenuItem>
                    )}
                    {groupNames.map((groupName) => (
                      <MenuItem key={groupName} value={groupName}>
                        {groupName}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </FormRow>
              <FormRow label="标签" labelWidth={120}>
                <Autocomplete
                  multiple
                  freeSolo
                  size="small"
                  fullWidth
                  options={tagSuggestions}
                  value={formData.tags ?? []}
                  filterSelectedOptions
                  onChange={(_, value) => {
                    setFormData((prev) => ({
                      ...prev,
                      tags: normalizeBookmarkTags(value.map((v) => String(v))),
                    }));
                  }}
                  renderInput={(params) => (
                    <TextField {...params} placeholder="输入后回车添加" />
                  )}
                />
              </FormRow>
              <FormRow label="颜色" labelWidth={120}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  <Tooltip title="无">
                    <Box
                      component="button"
                      type="button"
                      onClick={() =>
                        setFormData((prev) => ({
                          ...prev,
                          color: BookmarkColor.None,
                        }))
                      }
                      sx={{
                        width: 22,
                        height: 22,
                        borderRadius: "50%",
                        border: 2,
                        borderColor: !formData.color ? "primary.main" : "divider",
                        bgcolor: "transparent",
                        cursor: "pointer",
                        p: 0,
                      }}
                    />
                  </Tooltip>
                  {BOOKMARK_COLOR_OPTIONS.map((token) => {
                    const hex = bookmarkColorHex(token);
                    const selected = formData.color === token;
                    return (
                      <Tooltip key={token} title={token}>
                        <Box
                          component="button"
                          type="button"
                          onClick={() =>
                            setFormData((prev) => ({
                              ...prev,
                              color: token,
                            }))
                          }
                          sx={{
                            width: 22,
                            height: 22,
                            borderRadius: "50%",
                            border: 2,
                            borderColor: selected ? "primary.main" : "transparent",
                            bgcolor: hex || "transparent",
                            cursor: "pointer",
                            p: 0,
                          }}
                        />
                      </Tooltip>
                    );
                  })}
                </Box>
              </FormRow>
            </Stack>
          )}

          {tab === BookmarkFormTab.Connection && (
            <Stack spacing={2}>
              <FormRow label="主机地址" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="host"
                  value={formData.host}
                  onChange={handleChange}
                  placeholder="例如: 192.168.1.100 或 example.com"
                />
              </FormRow>
              <FormRow label="端口" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="port"
                  type="number"
                  value={formData.port}
                  onChange={handleChange}
                  placeholder="默认: 22"
                />
              </FormRow>
              <FormRow label="用户名" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="user"
                  value={formData.user}
                  onChange={handleChange}
                  placeholder="SSH 登录用户名"
                />
              </FormRow>
              <FormRow label="跳板机(ProxyJump)" labelWidth={120}>
                <Autocomplete
                  size="small"
                  fullWidth
                  options={allBookmarks.filter((b) => b.id !== formData.id)}
                  filterOptions={filterJumpOptions}
                  getOptionLabel={(option) =>
                    `${option.title} (${option.host})`
                  }
                  value={
                    allBookmarks.find((b) => b.id === formData.proxy_jump_id) ||
                    null
                  }
                  onChange={(_, newValue) => {
                    setFormData((prev) => ({
                      ...prev,
                      proxy_jump_id: newValue ? newValue.id : "",
                    }));
                  }}
                  renderInput={(params) => (
                    <TextField {...params} placeholder="选择跳板机（可选）" />
                  )}
                  isOptionEqualToValue={(option, value) => option.id === value.id}
                />
              </FormRow>

              <Divider sx={{ my: 1 }} />
              <Typography
                variant="subtitle2"
                sx={{ fontWeight: 600, color: "text.secondary" }}
              >
                拨号代理
              </Typography>
              <FormRow label="代理模式" labelWidth={120}>
                <FormControl fullWidth size="small">
                  <Select
                    value={formData.proxy_mode || ProxyMode.Inherit}
                    onChange={(e) => {
                      const mode = e.target.value;
                      setFormData((prev) => ({
                        ...prev,
                        proxy_mode: mode,
                        ...(mode === ProxyMode.Custom && !prev.proxy_type
                          ? { proxy_type: ProxyType.Http }
                          : {}),
                      }));
                    }}
                  >
                    <MenuItem value={ProxyMode.Inherit}>跟随全局设置</MenuItem>
                    <MenuItem value={ProxyMode.None}>直连（不使用代理）</MenuItem>
                    <MenuItem value={ProxyMode.Custom}>自定义代理</MenuItem>
                  </Select>
                </FormControl>
              </FormRow>
              {formData.proxy_mode === ProxyMode.Custom && (
                <>
                  <FormRow label="代理类型" labelWidth={120}>
                    <TextField
                      select
                      fullWidth
                      size="small"
                      value={formData.proxy_type || ProxyType.Http}
                      onChange={(e) =>
                        setFormData((prev) => ({
                          ...prev,
                          proxy_type: e.target.value,
                        }))
                      }
                    >
                      <MenuItem value={ProxyType.Http}>HTTP</MenuItem>
                      <MenuItem value={ProxyType.Socks5}>SOCKS5</MenuItem>
                    </TextField>
                  </FormRow>
                  <FormRow label="代理主机" labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_host"
                      value={formData.proxy_host || ""}
                      onChange={handleChange}
                      placeholder="例如: 127.0.0.1"
                    />
                  </FormRow>
                  <FormRow label="代理端口" labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_port"
                      type="number"
                      value={formData.proxy_port || ""}
                      onChange={handleChange}
                      placeholder="例如: 1080"
                    />
                  </FormRow>
                  <FormRow label="用户名" labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_user"
                      value={formData.proxy_user || ""}
                      onChange={handleChange}
                      placeholder="可选"
                    />
                  </FormRow>
                  <FormRow label="密码" labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_password"
                      type="password"
                      value={formData.proxy_password || ""}
                      onChange={handleChange}
                      placeholder="可选"
                    />
                  </FormRow>
                  <Typography variant="caption" color="text.secondary">
                    有跳板机时，代理只作用于连到第一跳；与系统代理无关
                  </Typography>
                </>
              )}
            </Stack>
          )}

          {tab === BookmarkFormTab.Auth && (
            <Stack spacing={2}>
              <FormRow label="Agent 转发" labelWidth={120}>
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={formData.forward_agent}
                      onChange={(_, checked) =>
                        setFormData((prev) => ({
                          ...prev,
                          forward_agent: checked,
                        }))
                      }
                    />
                  }
                  label="转发 SSH Agent（需本机 Agent 在运行）"
                />
              </FormRow>
              <FormRow label="密码" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="password"
                  type="password"
                  value={formData.password}
                  onChange={handleChange}
                  placeholder="密码认证（可选）"
                />
              </FormRow>
              <FormRow label="应用内密钥" labelWidth={120}>
                <TextField
                  select
                  fullWidth
                  size="small"
                  value={formData.ssh_key_id || ""}
                  onChange={(e) => {
                    const id = e.target.value;
                    setFormData((prev) => ({
                      ...prev,
                      ssh_key_id: id,
                      ...(id ? { private_key: "", private_key_password: "" } : {}),
                    }));
                  }}
                >
                  <MenuItem value="">不使用</MenuItem>
                  {formData.ssh_key_id &&
                    !storedKeys.some((key) => key.id === formData.ssh_key_id) && (
                      <MenuItem value={formData.ssh_key_id}>已保存的密钥</MenuItem>
                    )}
                  {storedKeys.map((key) => (
                    <MenuItem key={key.id} value={key.id}>
                      {key.name}（{key.algorithm}）
                    </MenuItem>
                  ))}
                </TextField>
              </FormRow>
              <FormRow label="密钥文件" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="private_key"
                  value={formData.private_key}
                  placeholder="私钥文件路径（可选）"
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            edge="end"
                            onClick={handleSelectFile}
                            size="small"
                            title="选择文件"
                          >
                            <MoreHorizIcon />
                          </IconButton>
                        </InputAdornment>
                      ),
                    },
                  }}
                />
              </FormRow>
              <FormRow label="密钥密码" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="private_key_password"
                  type="password"
                  value={formData.private_key_password}
                  onChange={handleChange}
                  disabled={Boolean(formData.ssh_key_id)}
                  placeholder={
                    formData.ssh_key_id
                      ? "应用内密钥不使用此口令"
                      : "私钥密码（可选）"
                  }
                />
              </FormRow>
              <FormRow label="证书文件" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="certificate"
                  value={formData.certificate || ""}
                  placeholder="OpenSSH 用户证书（可选）"
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          {formData.certificate ? (
                            <IconButton
                              edge="end"
                              onClick={() =>
                                setFormData((prev) => ({
                                  ...prev,
                                  certificate: "",
                                }))
                              }
                              size="small"
                              title="清除证书"
                            >
                              <ClearIcon />
                            </IconButton>
                          ) : null}
                          <IconButton
                            edge="end"
                            onClick={handleSelectCertificate}
                            size="small"
                            title="选择证书文件"
                          >
                            <MoreHorizIcon />
                          </IconButton>
                        </InputAdornment>
                      ),
                    },
                  }}
                />
              </FormRow>
            </Stack>
          )}

          {tab === BookmarkFormTab.Session && (
            <Stack spacing={2}>
              <FormRow label="TERM" labelWidth={120}>
                <TextField
                  select
                  fullWidth
                  size="small"
                  name="term"
                  value={formData.term || ""}
                  onChange={handleChange}
                  slotProps={{
                    select: {
                      displayEmpty: true,
                      renderValue: (selected) => {
                        const v = String(selected ?? "");
                        return v || "默认（xterm-256color）";
                      },
                    },
                  }}
                >
                  <MenuItem value="">默认（xterm-256color）</MenuItem>
                  {TERM_OPTIONS.map((t) => (
                    <MenuItem key={t} value={t}>
                      {t}
                    </MenuItem>
                  ))}
                  {formData.term &&
                    !(TERM_OPTIONS as readonly string[]).includes(formData.term) && (
                      <MenuItem value={formData.term}>{formData.term}</MenuItem>
                    )}
                </TextField>
              </FormRow>
              <FormRow label="终端编码" labelWidth={120}>
                <TextField
                  select
                  fullWidth
                  size="small"
                  name="charset"
                  value={formData.charset || "utf-8"}
                  onChange={handleChange}
                >
                  {CHARSET_OPTIONS.map((c) => (
                    <MenuItem key={c.value} value={c.value}>
                      {c.label}
                    </MenuItem>
                  ))}
                </TextField>
              </FormRow>
              <FormRow label="环境变量" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  multiline
                  minRows={2}
                  value={envText}
                  onChange={(e) => setEnvText(e.target.value)}
                  placeholder={"每行一条 KEY=value"}
                />
              </FormRow>
              <FormRow label="启动命令" labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="startup_cmd"
                  value={formData.startup_cmd || ""}
                  onChange={handleChange}
                  placeholder="登录后自动执行，例如：cd /var/www && ls（可选）"
                />
              </FormRow>
            </Stack>
          )}
        </Box>

        <Box
          sx={{
            flexShrink: 0,
            display: "flex",
            justifyContent: "flex-end",
            gap: 1,
            px: 2,
            py: 1.5,
            borderTop: 1,
            borderColor: "divider",
          }}
        >
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleTestConnection}
            loading={isLoading}
          >
            测试连接
          </Button>
          <Button variant="outlined" onClick={handleSave} loading={isLoading}>
            保存
          </Button>
          <Button
            variant="contained"
            onClick={handleSaveAndConnect}
            loading={isLoading}
          >
            保存并连接
          </Button>
        </Box>
      </Paper>
    </Box>
  );
};

export default BookmarkForm;
