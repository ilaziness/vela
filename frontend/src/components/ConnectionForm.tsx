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
} from "../../bindings/github.com/ilaziness/vexo/services";
import * as BookmarkService from "../../bindings/github.com/ilaziness/vexo/services/bookmarkservice";
import { SSHLinkInfo, toConnectRequest } from "../types/ssh";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";
import {
  SSHBookmark,
  BookmarkGroup,
  BookmarkListItem,
} from "../../bindings/github.com/ilaziness/vexo/services/models";
import { ProxyMode, ProxyType } from "../types/proxy";

// 输入连接信息表单
interface ConnectionFormProps {
  onConnect: (info: SSHLinkInfo) => void;
  error?: string;
  connecting?: boolean;
}

const DEFAULT_GROUP = "默认书签";

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
  const [selectedGroup, setSelectedGroup] = useState(DEFAULT_GROUP);
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
        console.error("获取书签列表失败:", err);
      });
  }, []);

  const onSelectKeyFile = async () => {
    try {
      const file = await SSHService.SelectKeyFile();
      LogService.Debug(`select key file ${file}`);
      setKey(file);
    } catch (err: any) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const onSelectCertificate = async () => {
    try {
      const file = await SSHService.SelectCertificateFile();
      LogService.Debug(`select certificate file ${file}`);
      if (file) {
        setCertificate(file);
      }
    } catch (err: any) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const validate = (): number | null => {
    if (!host.trim() || !port.toString().trim() || !user.trim()) {
      errorMessage("Host、Port 和 Username 为必填项");
      return null;
    }
    const p = Number(port);
    if (Number.isNaN(p) || p <= 0) {
      errorMessage("Port 必须是有效的数字");
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
      successMessage("连接测试成功");
    } catch (err) {
      LogService.Warn(`Connection test failed: ${err}`);
      errorMessage("连接测试失败: " + parseCallServiceError(err));
    } finally {
      setTesting(false);
    }
  };

  const handleSaveClick = () => {
    // 加载书签分组
    BookmarkService.ListBookmarks()
      .then((bookmarkGroups) => {
        const groupNames = bookmarkGroups
          .filter((group): group is BookmarkGroup => group !== null)
          .map((group) => group.name);
        setGroups(groupNames);
        setSelectedGroup(groupNames[0] || DEFAULT_GROUP);
        setSaveDialogOpen(true);
      })
      .catch((err) => {
        errorMessage(parseCallServiceError(err));
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
        errorMessage(parseCallServiceError(err));
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
            label="Host"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            required
            label="Port"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            required
            label="Username"
            value={user}
            onChange={(e) => setUser(e.target.value)}
            variant="outlined"
            margin="normal"
            size="small"
            sx={{ m: 0.8 }}
          />
          <TextField
            label="Password"
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
              label="Key file"
              value={key}
              variant="outlined"
              size="small"
              slotProps={{
                input: { readOnly: true },
              }}
              placeholder="please select key file"
              sx={{ flex: 1 }}
            />
            <Button type="button" onClick={onSelectKeyFile} variant="outlined">
              Select
            </Button>
          </Box>
          {key && (
            <TextField
              label="Key Password"
              value={keyPassword}
              onChange={(e) => setKeyPassword(e.target.value)}
              variant="outlined"
              margin="normal"
              type="password"
              size="small"
              sx={{ m: 0.8 }}
              placeholder="private key password (optional)"
            />
          )}
          <Box sx={{ display: "flex", gap: 1, m: 0.8 }}>
            <TextField
              label="Certificate"
              value={certificate}
              variant="outlined"
              size="small"
              slotProps={{
                input: { readOnly: true },
              }}
              placeholder="OpenSSH user certificate"
              sx={{ flex: 1 }}
            />
            <Button type="button" onClick={onSelectCertificate} variant="outlined">
              Select
            </Button>
            {certificate ? (
              <Button type="button" onClick={() => setCertificate("")} variant="outlined">
                Clear
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
                label="ProxyJump"
                placeholder="Select proxy jump bookmark"
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
            测试连接
          </Button>
          <Button
            variant="contained"
            type="submit"
            size="large"
            loading={connecting}
          >
            连接
          </Button>
          <Button
            variant="outlined"
            size="large"
            loading={connecting}
            onClick={handleSaveClick}
          >
            保存
          </Button>
        </Box>
      </Box>

      {/* 保存书签对话框 */}
      <Dialog open={saveDialogOpen} onClose={() => setSaveDialogOpen(false)}>
        <DialogTitle>保存到书签</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 2, minWidth: 400 }}>
            <InputLabel id="bookmark-group-select-label">选择分组</InputLabel>
            <Select
              labelId="bookmark-group-select-label"
              value={selectedGroup}
              label="选择分组"
              onChange={(e) => setSelectedGroup(e.target.value)}
              fullWidth
            >
              {groups.map((group) => (
                <MenuItem key={group} value={group}>
                  {group}
                </MenuItem>
              ))}
            </Select>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSaveDialogOpen(false)}>取消</Button>
          <Button onClick={handleSaveBookmark}>保存</Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

export default ConnectionForm;
