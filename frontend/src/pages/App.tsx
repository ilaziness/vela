import { useEffect, useState } from "react";
import { Box, CircularProgress } from "@mui/material";
import SessionTabs from "../components/session/SessionTabs.tsx";
import Header from "../components/common/Header.tsx";
import Message from "../components/common/Message.tsx";
import PasswordInputDialog from "../components/common/PasswordInputDialog.tsx";
import VaultUnlockGate from "../components/common/VaultUnlockGate.tsx";
import HostKeyPrompt from "../components/ssh/HostKeyPrompt";
import KeyboardInteractivePrompt from "../components/ssh/KeyboardInteractivePrompt";
import ShortcutListener from "../components/common/ShortcutListener";
import { AppService } from "../../bindings/github.com/ilaziness/vela/services";
import * as ConfigService from "../../bindings/github.com/ilaziness/vela/services/configservice";

type GateState = "loading" | "locked" | "open";

function App() {
  const [gate, setGate] = useState<GateState>("loading");

  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const needs = await ConfigService.NeedsUnlock();
        if (!alive) return;
        setGate(needs ? "locked" : "open");
      } catch (err) {
        console.error("NeedsUnlock failed", err);
        if (alive) setGate("open");
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  if (gate === "loading") {
    return (
      <Box
        sx={{
          height: "100%",
          width: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <CircularProgress size={32} />
      </Box>
    );
  }

  if (gate === "locked") {
    return <VaultUnlockGate onUnlocked={() => setGate("open")} />;
  }

  return (
    <>
      <ShortcutListener />
      <Box
        sx={{
          display: "flex",
          flexDirection: "row",
          height: "100%",
          width: "100%",
        }}
      >
        <Header />
        <SessionTabs onClose={() => AppService.MainWindowClose()} />
      </Box>
      <Message />
      <PasswordInputDialog />
      <HostKeyPrompt />
      <KeyboardInteractivePrompt />
    </>
  );
}

export default App;
