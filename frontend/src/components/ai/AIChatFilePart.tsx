import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Box } from "@mui/material";
import InsertDriveFileOutlinedIcon from "@mui/icons-material/InsertDriveFileOutlined";
import type {
  ChatFileMessagePart,
  ChatPartRendererProps,
} from "@mui/x-chat/headless";
import { AIService } from "../../../bindings/github.com/ilaziness/vela/services";
import { fileToDataURL } from "../../func/aiAttachments";
import { parseCallServiceError } from "../../func/service";
import { useMessageStore } from "../../stores/message";

async function blobToDataURL(url: string): Promise<string> {
  return fileToDataURL(await (await fetch(url)).blob());
}

const chipSx = {
  display: "inline-flex",
  alignItems: "center",
  verticalAlign: "middle",
  gap: 0.5,
  boxSizing: "border-box",
  appearance: "none",
  height: 24,
  maxWidth: 180,
  mr: 0.75,
  my: 0.25,
  px: 0.75,
  py: 0,
  border: "1px solid",
  borderColor: "currentColor",
  borderRadius: 2,
  bgcolor: "transparent",
  color: "inherit",
  opacity: 0.75,
  cursor: "pointer",
  font: "inherit",
  fontSize: "0.75rem",
  lineHeight: 1,
  "&:hover": { opacity: 1 },
} as const;

export function AIChatFilePart(props: ChatPartRendererProps<ChatFileMessagePart>) {
  const { part } = props;
  const isImage = (part.mediaType || "").startsWith("image/");
  const label = part.filename || "file";
  const inlineSrc =
    part.url.startsWith("data:") || part.url.startsWith("blob:") ? part.url : "";
  const [thumbSrc, setThumbSrc] = useState(inlineSrc);
  const [previewSrc, setPreviewSrc] = useState("");

  useEffect(() => {
    if (!isImage || thumbSrc || !part.url) return;
    void AIService.ReadChatAttachment(part.url, part.mediaType || "")
      .then(setThumbSrc)
      .catch(() => {});
  }, [isImage, part.url, part.mediaType, thumbSrc]);

  useEffect(() => {
    if (!previewSrc) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setPreviewSrc("");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [previewSrc]);

  const handleOpen = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!part.url) return;
    try {
      if (isImage) {
        setPreviewSrc(
          thumbSrc ||
            (await AIService.ReadChatAttachment(part.url, part.mediaType || "")),
        );
        return;
      }
      const uri = part.url.startsWith("blob:")
        ? await blobToDataURL(part.url)
        : part.url;
      await AIService.OpenChatAttachment(uri, part.filename || "");
    } catch (err) {
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
    }
  };

  return (
    <>
      <Box
        component="button"
        type="button"
        onMouseDown={(e) => e.stopPropagation()}
        onClick={(e) => void handleOpen(e)}
        title={isImage ? `预览 ${label}` : `用系统默认应用打开 ${label}`}
        sx={chipSx}
      >
        {isImage && thumbSrc ? (
          <Box
            component="img"
            src={thumbSrc}
            alt=""
            sx={{
              width: 16,
              height: 16,
              objectFit: "cover",
              borderRadius: "3px",
              flexShrink: 0,
              pointerEvents: "none",
            }}
          />
        ) : (
          <InsertDriveFileOutlinedIcon sx={{ fontSize: 16, flexShrink: 0 }} />
        )}
        <Box
          component="span"
          sx={{
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            minWidth: 0,
          }}
        >
          {label}
        </Box>
      </Box>
      {previewSrc
        ? createPortal(
            <Box
              role="dialog"
              aria-modal="true"
              aria-label={label}
              onClick={() => setPreviewSrc("")}
              sx={(theme) => ({
                position: "fixed",
                inset: 0,
                zIndex: theme.zIndex.modal,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                gap: 1,
                bgcolor: "rgba(0, 0, 0, 0.78)",
                p: 2,
                cursor: "zoom-out",
              })}
            >
              <Box
                component="img"
                src={previewSrc}
                alt={label}
                onClick={(e) => e.stopPropagation()}
                sx={{
                  display: "block",
                  maxWidth: "92vw",
                  maxHeight: "88vh",
                  objectFit: "contain",
                  borderRadius: 1,
                  cursor: "default",
                }}
              />
              {part.filename ? (
                <Box sx={{ color: "#fff", fontSize: "0.8rem", opacity: 0.85 }}>
                  {part.filename}
                </Box>
              ) : null}
            </Box>,
            document.body,
          )
        : null}
    </>
  );
}
