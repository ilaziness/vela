import React, { useState, useEffect } from "react";
import { Box, Paper } from "@mui/material";
import {
  SettingsNav,
  SettingsTab,
  GeneralSettings,
  TerminalSettings,
  SSHSettings,
  KnownHostsSettings,
  SyncSettings,
  AISettings,
  KeySettings,
  KeybindingsSettings,
  About,
} from "../components/settings";
import OpBar from "../components/OpBar";
import Message from "../components/Message";
import Loading from "../components/Loading";
import PasswordInputDialog from "../components/PasswordInputDialog";
import {
  Config,
  AppInfo,
  ConfigService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { GetAppInfo } from "../../bindings/github.com/ilaziness/vela/services/appservice";
import { useMessageStore } from "../stores/message";
import { parseCallServiceError } from "../func/service";
import { useFrameless } from "../hooks/useFrameless";

const Setting: React.FC = () => {
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [appInfo, setAppInfo] = useState<AppInfo>({} as AppInfo);
  const { successMessage, errorMessage } = useMessageStore();
  const frameless = useFrameless() === true;

  useEffect(() => {
    const loadConfig = async () => {
      try {
        const configData = await ConfigService.ReadConfig();
        setConfig(configData || new Config());
      } catch (error) {
        console.error("Failed to load config:", error);
      } finally {
        setLoading(false);
      }
    };

    loadConfig();
    GetAppInfo().then((info) => {
      setAppInfo(info);
    });
  }, []);

  const handleSyncSave = async (syncConfig: Config["Sync"]) => {
    if (!config) return;
    try {
      await ConfigService.SaveSyncConfig(syncConfig);
      setConfig((prev) =>
        prev ? { ...prev, Sync: { ...prev.Sync, ...syncConfig } } : prev
      );
      successMessage("同步配置保存成功");
    } catch (error) {
      console.error("Failed to save sync config:", error);
      errorMessage(parseCallServiceError(error) || "同步配置保存失败");
    }
  };

  if (loading || !config) {
    return (
      <Box
        sx={{
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Loading />
      </Box>
    );
  }

  return (
    <>
      {frameless && (
        <Box sx={{ height: 40 }}>
          <OpBar />
        </Box>
      )}
      <Box
        sx={{
          height: frameless ? "calc(100% - 40px)" : "100%",
          display: "flex",
          flexDirection: "column",
        }}
      >
        <Box sx={{ display: "flex", flex: 1, overflow: "hidden" }}>
          <Paper
            sx={{
              width: 150,
              flexShrink: 0,
              borderRight: 1,
              borderColor: "divider",
              display: "flex",
              flexDirection: "column",
            }}
            elevation={0}
            square
          >
            <SettingsNav activeTab={activeTab} onTabChange={setActiveTab} />
          </Paper>

          <Box
            sx={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              overflow: "hidden",
            }}
          >
            <Box
              sx={{
                flex: 1,
                overflowY: "auto",
                p: 2,
              }}
            >
              {activeTab === "general" && (
                <GeneralSettings config={config.General} />
              )}

              {activeTab === "terminal" && (
                <TerminalSettings
                  config={config.Terminal}
                  onSaved={(terminal) =>
                    setConfig((prev) =>
                      prev ? { ...prev, Terminal: terminal } : prev,
                    )
                  }
                />
              )}

              {activeTab === "ssh" && <SSHSettings config={config.SSH} />}

              {activeTab === "sync" && (
                <SyncSettings
                  syncConfig={config.Sync}
                  onChange={handleSyncSave}
                />
              )}

              {activeTab === "ai" && <AISettings />}

              {activeTab === "keys" && <KeySettings />}

              {activeTab === "knownHosts" && <KnownHostsSettings />}

              {activeTab === "keybindings" && <KeybindingsSettings />}

              {activeTab === "about" && (
                <Paper sx={{ p: 3 }} elevation={1}>
                  <About appinfo={appInfo} />
                </Paper>
              )}
            </Box>
          </Box>
        </Box>
      </Box>
      <Message />
      <PasswordInputDialog />
    </>
  );
};

export default Setting;
