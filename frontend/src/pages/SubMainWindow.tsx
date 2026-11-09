import { Box } from "@mui/material";
import SessionTabs from "../components/session/SessionTabs.tsx";
import Header from "../components/subwindow/Header.tsx";
import Message from "../components/common/Message.tsx";
import HostKeyPrompt from "../components/ssh/HostKeyPrompt";
import KeyboardInteractivePrompt from "../components/ssh/KeyboardInteractivePrompt";
import PasswordInputDialog from "../components/common/PasswordInputDialog";
import ShortcutListener from "../components/common/ShortcutListener";

function SubMainWindow() {
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
        <SessionTabs />
      </Box>
      <Message />
      <HostKeyPrompt />
      <KeyboardInteractivePrompt />
      <PasswordInputDialog />
    </>
  );
}

export default SubMainWindow;
