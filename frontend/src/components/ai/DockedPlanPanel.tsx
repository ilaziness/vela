import React, { useMemo } from 'react';
import { Box, type SxProps, type Theme } from '@mui/material';
import { useChat, useChatComposer } from '@mui/x-chat/headless';
import { latestPlanFromMessages, PlanTaskPart } from './PlanTaskPart';
import {
  AI_PASTE_MIME_TYPES,
  filterAttachFiles,
  formatAttachmentRejectMessage,
  imagesFromClipboard,
  localFilePath,
} from '../../func/aiAttachments';
import { useMessageStore } from '../../stores/message';

const composerInnerSx: SxProps<Theme> = (theme) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'stretch',
  position: 'relative',
  gap: theme.spacing(0.5),
  padding: theme.spacing(1, 1.5),
  border: '1px solid',
  borderColor: (theme.vars || theme).palette.divider,
  borderRadius: (theme.vars || theme).shape.borderRadius,
  backgroundColor: (theme.vars || theme).palette.background.paper,
  boxSizing: 'border-box',
  // Let the textarea size to its content; do not stretch it to fill leftover height.
  '& .MuiChatComposer-textArea': {
    flex: '0 0 auto',
  },
  transition: theme.transitions.create(['border-color', 'box-shadow'], {
    duration: theme.transitions.duration.short,
  }),
  '@media (prefers-reduced-motion: reduce)': {
    transition: 'none',
  },
  '&:focus-within': {
    borderColor: (theme.vars || theme).palette.primary.main,
    boxShadow: `0 0 0 1px ${(theme.vars || theme).palette.primary.main}`,
  },
  'form[data-disabled] &': {
    backgroundColor: (theme.vars || theme).palette.action.disabledBackground,
    opacity: (theme.vars || theme).palette.action.disabledOpacity,
    pointerEvents: 'none',
  },
});

function reportRejections(
  rejections: ReturnType<typeof filterAttachFiles>['rejections'],
) {
  const msg = formatAttachmentRejectMessage(rejections);
  if (msg) useMessageStore.getState().errorMessage(msg);
}

/** Composer root slot: plan dock above the input, updates in place. */
export const ComposerPlanDockRoot = React.forwardRef<
  HTMLFormElement,
  React.ComponentPropsWithoutRef<'form'> & { sx?: SxProps<Theme> }
>(function ComposerPlanDockRoot({ children, sx, onPaste, onDragOver, onDrop, ...other }, ref) {
  const { messages } = useChat();
  const composer = useChatComposer();
  const plan = useMemo(() => latestPlanFromMessages(messages), [messages]);

  // Capture before TextArea's Enter→submit; use composer store (same as send button).
  const handleKeyDownCapture: React.KeyboardEventHandler<HTMLFormElement> = (event) => {
    if (
      event.key === 'Enter' &&
      !event.shiftKey &&
      !event.nativeEvent.isComposing &&
      composer.value.trim() === ''
    ) {
      event.preventDefault();
    }
  };

  const handlePaste: React.ClipboardEventHandler<HTMLFormElement> = (event) => {
    onPaste?.(event);
    if (event.defaultPrevented) return;

    const images = imagesFromClipboard(event.clipboardData);
    if (images.length === 0) return;

    const { accepted, rejections } = filterAttachFiles(images, {
      allow: AI_PASTE_MIME_TYPES,
      existingCount: composer.attachments.length,
    });
    reportRejections(rejections);
    // Nothing to attach: keep default paste so accompanying text still lands.
    if (accepted.length === 0) return;

    const plain = event.clipboardData.getData('text/plain');
    event.preventDefault();
    for (const file of accepted) {
      composer.addAttachment(file);
    }
    if (plain) {
      const next = composer.value ? `${composer.value}${plain}` : plain;
      composer.setValue(next);
    }
  };

  const handleDragOver: React.DragEventHandler<HTMLFormElement> = (event) => {
    onDragOver?.(event);
    if (event.defaultPrevented) return;
    if (event.dataTransfer?.types?.includes('Files')) {
      event.preventDefault();
      event.dataTransfer.dropEffect = 'copy';
    }
  };

  const handleDrop: React.DragEventHandler<HTMLFormElement> = (event) => {
    onDrop?.(event);
    if (event.defaultPrevented) return;
    const files = event.dataTransfer?.files;
    if (!files?.length) return;
    event.preventDefault();
    const { accepted, rejections } = filterAttachFiles(Array.from(files), {
      existingCount: composer.attachments.length,
    });
    reportRejections(rejections);
    let missingPath = false;
    for (const file of accepted) {
      if (!localFilePath(file)) {
        missingPath = true;
        continue;
      }
      composer.addAttachment(file);
    }
    if (missingPath) {
      useMessageStore.getState().errorMessage('无法获取文件路径，请使用附件按钮选择文件');
    }
  };

  return (
    <Box
      component="form"
      ref={ref}
      {...other}
      onKeyDownCapture={handleKeyDownCapture}
      onPaste={handlePaste}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      sx={[
        (theme) => ({
          display: 'flex',
          flexDirection: 'column',
          flexShrink: 0,
          gap: theme.spacing(1),
          margin: theme.spacing(0, 1.5, 1.5),
          boxSizing: 'border-box',
        }),
        ...(Array.isArray(sx) ? sx : sx ? [sx] : []),
      ]}
    >
      {plan ? <PlanTaskPart plan={plan} /> : null}
      <Box sx={composerInnerSx}>{children}</Box>
    </Box>
  );
});
