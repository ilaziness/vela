import React, { useEffect, useRef, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@mui/material";
import { Events } from "@wailsio/runtime";
import { SSHService } from "../../bindings/github.com/ilaziness/vela/services";
import { useMessageStore } from "../stores/message";
import { t } from "../i18n";
import { translateServiceError } from "../func/service";

interface KeyboardQuestion {
  prompt: string;
  echo: boolean;
}

interface KeyboardChallenge {
  id: string;
  host: string;
  user: string;
  name: string;
  instruction: string;
  questions: KeyboardQuestion[];
}

function parseChallenge(data: unknown): KeyboardChallenge | null {
  const raw = typeof data === "string" ? JSON.parse(data) : data;
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const payload = raw as Record<string, unknown>;
  if (typeof payload.id !== "string" || payload.id === "") {
    return null;
  }
  const questions = Array.isArray(payload.questions) ? payload.questions : [];
  return {
    id: payload.id,
    host: typeof payload.host === "string" ? payload.host : "",
    user: typeof payload.user === "string" ? payload.user : "",
    name: typeof payload.name === "string" ? payload.name : "",
    instruction:
      typeof payload.instruction === "string" ? payload.instruction : "",
    questions: questions.map((item) => {
      const question = (item ?? {}) as Record<string, unknown>;
      return {
        prompt: typeof question.prompt === "string" ? question.prompt : "",
        echo: question.echo === true,
      };
    }),
  };
}

const KeyboardInteractivePrompt: React.FC = () => {
  const { errorMessage } = useMessageStore();
  const [queue, setQueue] = useState<KeyboardChallenge[]>([]);
  const [answers, setAnswers] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const handled = useRef(new Set<string>());
  const current = queue[0] ?? null;
  const currentID = current?.id ?? "";
  const [answersFor, setAnswersFor] = useState("");
  if (answersFor !== currentID) {
    setAnswersFor(currentID);
    setAnswers((current?.questions ?? []).map(() => ""));
  }

  useEffect(() => {
    const unsubscribe = Events.On("eventKeyboardInteractive", (event: { data?: unknown }) => {
      try {
        const challenge = parseChallenge(event.data);
        if (!challenge) {
          console.error("Invalid keyboard-interactive payload", event.data);
          return;
        }
        setQueue((items) => [...items, challenge]);
      } catch (e) {
        console.error("Invalid keyboard-interactive payload", e);
      }
    });
    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    const unsubscribe = Events.On("eventKeyboardInteractiveClose", (event: { data?: unknown }) => {
      const id = typeof event.data === "string" ? event.data : "";
      if (!id) {
        return;
      }
      handled.current.add(id);
      setQueue((items) => items.filter((item) => item.id !== id));
    });
    return () => {
      unsubscribe();
    };
  }, []);

  const drop = (id: string) => {
    setQueue((items) => items.filter((item) => item.id !== id));
  };

  const handleSubmit = async () => {
    if (!current || busy || handled.current.has(current.id)) {
      return;
    }
    const id = current.id;
    const payload = current.questions.map((_, index) => answers[index] ?? "");
    setBusy(true);
    try {
      await SSHService.AnswerKeyboardInteractive(id, payload);
      handled.current.add(id);
      drop(id);
    } catch (err) {
      errorMessage(translateServiceError(err));
    } finally {
      setBusy(false);
    }
  };

  const handleCancel = async () => {
    if (!current || busy || handled.current.has(current.id)) {
      return;
    }
    const id = current.id;
    handled.current.add(id);
    drop(id);
    try {
      await SSHService.CancelKeyboardInteractive(id);
    } catch (err) {
      errorMessage(translateServiceError(err));
    }
  };

  const target =
    current == null
      ? ""
      : current.user
        ? `${current.user}@${current.host}`
        : current.host;

  return (
    <Dialog
      open={current != null}
      onClose={() => {
        void handleCancel();
      }}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>{t("keyboard.title")}</DialogTitle>
      <DialogContent>
        {current && (
          <>
            {target && (
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                {target}
              </Typography>
            )}
            {current.name && (
              <Typography variant="body2" sx={{ mb: 1 }}>
                {current.name}
              </Typography>
            )}
            {current.instruction && (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ mb: 2, whiteSpace: "pre-wrap" }}
              >
                {current.instruction}
              </Typography>
            )}
            {current.questions.map((question, index) => (
              <TextField
                key={`${current.id}-${index}`}
                fullWidth
                autoFocus={index === 0}
                margin="dense"
                size="small"
                label={question.prompt || t("keyboard.default_prompt")}
                type={question.echo ? "text" : "password"}
                value={answers[index] ?? ""}
                onChange={(event) => {
                  const value = event.target.value;
                  setAnswers((prev) => {
                    const next = [...prev];
                    next[index] = value;
                    return next;
                  });
                }}
                onKeyDown={(event) => {
                  if (
                    event.key === "Enter" &&
                    index === current.questions.length - 1
                  ) {
                    event.preventDefault();
                    void handleSubmit();
                  }
                }}
              />
            ))}
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => void handleCancel()} disabled={busy}>
          {t("common.cancel")}
        </Button>
        <Button
          onClick={() => void handleSubmit()}
          variant="contained"
          disabled={busy}
        >
          {t("common.ok")}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default KeyboardInteractivePrompt;
