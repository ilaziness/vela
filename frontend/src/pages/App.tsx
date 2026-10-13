import { Box } from "@mui/material";
import SSHTabs from "../components/SSHTabs.tsx";
import Header from "../components/Header.tsx";
import Message from "../components/Message.tsx";
import PasswordInputDialog from "../components/PasswordInputDialog.tsx";
import HostKeyPrompt from "../components/HostKeyPrompt";
import KeyboardInteractivePrompt from "../components/KeyboardInteractivePrompt";
import ShortcutListener from "../components/ShortcutListener";
import { AppService } from "../../bindings/github.com/ilaziness/vela/services";

function App() {
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
        <SSHTabs onClose={() => AppService.MainWindowClose()} />
      </Box>
      <Message />
      <PasswordInputDialog />
      <HostKeyPrompt />
      <KeyboardInteractivePrompt />
    </>
  );
}

export default App;
