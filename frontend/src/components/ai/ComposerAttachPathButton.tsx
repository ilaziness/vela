import React from "react";
import { IconButton, Tooltip } from "@mui/material";
import ScreenshotMonitorOutlined from "@mui/icons-material/ScreenshotMonitorOutlined";
import { ChatComposerAttachButton, type ChatComposerAttachButtonProps } from "@mui/x-chat";
import { useChatComposer } from "@mui/x-chat/headless";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services";
import {
  AI_ATTACH_MAX_COUNT,
  dataURLToFile,
  formatAttachmentRejectMessage,
  rememberLocalFilePath,
} from "../../func/aiAttachments";
import { captureCurrentTerminalPng } from "../../func/terminalScreenshot";
import { t } from "../../i18n";
import { useMessageStore } from "../../stores/message";
import { translateServiceError } from "../../func/service";

async function addPickedLocalFiles(
  composer: ReturnType<typeof useChatComposer>,
) {
  let paths: string[] | null;
  try {
    paths = await AIService.PickChatAttachments();
  } catch (err) {
    useMessageStore.getState().errorMessage(translateServiceError(err));
    return;
  }
  if (!paths?.length) return;

  const remaining = AI_ATTACH_MAX_COUNT - composer.attachments.length;
  if (remaining <= 0) {
    useMessageStore
      .getState()
      .errorMessage(formatAttachmentRejectMessage([{ reason: "file-count" }]));
    return;
  }
  if (paths.length > remaining) {
    useMessageStore
      .getState()
      .errorMessage(formatAttachmentRejectMessage([{ reason: "file-count" }]));
    paths = paths.slice(0, remaining);
  }

  for (const path of paths) {
    try {
      const dataURL = await AIService.ReadLocalAttachment(path);
      const mime = dataURL.slice(5, dataURL.indexOf(";")) || "";
      const name = path.replace(/^.*[/\\]/, "") || "file";
      const file = await dataURLToFile(dataURL, name, mime);
      rememberLocalFilePath(file, path);
      composer.addAttachment(file);
    } catch (err) {
      useMessageStore.getState().errorMessage(translateServiceError(err));
    }
  }
}

function ComposerTerminalScreenshotButton({
  composer,
}: {
  composer: ReturnType<typeof useChatComposer>;
}) {
  const label = t("ai.chat.screenshot");

  const handleClick = async (event: React.MouseEvent) => {
    event.preventDefault();
    const file = await captureCurrentTerminalPng();
    if (!file) {
      useMessageStore.getState().errorMessage(t("ai.chat.screenshot_unavailable"));
      return;
    }
    if (composer.attachments.length >= AI_ATTACH_MAX_COUNT) {
      useMessageStore
        .getState()
        .errorMessage(formatAttachmentRejectMessage([{ reason: "file-count" }]));
      return;
    }
    composer.addAttachment(file);
  };

  return (
    <Tooltip title={label}>
      <span>
        <IconButton
          type="button"
          aria-label={label}
          disabled={composer.isSubmitting}
          disableRipple
          onClick={(event) => void handleClick(event)}
          sx={{
            width: 36,
            height: 36,
            p: 0,
            borderRadius: "50%",
            fontSize: "1.25rem",
            color: "text.secondary",
            "&:hover": {
              color: "text.primary",
              bgcolor: "action.hover",
            },
            "& svg": { width: "1em", height: "1em", fontSize: "inherit" },
          }}
        >
          <ScreenshotMonitorOutlined fontSize="inherit" />
        </IconButton>
      </span>
    </Tooltip>
  );
}

/** Attach picker plus terminal screenshot; picker files keep the original path. */
export const ComposerAttachPathButton = React.forwardRef<
  HTMLButtonElement,
  ChatComposerAttachButtonProps
>(function ComposerAttachPathButton(props, ref) {
  const composer = useChatComposer();
  return (
    <>
      <ChatComposerAttachButton
        {...props}
        ref={ref}
        onClick={(event) => {
          event.preventDefault();
          void addPickedLocalFiles(composer);
        }}
      />
      <ComposerTerminalScreenshotButton composer={composer} />
    </>
  );
});
