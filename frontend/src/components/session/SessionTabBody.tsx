import React, {
  useCallback,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  memo,
} from "react";
import { Box, Button, Tab, Tabs, Typography } from "@mui/material";
import { Events } from "@wailsio/runtime";
import {
  LogService,
  SessionService,
  BookmarkService,
  ConfigService,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { ConnectionStatus } from "../../types/ssh";
import {
  SessionLink,
  SessionProtocol,
  isSSHProtocol,
  sessionTabLabel,
  toConnectRequest,
} from "../../types/session";
import Terminal from "../terminal/Terminal";
import Sftp from "../sftp/Sftp";
import ConnectionForm from "../common/ConnectionForm";
import Loading from "../common/Loading";
import { formatSSHConnectError, sleep } from "../../func/service";
import {
  useSessionTabsStore,
  useReloadSessionTabStore,
} from "../../stores/sessionTabs";
import { useMessageStore } from "../../stores/message";
import { useTransferStore } from "../../stores/transfer";
import { sftpOwnerKey } from "../../func/types";
import { SSH_STATUS_BAR_HEIGHT } from "../../func/aiSidebar";
import StatusBar from "../ssh/StatusBar";
import { t, tf, useLanguage } from "../../i18n";

interface SSHContainerProps {
  tabIndex: string;
  isActive: boolean;
}

const tabHeight = "26px";
const statusBarHeight = `${SSH_STATUS_BAR_HEIGHT}px`;
const AUTO_RECONNECT_MAX_ATTEMPTS = 5;

// SSH 连接容器组件，管理连接状态和错误处理
const SSHTabBody: React.FC<SSHContainerProps> = ({ tabIndex, isActive }) => {
  useLanguage();
  const setName = useSessionTabsStore((state) => state.setName);
  const setSessionLink = useSessionTabsStore((state) => state.setSessionLink);
  const getByIndex = useSessionTabsStore((state) => state.getByIndex);
  const reloadTab = useReloadSessionTabStore((state) => state.reloadTab);
  const setTabConnectionStatus = useSessionTabsStore(
    (state) => state.setTabConnectionStatus,
  );
  const { infoMessage, successMessage, errorMessage } = useMessageStore();
  const clearTransferSession = useTransferStore((s) => s.clearSession);
  // Local linkID = mounted Terminal/SFTP UI. Store sessionLink.linkID = Exec/AI session.
  // On unexpected drop, WS marks the tab Disconnected but keeps store linkID until
  // enterReconnectUI / connect failure (tab API). Local id stays so the closed
  // terminal remains visible through auto-reconnect. Tab dot follows connectionStatus.
  const [linkID, setLinkID] = React.useState<string>("");
  const [connectionError, setConnectionError] = React.useState<string>("");
  const [connecting, setConnecting] = React.useState<boolean>(false);
  const [activeTab, setActiveTab] = React.useState(0); // 0 for terminal, 1 for sftp
  const [sessionLogging, setSessionLogging] = React.useState(false);
  const [sftpLoaded, setSftpLoaded] = React.useState(false);
  const [sftpReady, setSftpReady] = React.useState(false);
  const [termReady, setTermReady] = React.useState(false);
  const [isReloading, setIsReloading] = React.useState<boolean>(false);
  const [lastSSHInfo, setLastSSHInfo] = React.useState<SessionLink | null>(
    null,
  );
  // showReconnect: session ended / reconnect exhausted — not first-time connect form.
  const [showReconnect, setShowReconnect] = React.useState(false);
  const reloadingRef = useRef(false);
  const skipAutoReconnectRef = useRef(false);
  const autoReconnectingRef = useRef(false);
  const linkIDRef = useRef(linkID);
  const lastSSHInfoRef = useRef(lastSSHInfo);
  linkIDRef.current = linkID;
  lastSSHInfoRef.current = lastSSHInfo;

  const tabInfo = useMemo(() => getByIndex(tabIndex), [tabIndex, getByIndex]);
  const sftpIndex = 1;
  const ownerKey = useMemo(() => {
    const info = lastSSHInfo || tabInfo?.sessionLink;
    return info ? sftpOwnerKey(info) : "";
  }, [lastSSHInfo, tabInfo?.sessionLink]);

  const clearSessionUI = () => {
    setLinkID("");
    linkIDRef.current = "";
    setSessionLogging(false);
    setSftpLoaded(false);
    setSftpReady(false);
    setTermReady(false);
  };

  // End session UI + tab disconnected (store clears sessionLink.linkID).
  const enterReconnectUI = (message: string) => {
    setTabConnectionStatus(tabIndex, ConnectionStatus.Disconnected);
    setConnectionError(message);
    setShowReconnect(true);
    clearSessionUI();
  };

  const isSSH = isSSHProtocol(lastSSHInfo || tabInfo?.sessionLink);

  // connect session (SSH or serial)
  const connect = useCallback(
    async (li: SessionLink): Promise<string> => {
      setConnectionError("");
      setShowReconnect(false);
      setConnecting(true);
      setLastSSHInfo({ ...li, linkID: undefined });
      try {
        LogService.Debug(`SessionLink ${JSON.stringify(li)}`);
        setTabConnectionStatus(tabIndex, ConnectionStatus.Connecting);
        let nextLinkID = "";
        if (li.bookmarkID != "" && li.bookmarkID != undefined) {
          nextLinkID = await BookmarkService.ConnectBookmarkByID(li.bookmarkID);
        } else {
          nextLinkID = await SessionService.Connect(toConnectRequest(li));
        }
        LogService.Debug(`Session established with ID: ${nextLinkID}`);
        setLinkID(nextLinkID);
        linkIDRef.current = nextLinkID;
        setSessionLogging(false);
        setTermReady(false);
        setName(tabIndex, sessionTabLabel(li));
        setSessionLink(tabIndex, { ...li, linkID: nextLinkID });
        // Stay Connecting until Terminal WebSocket onopen → Connected.
        return nextLinkID;
      } catch (err: any) {
        const msg = formatSSHConnectError(err);
        LogService.Error(`Connection failed: ${err?.message || err}`).then(
          () => {},
        );
        // 旧会话已在刷新时关掉，失败后不能再挂到这个 linkID 上。
        setTabConnectionStatus(tabIndex, ConnectionStatus.Disconnected);
        clearSessionUI();
        setConnectionError(msg);
        if (reloadingRef.current && !autoReconnectingRef.current) {
          setShowReconnect(true);
        }
        return "";
      } finally {
        setConnecting(false);
        if (!autoReconnectingRef.current) {
          setIsReloading(false);
        }
      }
    },
    [tabIndex, setName, setSessionLink, setTabConnectionStatus],
  );

  const connectRef = useRef(connect);
  connectRef.current = connect;

  const runAutoReconnectRef = useRef(async () => {});
  runAutoReconnectRef.current = async () => {
    if (
      skipAutoReconnectRef.current ||
      reloadingRef.current ||
      autoReconnectingRef.current
    ) {
      return;
    }
    const info = lastSSHInfoRef.current;
    if (!info) {
      // No credentials to retry — fall back to the connect form with an error.
      setTabConnectionStatus(tabIndex, ConnectionStatus.Disconnected);
      clearSessionUI();
      setConnectionError(t("ssh.disconnected"));
      setShowReconnect(false);
      return;
    }

    // Serial never auto-reconnects (plan: no auto-reconnect on serial).
    if (info.protocol !== SessionProtocol.SSH) {
      enterReconnectUI(t("ssh.disconnected"));
      return;
    }
    let autoReconnect = true;
    try {
      const cfg = await ConfigService.ReadConfig();
      autoReconnect = cfg?.SSH?.autoReconnect ?? true;
    } catch (err) {
      LogService.Error(`ReadConfig for auto-reconnect failed: ${err}`).then(
        () => {},
      );
    }
    if (!autoReconnect) {
      enterReconnectUI(t("ssh.disconnected"));
      return;
    }

    autoReconnectingRef.current = true;
    try {
      for (let attempt = 1; attempt <= AUTO_RECONNECT_MAX_ATTEMPTS; attempt++) {
        if (skipAutoReconnectRef.current) {
          return;
        }
        infoMessage(
          tf(
            "ssh.reconnecting",
            attempt,
            AUTO_RECONNECT_MAX_ATTEMPTS,
          ),
        );
        const delayMs = Math.min(1000 * 2 ** (attempt - 1), 8000);
        await sleep(delayMs);
        if (skipAutoReconnectRef.current) {
          return;
        }

        skipAutoReconnectRef.current = true;
        reloadingRef.current = true;
        setIsReloading(true);
        setActiveTab(0);
        setSftpLoaded(false);
        setSftpReady(false);
        setTermReady(false);
        try {
          const nextID = await connectRef.current(info);
          if (nextID) {
            successMessage(t("ssh.reconnected"));
            return;
          }
        } finally {
          reloadingRef.current = false;
          skipAutoReconnectRef.current = false;
        }
      }
      errorMessage(t("ssh.auto_reconnect_failed"));
      enterReconnectUI(t("ssh.auto_reconnect_failed"));
    } finally {
      autoReconnectingRef.current = false;
      setIsReloading(false);
    }
  };

  const tabItems = useMemo(() => {
    const items = [
      {
        label: isSSH ? t("session.protocol.ssh") : t("session.protocol.serial"),
        component: (
          <Terminal
            linkID={linkID}
            isActive={isActive && (!isSSH || activeTab === 0)}
            onLoggingChange={setSessionLogging}
            onReady={() => setTermReady(true)}
          />
        ),
      },
    ];
    if (isSSH) {
      items.push({
        label: "SFTP",
        component: (
          <Sftp
            key={linkID}
            linkID={linkID}
            ownerKey={ownerKey}
            onReady={() => setSftpReady(true)}
          />
        ),
      });
    }
    return items;
  }, [linkID, ownerKey, isActive, activeTab, isSSH]);

  const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
    if (isSSH && newValue === sftpIndex && !sftpLoaded) {
      setSftpLoaded(true);
    }
  };

  const onReloadTab = useEffectEvent(async () => {
    if (reloadTab.index !== tabIndex) {
      return;
    }
    LogService.Debug(`reload tab ${reloadTab.index} - ${tabIndex}`);
    skipAutoReconnectRef.current = true;
    if (linkID != "") {
      try {
        await SessionService.CloseByID(linkID);
      } catch (e) {
        console.warn("CloseByID error during reload:", e);
      }
    }
    // 重置状态
    setActiveTab(0);
    setSftpLoaded(false);
    setSftpReady(false);
    setTermReady(false);
    // 如果有保存的连接信息，重新连接
    if (lastSSHInfo) {
      reloadingRef.current = true;
      setIsReloading(true);
      try {
        await connect(lastSSHInfo);
      } finally {
        reloadingRef.current = false;
        skipAutoReconnectRef.current = false;
      }
    } else {
      skipAutoReconnectRef.current = false;
    }
  });

  useEffect(() => {
    void onReloadTab();
  }, [reloadTab]);

  const onSessionClosed = useEffectEvent((event: any) => {
    try {
      const raw =
        typeof event.data === "string" ? JSON.parse(event.data) : event.data;
      const closedID = raw?.id as string | undefined;
      const reason = raw?.reason as string | undefined;
      if (!closedID || closedID !== linkIDRef.current) {
        return;
      }
      // Clean exit (e.g. remote `exit`): show reconnect UI, no auto-reconnect.
      if (reason !== "unexpected") {
        enterReconnectUI(t("ssh.connection_closed"));
        return;
      }
      void runAutoReconnectRef.current();
    } catch (e) {
      console.error("Invalid SSH session closed payload", e);
    }
  });

  useEffect(() => {
    const unsubscribe = Events.On("eventSessionClosed", (event: any) => {
      onSessionClosed(event);
    });
    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    return () => {
      skipAutoReconnectRef.current = true;
    };
  }, []);

  useEffect(() => {
    return () => {
      if (linkID) {
        LogService.Debug(
          `SSHContainer unmounting, closing connection ${linkID}`,
        );
        clearTransferSession(linkID);
        SessionService.CloseByID(linkID).catch((err) => {
          console.warn("Error closing connection on unmount:", err);
        });
      }
    };
  }, [linkID, clearTransferSession]);

  const onMountConnect = useEffectEvent(() => {
    if (tabInfo?.sessionLink) {
      setIsReloading(true);
      void connect(tabInfo.sessionLink);
    }
  });

  useEffect(() => {
    onMountConnect();
  }, []);

  if (isReloading) {
    return <Loading message={t("common.loading")} />;
  }

  if (linkID === "") {
    if (showReconnect && connectionError) {
      return (
        <Box
          sx={{
            width: "100%",
            height: "100%",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 2,
            px: 3,
          }}
        >
          <Typography color="error" align="center">
            {connectionError}
          </Typography>
          <Button
            variant="contained"
            disabled={!lastSSHInfo || connecting}
            onClick={() => {
              if (!lastSSHInfo) return;
              reloadingRef.current = true;
              setIsReloading(true);
              connect(lastSSHInfo).finally(() => {
                reloadingRef.current = false;
              });
            }}
          >
            {t("ssh.reconnect")}
          </Button>
        </Box>
      );
    }
    return (
      <ConnectionForm
        onConnect={connect}
        error={connectionError}
        connecting={connecting}
      />
    );
  }

  const tabValue = Math.min(activeTab, Math.max(tabItems.length - 1, 0));

  return (
    <Box
      sx={{
        width: "100%",
        height: "100%",
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Tabs
        value={tabValue}
        onChange={handleTabChange}
        aria-label="ssh tabs"
        sx={{
          "&.MuiTabs-root": {
            minHeight: tabHeight,
            height: tabHeight,
            borderBottom: 1,
            borderColor: "divider",
          },
        }}
      >
        {tabItems.map((item) => (
          <Tab
            key={item.label}
            label={item.label}
            sx={{
              "&.MuiTab-root": {
                height: tabHeight,
                minHeight: tabHeight,
                fontSize: "0.8125rem",
                minWidth: 72,
                width: 72,
                py: 0,
                px: 1,
              },
            }}
          />
        ))}
      </Tabs>

      {/* 终端区域 */}
      <Box
        sx={{
          height: `calc(100% - ${tabHeight} - ${statusBarHeight})`,
          width: "100%",
          display: "flex",
          flexDirection: "row",
          overflow: "hidden",
        }}
      >
        {/* SSH/SFTP 内容区域 */}
        <Box
          sx={{
            width: "100%",
            height: "100%",
            position: "relative",
          }}
        >
          {tabItems.map(
            (item, index) =>
              (index != sftpIndex || sftpLoaded) && (
                <Box
                  key={item.label}
                  sx={{
                    width: "100%",
                    height: "100%",
                    position: "absolute",
                    top: 0,
                    left: 0,
                    visibility: tabValue === index ? "visible" : "hidden",
                    pointerEvents: tabValue === index ? "auto" : "none",
                    zIndex: tabValue === index ? 1 : 0,
                  }}
                >
                  {item.component}
                </Box>
              ),
          )}
        </Box>
      </Box>

      {isSSH ? (
        <StatusBar
          sessionID={linkID}
          height={statusBarHeight}
          isActive={isActive}
          termReady={termReady}
          logging={sessionLogging}
          sftpReady={sftpReady}
        />
      ) : (
        <Box
          sx={{
            height: statusBarHeight,
            minHeight: statusBarHeight,
            borderTop: 1,
            borderColor: "divider",
            px: 1,
            display: "flex",
            alignItems: "center",
            fontSize: "0.75rem",
            color: "text.secondary",
          }}
        >
          {sessionLogging ? t("session.logging") : null}
        </Box>
      )}
    </Box>
  );
};

export default memo(SSHTabBody);
