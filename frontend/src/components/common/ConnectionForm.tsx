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
  SessionService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import * as BookmarkService from "../../../bindings/github.com/ilaziness/vela/services/bookmarkservice";
import { SessionLink, SessionProtocol, toConnectRequest } from "../../types/session";
import { useMessageStore } from "../../stores/message";
import { translateServiceError } from "../../func/service";
import {
  SSHBookmark,
  BookmarkGroup,
  BookmarkListItem,
} from "../../../bindings/github.com/ilaziness/vela/services/models";
import { ProxyMode, ProxyType } from "../../types/proxy";
import {
  DEFAULT_BOOKMARK_GROUP_ID,
  displayBookmarkGroupName,
  t,
} from "../../i18n";

interface ConnectionFormProps {
  onConnect: (info: SessionLink) => void;
  error?: string;
  connecting?: boolean;
}

const BAUD_RATES = [9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600];

const ConnectionForm: React.FC<ConnectionFormProps> = ({
  onConnect,
  error,
  connecting,
}) => {
  const [protocol, setProtocol] = useState<SessionProtocol>(SessionProtocol.SSH);
  const [host, setHost] = useState("");
  const [port, setPort] = useState<string>("22");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [key, setKey] = useState("");
  const [keyPassword, setKeyPassword] = useState("");
  const [proxyJumpID, setProxyJumpID] = useState("");
  const [certificate, setCertificate] = useState("");
  const [serialDevice, setSerialDevice] = useState("");
  const [serialPorts, setSerialPorts] = useState<string[]>([]);
  const [baudRate, setBaudRate] = useState(115200);
  const [dataBits, setDataBits] = useState(8);
  const [parity, setParity] = useState("none");
  const [stopBits, setStopBits] = useState(1);
  const [flowControl, setFlowControl] = useState("none");
  const [charset, setCharset] = useState("utf-8");
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
        setAllBookmarks(
          res.filter((b): b is BookmarkListItem => b !== null),
        );
      })
      .catch((err) => {
        console.error("GetAllBookmarks failed:", err);
      });
  }, []);

  React.useEffect(() => {
    if (protocol !== SessionProtocol.Serial) return;
    SessionService.ListSerialPorts()
      .then((ports) => setSerialPorts(ports || []))
      .catch((err) => {
        LogService.Warn(`ListSerialPorts failed: ${err}`);
        setSerialPorts([]);
      });
  }, [protocol]);

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

  const buildLink = (): SessionLink | null => {
    if (protocol === SessionProtocol.Serial) {
      if (!serialDevice.trim() || baudRate <= 0) {
        errorMessage(t("serial.fields_required"));
        return null;
      }
      return {
        protocol,
        serialDevice: serialDevice.trim(),
        baudRate,
        dataBits,
        parity,
        stopBits,
        flowControl,
        charset,
      };
    }
    if (!host.trim() || !port.toString().trim() || !user.trim()) {
      errorMessage(t("bookmark.connect_fields_required"));
      return null;
    }
    const p = Number(port);
    if (Number.isNaN(p) || p <= 0) {
      errorMessage(t("bookmark.port_invalid"));
      return null;
    }
    return {
      protocol,
      host,
      port: p,
      user,
      password,
      key,
      keyPassword: key ? keyPassword : undefined,
      proxyJumpID,
      certificate,
      charset,
    };
  };

  const submit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const link = buildLink();
    if (!link) return;
    onConnect(link);
  };

  const handleTestConnection = async () => {
    const link = buildLink();
    if (!link) return;
    setTesting(true);
    try {
      await SessionService.TestConnect(toConnectRequest(link));
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
    const link = buildLink();
    if (!link) return;
    const title =
      protocol === SessionProtocol.Serial
        ? link.serialDevice || "serial"
        : `${host}:${port}`;
    const newBookmark: SSHBookmark = {
      id: "",
      title,
      group_name: selectedGroup,
      protocol,
      host: link.host || "",
      port: link.port || 0,
      private_key: link.key || "",
      private_key_password: link.keyPassword || "",
      proxy_jump_id: link.proxyJumpID || "",
      user: link.user || "",
      password: link.password || "",
      icon: "",
      use_agent: true,
      ssh_key_id: "",
      certificate: link.certificate || "",
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
      charset: link.charset || "utf-8",
      serial_device: link.serialDevice || "",
      baud_rate: link.baudRate || 0,
      data_bits: link.dataBits || 0,
      parity: link.parity || "",
      stop_bits: link.stopBits || 0,
      flow_control: link.flowControl || "",
      tags: [],
      color: "",
    };

    BookmarkService.SaveBookmark(newBookmark)
      .then(() => {
        setSaveDialogOpen(false);
        onConnect(link);
      })
      .catch((err) => {
        errorMessage(translateServiceError(err));
      });
  };

  const sshJumpOptions = allBookmarks.filter(
    (b) => b.protocol === "ssh",
  );

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
          <FormControl size="small" sx={{ m: 0.8 }}>
            <InputLabel id="protocol-label">{t("session.protocol")}</InputLabel>
            <Select
              labelId="protocol-label"
              label={t("session.protocol")}
              value={protocol}
              onChange={(e) => setProtocol(e.target.value as SessionProtocol)}
            >
              <MenuItem value={SessionProtocol.SSH}>
                {t("session.protocol.ssh")}
              </MenuItem>
              <MenuItem value={SessionProtocol.Serial}>
                {t("session.protocol.serial")}
              </MenuItem>
            </Select>
          </FormControl>

          {protocol === SessionProtocol.SSH ? (
            <>
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
                <Button
                  type="button"
                  onClick={onSelectCertificate}
                  variant="outlined"
                >
                  {t("common.select_file")}
                </Button>
                {certificate ? (
                  <Button
                    type="button"
                    onClick={() => setCertificate("")}
                    variant="outlined"
                  >
                    {t("common.clear")}
                  </Button>
                ) : null}
              </Box>
              <Autocomplete
                size="small"
                sx={{ m: 0.8 }}
                options={sshJumpOptions}
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
            </>
          ) : (
            <>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="serial-port-label">
                  {t("serial.device")}
                </InputLabel>
                <Select
                  labelId="serial-port-label"
                  label={t("serial.device")}
                  value={serialDevice}
                  onChange={(e) => setSerialDevice(e.target.value)}
                >
                  {serialPorts.map((p) => (
                    <MenuItem key={p} value={p}>
                      {p}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="baud-label">{t("serial.baud_rate")}</InputLabel>
                <Select
                  labelId="baud-label"
                  label={t("serial.baud_rate")}
                  value={baudRate}
                  onChange={(e) => setBaudRate(Number(e.target.value))}
                >
                  {BAUD_RATES.map((b) => (
                    <MenuItem key={b} value={b}>
                      {b}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="databits-label">{t("serial.data_bits")}</InputLabel>
                <Select
                  labelId="databits-label"
                  label={t("serial.data_bits")}
                  value={dataBits}
                  onChange={(e) => setDataBits(Number(e.target.value))}
                >
                  {[5, 6, 7, 8].map((b) => (
                    <MenuItem key={b} value={b}>
                      {b}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="parity-label">{t("serial.parity")}</InputLabel>
                <Select
                  labelId="parity-label"
                  label={t("serial.parity")}
                  value={parity}
                  onChange={(e) => setParity(e.target.value)}
                >
                  {["none", "even", "odd", "mark", "space"].map((p) => (
                    <MenuItem key={p} value={p}>
                      {p}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="stopbits-label">{t("serial.stop_bits")}</InputLabel>
                <Select
                  labelId="stopbits-label"
                  label={t("serial.stop_bits")}
                  value={stopBits}
                  onChange={(e) => setStopBits(Number(e.target.value))}
                >
                  {[1, 2].map((b) => (
                    <MenuItem key={b} value={b}>
                      {b}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl size="small" sx={{ m: 0.8 }}>
                <InputLabel id="flow-label">{t("serial.flow_control")}</InputLabel>
                <Select
                  labelId="flow-label"
                  label={t("serial.flow_control")}
                  value={flowControl}
                  onChange={(e) => setFlowControl(e.target.value)}
                >
                  {["none", "hardware"].map((p) => (
                    <MenuItem key={p} value={p}>
                      {p}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </>
          )}

          <FormControl size="small" sx={{ m: 0.8 }}>
            <InputLabel id="charset-label">{t("bookmark.field.term_encoding")}</InputLabel>
            <Select
              labelId="charset-label"
              label={t("bookmark.field.term_encoding")}
              value={charset}
              onChange={(e) => setCharset(e.target.value)}
            >
              {["utf-8", "gbk", "gb18030"].map((c) => (
                <MenuItem key={c} value={c}>
                  {c}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
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
