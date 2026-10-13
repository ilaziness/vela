import { create } from 'zustand';
import {
  AIService,
} from '../../bindings/github.com/ilaziness/vela/services/index';
import type { AISession } from '../../bindings/github.com/ilaziness/vela/services/models';
import { parseCallServiceError } from '../func/service';
import { useMessageStore } from './message';
import { loadStoredSidebarWidth, persistSidebarWidth, clampSidebarWidth } from '../func/aiSidebar';

export interface AISessionItem {
  id: string;
  title: string;
  updatedAt: number;
}

function mapSession(session: AISession): AISessionItem {
  return {
    id: session.id,
    title: session.title || '新会话',
    updatedAt: session.updated_at,
  };
}

interface AIAssistantState {
  sidebarOpen: boolean;
  sidebarWidth: number;
  sessions: AISessionItem[];
  activeSessionId: string;
  historyDrawerOpen: boolean;
  loadingSessions: boolean;
  isStreaming: boolean;
  composerValue: string;

  toggleSidebarOpen: () => void;
  setSidebarOpen: (open: boolean) => void;
  setSidebarWidth: (width: number, persist?: boolean) => void;
  setHistoryDrawerOpen: (open: boolean) => void;
  setStreaming: (streaming: boolean) => void;
  setComposerValue: (value: string) => void;
  appendToComposer: (text: string) => void;
  loadSessions: () => Promise<void>;
  selectSession: (id: string) => void;
  createSession: () => Promise<boolean>;
  deleteSession: (id: string) => Promise<void>;
  refreshActiveSessionTitle: () => Promise<void>;
}

export const useAIAssistantStore = create<AIAssistantState>((set, get) => ({
  sidebarOpen: false,
  sidebarWidth: loadStoredSidebarWidth(),
  sessions: [],
  activeSessionId: '',
  historyDrawerOpen: false,
  loadingSessions: false,
  isStreaming: false,
  composerValue: '',

  toggleSidebarOpen: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),

  setSidebarOpen: (open) => set({ sidebarOpen: open }),

  setSidebarWidth: (width, persist = true) => {
    const clamped = clampSidebarWidth(width);
    if (persist) {
      persistSidebarWidth(clamped);
    }
    set({ sidebarWidth: clamped });
  },

  setHistoryDrawerOpen: (open) => set({ historyDrawerOpen: open }),

  setStreaming: (streaming) => set({ isStreaming: streaming }),

  setComposerValue: (value) => set({ composerValue: value }),

  appendToComposer: (text) => {
    if (!text) return;
    const { composerValue } = get();
    const next = composerValue
      ? `${composerValue}\n\n${text}`
      : text;
    set({ composerValue: next, sidebarOpen: true });
  },

  loadSessions: async () => {
    if (get().loadingSessions) return;
    const soft = Boolean(get().activeSessionId) || get().sessions.length > 0;
    if (!soft) {
      set({ loadingSessions: true });
    }
    try {
      const sessions = await AIService.ListSessions(50);
      const items = (sessions || [])
        .filter((s): s is AISession => s !== null)
        .map(mapSession);

      if (items.length === 0) {
        set({ sessions: [] });
        await get().createSession();
        set({ loadingSessions: false });
        return;
      }
      const { activeSessionId } = get();
      const nextActive = items.some((s) => s.id === activeSessionId)
        ? activeSessionId
        : items[0].id;

      set({ sessions: items, activeSessionId: nextActive, loadingSessions: false });
    } catch (err) {
      set({ loadingSessions: false });
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
    }
  },

  selectSession: (id) => {
    if (get().isStreaming) return;
    set({ activeSessionId: id, historyDrawerOpen: false });
  },

  createSession: async (): Promise<boolean> => {
    if (get().isStreaming) return false;
    try {
      const session = await AIService.CreateSession();
      if (!session) {
        useMessageStore.getState().errorMessage('创建会话失败');
        return false;
      }
      const item = mapSession(session);
      set((state) => ({
        sessions: [item, ...state.sessions.filter((s) => s.id !== item.id)],
        activeSessionId: item.id,
        historyDrawerOpen: false,
      }));
      return true;
    } catch (err) {
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
      return false;
    }
  },

  deleteSession: async (id) => {
    if (get().isStreaming) return;
    try {
      await AIService.DeleteSession(id);
      const { sessions, activeSessionId } = get();
      const remaining = sessions.filter((s) => s.id !== id);

      if (remaining.length === 0) {
        set({ sessions: [], activeSessionId: '', loadingSessions: true });
        await get().createSession();
        set({ loadingSessions: false });
        return;
      }

      const nextActive = activeSessionId === id ? remaining[0].id : activeSessionId;
      set({ sessions: remaining, activeSessionId: nextActive });
    } catch (err) {
      useMessageStore.getState().errorMessage(parseCallServiceError(err));
    }
  },

  refreshActiveSessionTitle: async () => {
    const { activeSessionId } = get();
    if (!activeSessionId) return;
    try {
      const session = await AIService.GetSession(activeSessionId);
      if (!session) return;
      const item = mapSession(session);
      set((state) => ({
        sessions: state.sessions
          .map((s) => (s.id === item.id ? item : s))
          .sort((a, b) => b.updatedAt - a.updatedAt),
      }));
    } catch {
      // ignore title refresh errors
    }
  },
}));
