import React, { useState } from "react";
import {
  Button,
  FormControl,
  TextField,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Select,
  MenuItem,
  InputLabel,
  Box,
  Autocomplete,
  createFilterOptions,
} from "@mui/material";
import {
  LogService,
  SSHService,
} from "../../bindings/github.com/ilaziness/vela/services";
import * as BookmarkService from "../../bindings/github.com/ilaziness/vela/services/bookmarkservice";
import { SSHLinkInfo, toConnectRequest } from "../types/ssh";
import { useMessageStore } from "../stores/message";
import { translateServiceError } from "../func/service";
import {
  SSHBookmark,
  BookmarkGroup,
  BookmarkListItem,
} from "../../bindings/github.com/ilaziness/vela/services/models";
import { ProxyMode, ProxyType } from "../types/proxy";
import {
  DEFAULT_BOOKMARK_GROUP_ID,
  displayBookmarkGroupName,
  t,
} from "../i18n";

// 输入连接信息表单
interface ConnectionFormProps {
  onConnect: (info: SSHLinkInfo) => void;
  error?: string;
  connecting?: boolean;
}

const ConnectionForm: React.FC<ConnectionFormProps> = ({
  onConnect,
  error,
  connecting,
}) => {
  const [host, setHost] = useState("");
  const [port, setPort] = useState<string>("22");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [key, setKey] = useState("");
  const [keyPassword, setKeyPassword] = useState("");
  const [proxyJumpID, setProxyJumpID] = useState("");
  const [certificate, setCertificate] = useState("");
  const [allBookmarks, setAllBookmarks] = useState<BookmarkListItem[]>([]);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState(DEFAULT_BOOKMARK_GROUP_ID);
  const [groups, setGroups] = useState<string[]>([]);
  const [testing, setTesting] = useState(false);

  const { errorMessage, successMessage } = useMessageStore();

  const filterOptions = createFilterOptions<BookmarkListItem>({
    limit: 20,
  });

  React.useEffect(() => {
    BookmarkService.GetAllBookmarks()
      .then((res) => {
        setAllBookmarks(res.filter((b): b is BookmarkListItem => b !== null));
      })
      .catch((err) => {
        console.error("GetAllBookmarks failed:", err);
      });
  }, []);

  const onSelectKeyFile = async () => {
    try {
      const file = await SSHService.SelectKeyFile();
      LogService.Debug(`select key file ${file}`);
      setKey(file);
    } catch (err: unknown) {
      errorMessage(translateServiceError(err));
    }
  };

  const onSelectCertificate = async () => {
    try {
      const file = await SSHService.SelectCertificateFile();
      LogService.Debug(`select certificate file ${file}`);
      if (file) {
        setCertificate(file);
      }
    } catch (err: unknown) {
      errorMessage(translateServiceError(err));
    }
  };

  const validate = (): number | null => {
    if (!host.trim() || !port.toString().trim() || !user.trim()) {
      errorMessage(t("bookmark.connect_fields_required"));
      return null;
    }
    const p = Number(port);
    if (Number.isNaN(p) || p <= 0) {
      errorMessage(t("bookmark.port_invalid"));
      return null;
    }
    return p;
  };

  const submit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const p = validate();
    if (p == null) {
      return;
    }

    onConnect({
      host,
      port: p,
      user,
      password,
      key,
      keyPassword: key ? keyPassword : undefined,
      proxyJumpID,
      certificate,
    });
  };

  const handleTestConnection = async () => {
    const p = validate();
    if (p == null) {
      return;
    }
    setTesting(true);
    try {
      await SSHService.TestConnectInfo(
        toConnectRequest({
          host,
          port: p,
          user,
          password,
          key,
          keyPassword,
          proxyJumpID,
          certificate,
        }),
      );
      successMessage(t("bookmark.test_success"));
    } catch (err) {
      LogService.Warn(`Connection test failed: ${err}`);
      errorMessage(t("bookmark.test_failed", translateServiceError(err)));
    } finally {
      setTesting(false);
    }
  };

  const handleSaveClick = () => {
    BookmarkService.ListBookmarks()
      .then((bookmarkGroups) => {
        const groupNames = bookmarkGroups
          .filter((group): group is BookmarkGroup => group !== null)
          .map((group) => group.name);
        setGroups(groupNames);
        setSelectedGroup(groupNames[0] || DEFAULT_BOOKMARK_GROUP_ID);
        setSaveDialogOpen(true);
      })
      .catch((err) => {
        errorMessage(translateServiceError(err));
      });
  };

  const handleSaveBookmark = () => {
    const newBookmark: SSHBookmark = {
      id: "",
      title: `${host}:${port}`,
      group_name: selectedGroup,
      host,
      port: Number(port),
      private_key: key,
      private_key_password: keyPassword,
      proxy_jump_id: proxyJumpID,
      user,
      password,
      icon: "",
      use_agent: true,
      ssh_key_id: "",
      certificate,
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
      color: "",
    };

    BookmarkService.SaveBookmark(newBookmark)
      .then(() => {
        setSaveDialogOpen(false);
        onConnect({
          host,
          port: Number(port),
          user,
          password,
          key,
          keyPassword: key ? keyPassword : undefined,
          proxyJumpID: proxyJumpID,
          certificate,
        });
      })
      .catch((err) => {
        errorMessage(translateServiceError(err));
      });
  };

  return (
    <>
      <Box
        component="form"
        sx={{
          display: "flex",
          flexDirection: "column",
          pt: 3,
          alignItems: "center",
          flex: 1,
        }}
        onSubmit={submit}
      >
        <FormControl sx={{ width: 300 }}>
          <TextField
            required
            label={t("bookmark.field.host")}
            value={host}
            onChange={(e) => setHost(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            required
            label={t("bookmark.field.port")}
            value={port}
            onChange={(e) => setPort(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            required
            label={t("bookmark.field.username")}
            value={user}
            onChange={(e) => setUser(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            label={t("bookmark.field.password")}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            variant="outlined"
            margin="normal"
            type="password"
            size="small"
            sx={{ m: 0.8 }}
          />
          <Box sx={{ display: "flex", gap: 1, m: 0.8 }}>
            <TextField
              label={t("bookmark.field.key_file")}
              value={key}
              variant="outlined"
              size="small"
              slotProps={{
                input: { readOnly: true },
              }}
              placeholder={t("bookmark.field.key_file_placeholder")}
              sx={{ flex: 1 }}
            />
            <Button type="button" onClick={onSelectKeyFile} variant="outlined">
              {t("common.select_file")}
            </Button>
          </Box>
          {key && (
            <TextField
              label={t("bookmark.field.key_passphrase")}
              value={keyPassword}
              onChange={(e) => setKeyPassword(e.target.value)}
              variant="outlined"
              margin="normal"
              type="password"
              size="small"
              sx={{ m: 0.8 }}
              placeholder={t("bookmark.field.key_passphrase_placeholder")}
            />
          )}
          <Box sx={{ display: "flex", gap: 1, m: 0.8 }}>
            <TextField
              label={t("bookmark.field.cert_file")}
              value={certificate}
              variant="outlined"
              size="small"
              slotProps={{
                input: { readOnly: true },
              }}
              placeholder={t("bookmark.field.cert_file_placeholder")}
              sx={{ flex: 1 }}
            />
            <Button type="button" onClick={onSelectCertificate} variant="outlined">
              {t("common.select_file")}
            </Button>
            {certificate ? (
              <Button type="button" onClick={() => setCertificate("")} variant="outlined">
                {t("common.clear")}
              </Button>
            ) : null}
          </Box>
          <Autocomplete
            size="small"
            sx={{ m: 0.8 }}
            options={allBookmarks}
            filterOptions={filterOptions}
            getOptionLabel={(option) => `${option.title} (${option.host})`}
            value={allBookmarks.find((b) => b.id === proxyJumpID) || null}
            onChange={(_, newValue) => {
              setProxyJumpID(newValue ? newValue.id : "");
            }}
            renderInput={(params) => (
              <TextField
                {...params}
                label={t("bookmark.field.proxy_jump")}
                placeholder={t("bookmark.field.proxy_jump_placeholder")}
              />
            )}
            isOptionEqualToValue={(option, value) => option.id === value.id}
          />
        </FormControl>
        {error && (
          <Typography color="error" sx={{ mt: 1, mb: 1 }}>
            {error}
          </Typography>
        )}
        <Box sx={{ display: "flex", gap: 1, mt: 1 }}>
          <Button
            variant="outlined"
            color="secondary"
            size="large"
            type="button"
            loading={testing}
            onClick={handleTestConnection}
          >
            {t("bookmark.test_connect")}
          </Button>
          <Button
            variant="contained"
            type="submit"
            size="large"
            loading={connecting}
          >
            {t("common.connect")}
          </Button>
          <Button
            variant="outlined"
            size="large"
            loading={connecting}
            onClick={handleSaveClick}
          >
            {t("common.save")}
          </Button>
        </Box>
      </Box>

      <Dialog open={saveDialogOpen} onClose={() => setSaveDialogOpen(false)}>
        <DialogTitle>{t("bookmark.save_to_bookmark")}</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 2, minWidth: 400 }}>
            <InputLabel id="bookmark-group-select-label">
              {t("bookmark.select_group_label")}
            </InputLabel>
            <Select
              labelId="bookmark-group-select-label"
              value={selectedGroup}
              label={t("bookmark.select_group_label")}
              onChange={(e) => setSelectedGroup(e.target.value)}
              fullWidth
            >
              {groups.map((group) => (
                <MenuItem key={group} value={group}>
                  {displayBookmarkGroupName(group)}
                </MenuItem>
              ))}
            </Select>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSaveDialogOpen(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleSaveBookmark}>{t("common.save")}</Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

export default ConnectionForm;
