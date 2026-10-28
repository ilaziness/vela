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
import {
  DEFAULT_BOOKMARK_GROUP_ID,
  displayBookmarkGroupName,
  t,
} from "../i18n";
import { translateServiceError } from "../func/service";

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
  group_name: DEFAULT_BOOKMARK_GROUP_ID,
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
        errorMessage(translateServiceError(err));
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
        errorMessage(translateServiceError(err));
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
      errorMessage(t("bookmark.name_required"));
      setTab(BookmarkFormTab.Basic);
      return false;
    }
    if (!formData.host.trim()) {
      errorMessage(t("bookmark.host_required"));
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (!formData.port || formData.port <= 0) {
      errorMessage(t("bookmark.port_invalid"));
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (!formData.user.trim()) {
      errorMessage(t("bookmark.username_required"));
      setTab(BookmarkFormTab.Connection);
      return false;
    }
    if (formData.proxy_mode === ProxyMode.Custom) {
      if (!formData.proxy_type) {
        errorMessage(t("bookmark.proxy_type_required"));
        setTab(BookmarkFormTab.Connection);
        return false;
      }
      if (!formData.proxy_host?.trim()) {
        errorMessage(t("bookmark.proxy_host_required"));
        setTab(BookmarkFormTab.Connection);
        return false;
      }
      const proxyPort = formData.proxy_port ?? 0;
      if (proxyPort < 1 || proxyPort > 65535) {
        errorMessage(t("bookmark.proxy_port_invalid"));
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
      errorMessage(translateServiceError(error));
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
      errorMessage(translateServiceError(error));
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
          {t("bookmark.select_to_edit")}
        </Typography>
        <Typography variant="body2" color="text.disabled">
          {t("bookmark.select_to_edit_hint")}
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
          <Tab label={t("bookmark.tab.basic")} value={BookmarkFormTab.Basic} />
          <Tab
            label={t("bookmark.tab.connection")}
            value={BookmarkFormTab.Connection}
          />
          <Tab label={t("bookmark.tab.auth")} value={BookmarkFormTab.Auth} />
          <Tab
            label={t("bookmark.tab.session")}
            value={BookmarkFormTab.Session}
          />
        </Tabs>

        <Box sx={{ flex: 1, overflowY: "auto", minHeight: 0, px: 3, pt: 2, pb: 3 }}>
          {tab === BookmarkFormTab.Basic && (
            <Stack spacing={2}>
              <FormRow label={t("bookmark.field.icon")} labelWidth={120}>
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
                    {formData.icon || t("common.default")}
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
              <FormRow label={t("bookmark.field.name")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="title"
                  value={formData.title}
                  onChange={handleChange}
                  placeholder={t("bookmark.field.name_placeholder")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.group")} labelWidth={120}>
                <FormControl fullWidth size="small">
                  <Select
                    value={formData.group_name}
                    onChange={handleGroupChange}
                    displayEmpty
                    renderValue={(v) => displayBookmarkGroupName(String(v))}
                  >
                    {groupNames.length === 0 && (
                      <MenuItem value={DEFAULT_BOOKMARK_GROUP_ID}>
                        {t("bookmark.group.default")}
                      </MenuItem>
                    )}
                    {groupNames.map((groupName) => (
                      <MenuItem key={groupName} value={groupName}>
                        {displayBookmarkGroupName(groupName)}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </FormRow>
              <FormRow label={t("bookmark.field.tags")} labelWidth={120}>
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
                    <TextField {...params} placeholder={t("bookmark.field.tags_placeholder")} />
                  )}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.color")} labelWidth={120}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  <Tooltip title={t("common.none")}>
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
              <FormRow label={t("bookmark.field.host")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="host"
                  value={formData.host}
                  onChange={handleChange}
                  placeholder={t("bookmark.field.host_placeholder")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.port")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="port"
                  type="number"
                  value={formData.port}
                  onChange={handleChange}
                  placeholder={t("bookmark.port_default_hint")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.username")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="user"
                  value={formData.user}
                  onChange={handleChange}
                  placeholder={t("bookmark.field.username_placeholder")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.proxy_jump")} labelWidth={120}>
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
                    <TextField {...params} placeholder={t("bookmark.field.proxy_jump_placeholder")} />
                  )}
                  isOptionEqualToValue={(option, value) => option.id === value.id}
                />
              </FormRow>

              <Divider sx={{ my: 1 }} />
              <Typography
                variant="subtitle2"
                sx={{ fontWeight: 600, color: "text.secondary" }}
              >
                {t("bookmark.dial_proxy")}
              </Typography>
              <FormRow label={t("bookmark.field.proxy_mode")} labelWidth={120}>
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
                    <MenuItem value={ProxyMode.Inherit}>{t("bookmark.proxy_mode.inherit")}</MenuItem>
                    <MenuItem value={ProxyMode.None}>{t("bookmark.proxy_mode.none")}</MenuItem>
                    <MenuItem value={ProxyMode.Custom}>{t("bookmark.proxy_mode.custom")}</MenuItem>
                  </Select>
                </FormControl>
              </FormRow>
              {formData.proxy_mode === ProxyMode.Custom && (
                <>
                  <FormRow label={t("bookmark.field.proxy_type")} labelWidth={120}>
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
                  <FormRow label={t("bookmark.field.proxy_host")} labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_host"
                      value={formData.proxy_host || ""}
                      onChange={handleChange}
                      placeholder={t("bookmark.proxy_host_placeholder")}
                    />
                  </FormRow>
                  <FormRow label={t("bookmark.field.proxy_port")} labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_port"
                      type="number"
                      value={formData.proxy_port || ""}
                      onChange={handleChange}
                      placeholder={t("bookmark.proxy_port_placeholder")}
                    />
                  </FormRow>
                  <FormRow label={t("bookmark.field.username")} labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_user"
                      value={formData.proxy_user || ""}
                      onChange={handleChange}
                      placeholder={t("common.optional")}
                    />
                  </FormRow>
                  <FormRow label={t("bookmark.field.password")} labelWidth={120}>
                    <TextField
                      fullWidth
                      size="small"
                      name="proxy_password"
                      type="password"
                      value={formData.proxy_password || ""}
                      onChange={handleChange}
                      placeholder={t("common.optional")}
                    />
                  </FormRow>
                  <Typography variant="caption" color="text.secondary">
                    {t("bookmark.proxy_jump_hint")}
                  </Typography>
                </>
              )}
            </Stack>
          )}

          {tab === BookmarkFormTab.Auth && (
            <Stack spacing={2}>
              <FormRow label={t("bookmark.field.agent_forward")} labelWidth={120}>
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
                  label={t("bookmark.field.agent_forward_label")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.password")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="password"
                  type="password"
                  value={formData.password}
                  onChange={handleChange}
                  placeholder={t("bookmark.field.password_placeholder")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.app_key")} labelWidth={120}>
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
                  <MenuItem value="">{t("bookmark.field.app_key_none")}</MenuItem>
                  {formData.ssh_key_id &&
                    !storedKeys.some((key) => key.id === formData.ssh_key_id) && (
                      <MenuItem value={formData.ssh_key_id}>{t("bookmark.field.app_key_saved")}</MenuItem>
                    )}
                  {storedKeys.map((key) => (
                    <MenuItem key={key.id} value={key.id}>
                      {key.name} ({key.algorithm})
                    </MenuItem>
                  ))}
                </TextField>
              </FormRow>
              <FormRow label={t("bookmark.field.key_file")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="private_key"
                  value={formData.private_key}
                  placeholder={t("bookmark.field.key_file_placeholder")}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            edge="end"
                            onClick={handleSelectFile}
                            size="small"
                            title={t("common.select_file")}
                          >
                            <MoreHorizIcon />
                          </IconButton>
                        </InputAdornment>
                      ),
                    },
                  }}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.key_passphrase")} labelWidth={120}>
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
                      ? t("bookmark.field.key_passphrase_app")
                      : t("bookmark.field.key_passphrase_placeholder")
                  }
                />
              </FormRow>
              <FormRow label={t("bookmark.field.cert_file")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="certificate"
                  value={formData.certificate || ""}
                  placeholder={t("bookmark.field.cert_file_placeholder")}
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
                              title={t("bookmark.clear_cert")}
                            >
                              <ClearIcon />
                            </IconButton>
                          ) : null}
                          <IconButton
                            edge="end"
                            onClick={handleSelectCertificate}
                            size="small"
                            title={t("bookmark.select_cert")}
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
                        return v || t("bookmark.term_type_default");
                      },
                    },
                  }}
                >
                  <MenuItem value="">{t("bookmark.term_type_default")}</MenuItem>
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
              <FormRow label={t("bookmark.field.term_encoding")} labelWidth={120}>
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
              <FormRow label={t("bookmark.field.env")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  multiline
                  minRows={2}
                  value={envText}
                  onChange={(e) => setEnvText(e.target.value)}
                  placeholder={t("bookmark.field.env_placeholder")}
                />
              </FormRow>
              <FormRow label={t("bookmark.field.startup_cmd")} labelWidth={120}>
                <TextField
                  fullWidth
                  size="small"
                  name="startup_cmd"
                  value={formData.startup_cmd || ""}
                  onChange={handleChange}
                  placeholder={t("bookmark.field.startup_cmd_placeholder")}
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
            {t("bookmark.test_connect")}
          </Button>
          <Button variant="outlined" onClick={handleSave} loading={isLoading}>{t("common.save")}</Button>
          <Button
            variant="contained"
            onClick={handleSaveAndConnect}
            loading={isLoading}
          >
            {t("bookmark.save_and_connect")}
          </Button>
        </Box>
      </Paper>
    </Box>
  );
};

export default BookmarkForm;
