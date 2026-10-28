import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Box,
  Paper,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  CircularProgress,
  Button,
  Stack,
  Chip,
} from "@mui/material";
import {
  AppService,
  KeyBindingInfo,
  LogService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import {
  parseCallServiceError,
  translateServiceError,
} from "../../func/service";
import { t, useLanguage } from "../../i18n";
import { keyboardEventToLogical } from "../../func/shortcuts";
import { useMessageStore } from "../../stores/message";
import { useKeybindingsStore } from "../../stores/keybindings";
import { CODE_FONT_FAMILY } from "../../theme/typography";

const KeybindingsSettings: React.FC = () => {
  const lang = useLanguage();
  const bindings = useKeybindingsStore((s) => s.bindings);
  const reload = useKeybindingsStore((s) => s.reload);
  const [loading, setLoading] = useState(true);
  const [recordingAction, setRecordingAction] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const suspendedRef = useRef(false);
  const submittingRef = useRef(false);
  const startingRef = useRef(false);
  const mountedRef = useRef(true);

  const resumeIfNeeded = useCallback(async () => {
    if (!suspendedRef.current) {
      return;
    }
    try {
      await AppService.ResumeKeyBindings();
      suspendedRef.current = false;
    } catch (err) {
      void LogService.Error(
        "ResumeKeyBindings failed: " + parseCallServiceError(err),
      );
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    let cancelled = false;
    const load = async () => {
      try {
        await reload();
      } catch (err) {
        if (!cancelled) {
          useMessageStore
            .getState()
            .errorMessage(
              translateServiceError(err) ||
                t("settings.keybindings.load_failed"),
            );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
      mountedRef.current = false;
      // 卸载时强制恢复，避免 Suspend 成功后竞态导致全局快捷键永久失效。
      suspendedRef.current = false;
      void AppService.ResumeKeyBindings().catch((err) => {
        void LogService.Error(
          "ResumeKeyBindings on unmount failed: " +
            parseCallServiceError(err),
        );
      });
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const stopRecording = useCallback(async () => {
    setRecordingAction(null);
    await resumeIfNeeded();
  }, [resumeIfNeeded]);

  const startRecording = useCallback(
    async (action: string) => {
      if (
        busy ||
        recordingAction ||
        submittingRef.current ||
        startingRef.current
      ) {
        return;
      }
      startingRef.current = true;
      try {
        await AppService.SuspendKeyBindings();
        if (!mountedRef.current) {
          await AppService.ResumeKeyBindings();
          return;
        }
        suspendedRef.current = true;
        setRecordingAction(action);
      } catch (err) {
        suspendedRef.current = false;
        if (mountedRef.current) {
          useMessageStore
            .getState()
            .errorMessage(
              translateServiceError(err) ||
                t("settings.keybindings.record_failed"),
            );
        }
      } finally {
        startingRef.current = false;
      }
    },
    [busy, recordingAction],
  );

  useEffect(() => {
    if (!recordingAction) {
      return;
    }
    const onKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (submittingRef.current || e.repeat) {
        return;
      }
      if (e.key === "Escape") {
        void stopRecording();
        return;
      }
      const logical = keyboardEventToLogical(e);
      if (!logical) {
        return;
      }
      const action = recordingAction;
      submittingRef.current = true;
      void (async () => {
        setBusy(true);
        try {
          await AppService.SetKeyBinding(action, logical);
          if (!mountedRef.current) {
            return;
          }
          await reload();
          useMessageStore.getState().successMessage(t("settings.keybindings.updated"));
        } catch (err) {
          if (mountedRef.current) {
            useMessageStore
              .getState()
              .errorMessage(
                translateServiceError(err) ||
                  t("settings.keybindings.set_failed"),
              );
          }
        } finally {
          submittingRef.current = false;
          if (mountedRef.current) {
            setBusy(false);
            await stopRecording();
          } else {
            suspendedRef.current = false;
            void AppService.ResumeKeyBindings().catch(() => {});
          }
        }
      })();
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
    };
  }, [recordingAction, reload, stopRecording]);

  const handleResetOne = useCallback(
    async (action: string) => {
      if (busy || recordingAction) {
        return;
      }
      setBusy(true);
      try {
        await AppService.SetKeyBinding(action, "");
        await reload();
        useMessageStore.getState().successMessage(t("settings.keybindings.reset_one_ok"));
      } catch (err) {
        useMessageStore
          .getState()
          .errorMessage(
            translateServiceError(err) ||
              t("settings.keybindings.reset_one_failed"),
          );
      } finally {
        setBusy(false);
      }
    },
    [busy, recordingAction, reload],
  );

  const handleResetAll = useCallback(async () => {
    if (busy || recordingAction) {
      return;
    }
    setBusy(true);
    try {
      await AppService.ResetAllKeyBindings();
      await reload();
      useMessageStore.getState().successMessage(t("settings.keybindings.reset_all_ok"));
    } catch (err) {
      useMessageStore
        .getState()
        .errorMessage(
          translateServiceError(err) ||
            t("settings.keybindings.reset_all_failed"),
        );
    } finally {
      setBusy(false);
    }
  }, [busy, recordingAction, reload]);

  const groups = useMemo(() => {
    void lang;
    const map = new Map<string, KeyBindingInfo[]>();
    for (const item of bindings) {
      const cat = item.category || t("settings.keybindings.category_other");
      const list = map.get(cat) ?? [];
      list.push(item);
      map.set(cat, list);
    }
    return Array.from(map.entries());
  }, [bindings, lang]);

  if (loading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box>
      <Stack
        direction="row"
        spacing={2}
        sx={{ mb: 2, alignItems: "center", justifyContent: "space-between" }}
      >
        <Typography variant="body2" color="text.secondary">
          {t("settings.keybindings.hint")}
        </Typography>
        <Button
          size="small"
          variant="outlined"
          disabled={busy || !!recordingAction}
          onClick={() => void handleResetAll()}
        >
          {t("settings.keybindings.reset_all")}
        </Button>
      </Stack>
      {groups.map(([category, items]) => (
        <Paper key={category} sx={{ p: 2, mb: 2 }} elevation={1}>
          <Typography variant="subtitle1" sx={{ mb: 1, fontWeight: 600 }}>
            {category}
          </Typography>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t("settings.keybindings.col_action")}</TableCell>
                  <TableCell>{t("settings.keybindings.col_shortcut")}</TableCell>
                  <TableCell align="right" width={120}>
                    {t("settings.keybindings.col_reset")}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {items.map((item) => {
                  const recording = recordingAction === item.action;
                  return (
                    <TableRow key={item.action} hover>
                      <TableCell>
                        {item.label}
                        {item.custom ? (
                          <Chip
                            label={t("settings.keybindings.customized")}
                            size="small"
                            sx={{ ml: 1 }}
                            color="primary"
                            variant="outlined"
                          />
                        ) : null}
                      </TableCell>
                      <TableCell
                        onClick={() => {
                          if (!busy && !recordingAction) {
                            void startRecording(item.action);
                          }
                        }}
                        sx={{
                          cursor:
                            busy || recordingAction ? "default" : "pointer",
                          fontFamily: recording
                            ? "inherit"
                            : CODE_FONT_FAMILY,
                          fontSize: "0.875rem",
                          letterSpacing: recording ? undefined : "0.02em",
                          color: recording ? "primary.main" : "text.primary",
                          userSelect: "none",
                        }}
                      >
                        {recording
                          ? t("settings.keybindings.press_shortcut")
                          : item.keys || "—"}
                      </TableCell>
                      <TableCell align="right">
                        <Button
                          size="small"
                          disabled={
                            !item.custom || busy || !!recordingAction
                          }
                          onClick={() => void handleResetOne(item.action)}
                        >
                          {t("settings.keybindings.reset_one")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      ))}
    </Box>
  );
};

export default KeybindingsSettings;
