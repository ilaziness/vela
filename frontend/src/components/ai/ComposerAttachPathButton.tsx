import React from "react";
import { ChatComposerAttachButton, type ChatComposerAttachButtonProps } from "@mui/x-chat";
import { useChatComposer } from "@mui/x-chat/headless";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services";
import {
  AI_ATTACH_MAX_COUNT,
  dataURLToFile,
  formatAttachmentRejectMessage,
  rememberLocalFilePath,
} from "../../func/aiAttachments";
import { parseCallServiceError } from "../../func/service";
import { useMessageStore } from "../../stores/message";

async function addPickedLocalFiles(
  composer: ReturnType<typeof useChatComposer>,
) {
  let paths: string[] | null;
  try {
    paths = await AIService.PickChatAttachments();
  } catch (err) {
    useMessageStore.getState().errorMessage(parseCallServiceError(err));
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
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
    }
  }
}

/** Native file picker so picker attachments store the original path, not a copy. */
export const ComposerAttachPathButton = React.forwardRef<
  HTMLButtonElement,
  ChatComposerAttachButtonProps
>(function ComposerAttachPathButton(props, ref) {
  const composer = useChatComposer();
  return (
    <ChatComposerAttachButton
      {...props}
      ref={ref}
      onClick={(event) => {
        event.preventDefault();
        void addPickedLocalFiles(composer);
      }}
    />
  );
});
