import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  CircularProgress,
  Box,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Typography,
} from "@mui/material";
import { useMessageStore } from "../stores/message";
import { SSHTunnelService } from "../../bindings/github.com/ilaziness/vela/services";
import { translateServiceError } from "../func/service";
import { t, tf } from "../i18n";
import {
  TunnelType,
  validateIpPort,
  validateRemoteAddr,
  validatePort,
  isTunnelFormValid,
  getTunnelTypeName,
  getLocalAddrPlaceholder,
  getLocalAddrHelperText,
  getRemoteAddrHelperText,
  getRemotePortHelperText,
} from "../func/validation";

interface TunnelFormProps {
  open: boolean;
  sessionID: string;
  onClose: () => void;
  onSuccess: () => void;
}

const TunnelForm: React.FC<TunnelFormProps> = ({
  open,
  sessionID,
  onClose,
  onSuccess,
}) => {
  const { errorMessage, successMessage } = useMessageStore();

  const [tunnelType, setTunnelType] = useState<TunnelType>("local");
  const [formData, setFormData] = useState({
    localAddr: "",
    remoteAddr: "",
    remotePort: "",
  });
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState({
    localAddr: "",
    remoteAddr: "",
    remotePort: "",
  });

  // 重置表单
  useEffect(() => {
    if (open) {
      setTunnelType("local");
      setFormData({
        localAddr: "127.0.0.1:", // 本地和动态转发预填写
        remoteAddr: "",
        remotePort: "",
      });
      setErrors({ localAddr: "", remoteAddr: "", remotePort: "" });
    }
  }, [open]);

  // 隧道类型改变时重置表单
  useEffect(() => {
    if (tunnelType === "local" || tunnelType === "dynamic") {
      setFormData((prev) => ({
        ...prev,
        localAddr: "127.0.0.1:",
        remoteAddr: tunnelType === "local" ? prev.remoteAddr : "",
        remotePort: "",
      }));
    } else if (tunnelType === "remote") {
      setFormData((prev) => ({
        ...prev,
        localAddr: "",
        remoteAddr: "",
        remotePort: prev.remotePort,
      }));
    }
  }, [tunnelType]);

  // 实时验证并更新错误状态
  const validateAndUpdateField = (field: string, value: string) => {
    let error = "";

    switch (field) {
      case "localAddr":
        {
          const ipPortResult = validateIpPort(value);
          error = ipPortResult.error;
        }
        break;
      case "remoteAddr":
        if (tunnelType === "local") {
          const remoteResult = validateRemoteAddr(value);
          error = remoteResult.error;
        }
        break;
      case "remotePort":
        if (tunnelType === "remote") {
          const portResult = validatePort(value);
          error = portResult.error;
        }
        break;
    }

    setErrors((prev) => ({ ...prev, [field]: error }));
    return error === "";
  };

  // 表单验证
  const isFormValid = () => {
    return isTunnelFormValid(tunnelType, formData);
  };

  // 处理表单提交
  const handleSubmit = async () => {
    if (!isFormValid()) return;

    setLoading(true);
    try {
      if (tunnelType === "local") {
        await SSHTunnelService.StartLocal(
          sessionID,
          formData.localAddr,
          formData.remoteAddr,
        );
        successMessage(
          tf("tunnel.start_success", getTunnelTypeName(tunnelType)),
        );
      } else if (tunnelType === "remote") {
        const remotePort = Number.parseInt(formData.remotePort, 10);
        await SSHTunnelService.StartRemote(
          sessionID,
          remotePort,
          formData.localAddr,
        );
        successMessage(
          tf("tunnel.start_success", getTunnelTypeName(tunnelType)),
        );
      } else {
        await SSHTunnelService.StartDynamic(sessionID, formData.localAddr);
        successMessage(
          tf("tunnel.start_success", getTunnelTypeName(tunnelType)),
        );
      }

      onSuccess();
      onClose();
      setFormData({ localAddr: "", remoteAddr: "", remotePort: "" });
    } catch (err) {
      const errorMsg =
        err instanceof Error
          ? translateServiceError(err)
          : t("tunnel.start_tunnel_failed");
      errorMessage(
        tf(
          "tunnel.start_failed",
          getTunnelTypeName(tunnelType),
          errorMsg,
        ),
      );
    } finally {
      setLoading(false);
    }
  };

  // 渲染表单字段
  const renderFormFields = () => {
    return (
      <>
        <FormControl fullWidth required>
          <InputLabel>{t("tunnel.type_label")}</InputLabel>
          <Select
            value={tunnelType}
            onChange={(e) => setTunnelType(e.target.value as TunnelType)}
            label={t("tunnel.type_label")}
          >
            <MenuItem value="local">{t("validation.tunnel_local")}</MenuItem>
            <MenuItem value="remote">{t("validation.tunnel_remote")}</MenuItem>
            <MenuItem value="dynamic">
              {t("validation.tunnel_dynamic")}
            </MenuItem>
          </Select>
        </FormControl>

        <TextField
          label={t("tunnel.local_address")}
          value={formData.localAddr}
          onChange={(e) => {
            setFormData({ ...formData, localAddr: e.target.value });
            validateAndUpdateField("localAddr", e.target.value);
          }}
          placeholder={getLocalAddrPlaceholder(tunnelType)}
          helperText={getLocalAddrHelperText(tunnelType, errors.localAddr)}
          error={!!errors.localAddr}
          fullWidth
          required
        />

        {tunnelType === "local" && (
          <TextField
            label={t("tunnel.remote_address")}
            value={formData.remoteAddr}
            onChange={(e) => {
              setFormData({ ...formData, remoteAddr: e.target.value });
              validateAndUpdateField("remoteAddr", e.target.value);
            }}
            placeholder={t("tunnel.example_remote_addr")}
            helperText={getRemoteAddrHelperText(errors.remoteAddr)}
            error={!!errors.remoteAddr}
            fullWidth
            required
          />
        )}

        {tunnelType === "remote" && (
          <TextField
            label={t("tunnel.remote_port")}
            value={formData.remotePort}
            onChange={(e) => {
              setFormData({ ...formData, remotePort: e.target.value });
              validateAndUpdateField("remotePort", e.target.value);
            }}
            placeholder={t("tunnel.example_remote_port")}
            type="number"
            slotProps={{
              htmlInput: {
                min: 1,
                max: 65535,
              },
            }}
            helperText={getRemotePortHelperText(errors.remotePort)}
            error={!!errors.remotePort}
            fullWidth
            required
          />
        )}

        {tunnelType === "dynamic" && (
          <Box sx={{ textAlign: "center", py: 1, color: "text.secondary" }}>
            <Typography variant="body2">
              {t("tunnel.dynamic_description")}
            </Typography>
          </Box>
        )}
      </>
    );
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("tunnel.create_dialog_title")}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, mt: 1 }}>
          {renderFormFields()}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          {t("common.cancel")}
        </Button>
        <Button
          onClick={handleSubmit}
          variant="contained"
          disabled={!isFormValid() || loading}
          startIcon={loading ? <CircularProgress size={16} /> : null}
        >
          {loading ? t("tunnel.starting") : t("tunnel.start")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default TunnelForm;
