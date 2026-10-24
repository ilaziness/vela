import React, { useMemo } from 'react';
import { Box, type SxProps, type Theme } from '@mui/material';
import { useChat } from '@mui/x-chat/headless';
import { latestPlanFromMessages, PlanTaskPart } from './PlanTaskPart';

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

/** Composer root slot: plan dock above the input, updates in place. */
export const ComposerPlanDockRoot = React.forwardRef<
  HTMLFormElement,
  React.ComponentPropsWithoutRef<'form'> & { sx?: SxProps<Theme> }
>(function ComposerPlanDockRoot({ children, sx, ...other }, ref) {
  const { messages } = useChat();
  const plan = useMemo(() => latestPlanFromMessages(messages), [messages]);

  return (
    <Box
      component="form"
      ref={ref}
      {...other}
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
