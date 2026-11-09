import { Snackbar, Alert } from "@mui/material";
import { useMessageStore } from "../../stores/message.ts";

function Message() {
  const { message, setClose } = useMessageStore();
  return (
    <>
      <Snackbar
        open={message.open}
        onClose={setClose}
        autoHideDuration={3000}
      >
        <Alert
          onClose={setClose}
          severity={message.type}
          sx={{ width: '100%' }}
        >
          {message.text}
        </Alert>
      </Snackbar>
    </>
  );
}

export default Message;
