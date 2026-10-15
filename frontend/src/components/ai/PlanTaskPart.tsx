import { useEffect, useRef, useState, type SyntheticEvent } from 'react';
import { Box, CircularProgress, Typography } from '@mui/material';
import CheckCircleOutlinedIcon from '@mui/icons-material/CheckCircleOutlined';
import RadioButtonUncheckedOutlinedIcon from '@mui/icons-material/RadioButtonUncheckedOutlined';
import ErrorOutlinedIcon from '@mui/icons-material/ErrorOutlined';
import { UPSERT_PLAN_TOOL } from '../../func/aiContext';

export type PlanTaskStatus = 'pending' | 'running' | 'done' | 'failed';

export interface PlanTask {
  id: string;
  title: string;
  status: PlanTaskStatus;
}

export interface PlanSnapshot {
  title: string;
  tasks: PlanTask[];
}

function asPlan(value: unknown): PlanSnapshot | null {
  if (!value || typeof value !== 'object') return null;
  const v = value as Record<string, unknown>;
  if (!Array.isArray(v.tasks)) return null;
  const tasks: PlanTask[] = v.tasks.map((t: any, i: number) => ({
    id: String(t?.id ?? i),
    title: String(t?.title ?? `任务 ${i + 1}`),
    status: normalizeStatus(t?.status),
  }));
  return {
    title: String(v.title ?? '计划'),
    tasks,
  };
}

function normalizeStatus(s: unknown): PlanTaskStatus {
  const v = String(s || 'pending').toLowerCase();
  if (v === 'running' || v === 'done' || v === 'failed' || v === 'pending') return v;
  return 'pending';
}

function isTerminal(status: PlanTaskStatus): boolean {
  return status === 'done' || status === 'failed';
}

function allTerminal(tasks: PlanTask[]): boolean {
  return tasks.length > 0 && tasks.every((t) => isTerminal(t.status));
}

function anyRunning(tasks: PlanTask[]): boolean {
  return tasks.some((t) => t.status === 'running');
}

function anyFailed(tasks: PlanTask[]): boolean {
  return tasks.some((t) => t.status === 'failed');
}

function doneCount(tasks: PlanTask[]): number {
  return tasks.filter((t) => t.status === 'done').length;
}

function planBadge(tasks: PlanTask[]): { label: string; palette: 'primary' | 'success' | 'error' } {
  const total = tasks.length;
  if (!allTerminal(tasks)) {
    return { label: `${doneCount(tasks)} / ${total}`, palette: 'primary' };
  }
  if (anyFailed(tasks)) {
    return { label: '失败', palette: 'error' };
  }
  return { label: '完成', palette: 'success' };
}

function StatusIcon({ status }: { status: PlanTaskStatus }) {
  switch (status) {
    case 'done':
      return <CheckCircleOutlinedIcon color="success" fontSize="small" />;
    case 'failed':
      return <ErrorOutlinedIcon color="error" fontSize="small" />;
    case 'running':
      return <CircularProgress size={16} color="primary" />;
    default:
      return <RadioButtonUncheckedOutlinedIcon sx={{ color: 'text.disabled' }} fontSize="small" />;
  }
}

export interface PlanTaskPartProps {
  plan: PlanSnapshot | null;
}

/** Renders upsert_plan tool output as a collapsible task list. */
export function PlanTaskPart({ plan }: PlanTaskPartProps) {
  if (!plan || plan.tasks.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        （空计划）
      </Typography>
    );
  }
  return <PlanTaskList plan={plan} />;
}

function PlanTaskList({ plan }: { plan: PlanSnapshot }) {
  const { tasks } = plan;
  const terminal = allTerminal(tasks);
  const running = anyRunning(tasks);
  const badge = planBadge(tasks);

  const [open, setOpen] = useState(() => running || !terminal);
  const prevTerminalRef = useRef(terminal);

  useEffect(() => {
    if (running) {
      setOpen(true);
    } else if (terminal && !prevTerminalRef.current) {
      setOpen(false);
    }
    prevTerminalRef.current = terminal;
  }, [running, terminal]);

  const handleToggle = (e: SyntheticEvent<HTMLDetailsElement>) => {
    setOpen(e.currentTarget.open);
  };

  return (
    <Box
      component="details"
      open={open}
      onToggle={handleToggle}
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        bgcolor: 'background.paper',
        minWidth: 220,
        '& summary': {
          listStyle: 'none',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          px: 1.5,
          py: 1,
          '&::-webkit-details-marker': { display: 'none' },
        },
      }}
    >
      <Box component="summary">
        <Typography variant="subtitle2" sx={{ fontWeight: 600, flex: 1, minWidth: 0 }}>
          {plan.title || '计划'}
        </Typography>
        <Typography
          component="span"
          variant="caption"
          sx={{
            flexShrink: 0,
            px: 0.75,
            py: 0.25,
            borderRadius: 1,
            fontWeight: 600,
            bgcolor: (theme) =>
              badge.palette === 'error'
                ? theme.palette.error.main + '1a'
                : badge.palette === 'success'
                  ? theme.palette.success.main + '1a'
                  : theme.palette.primary.main + '1a',
            color: `${badge.palette}.main`,
          }}
        >
          {badge.label}
        </Typography>
      </Box>
      <Box sx={{ px: 1.5, pb: 1, pt: 0.25 }}>
        {tasks.map((task) => (
          <Box
            key={task.id}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1,
              py: 0.375,
              minHeight: 28,
            }}
          >
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 24,
                flexShrink: 0,
              }}
            >
              <StatusIcon status={task.status} />
            </Box>
            <Typography variant="body2" sx={{ flex: 1, minWidth: 0 }}>
              {task.title}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export function planFromToolPart(part: {
  toolInvocation?: { input?: unknown; output?: unknown; toolName?: string };
}): PlanSnapshot | null {
  const inv = part.toolInvocation;
  if (!inv || inv.toolName !== UPSERT_PLAN_TOOL) return null;
  return asPlan(inv.output) || asPlan(inv.input);
}

/** Last upsert_plan in message order (newest wins). */
export function latestPlanFromMessages(
  messages: ReadonlyArray<{ parts?: ReadonlyArray<unknown> }>,
): PlanSnapshot | null {
  let latest: PlanSnapshot | null = null;
  for (const message of messages) {
    for (const part of message.parts ?? []) {
      const plan = planFromToolPart(part as Parameters<typeof planFromToolPart>[0]);
      if (plan) {
        latest = plan;
      }
    }
  }
  return latest;
}
