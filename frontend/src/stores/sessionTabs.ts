import { create } from "zustand";
import { ConnectionStatus } from "../types/ssh";
import type { SessionLink, SessionTab } from "../types/session";
import { genTabIndex } from "../func/service";
import { isNewConnectionTabName, NEW_CONNECTION_TAB_ID } from "../i18n";
import {
  tabWithConnectionStatus,
  tabWithSessionConnectionStatus,
} from "./connectionStatus";

export {
  tabWithConnectionStatus,
  tabWithSessionConnectionStatus,
} from "./connectionStatus";

export interface SessionTabsState {
  sessionTabs: SessionTab[];
  currentTab: string;
  pushTab: (tab: SessionTab) => void;
  delTab: (delTab: string, currentTab: string) => string;
  setName: (index: string, name: string) => void;
  setCurrentTab: (index: string) => void;
  getByIndex: (index: string) => SessionTab | undefined;
  setSessionLink: (index: string, sessionLink: SessionLink) => void;
  reorderTabs: (sourceIndex: number, destinationIndex: number) => void;
  setSessionConnectionStatus: (
    linkID: string,
    status: ConnectionStatus,
  ) => void;
  setTabConnectionStatus: (
    tabIndex: string,
    status: ConnectionStatus,
  ) => void;
}

const initialTabIndex = genTabIndex();

export const useSessionTabsStore = create<SessionTabsState>((set, get) => ({
  sessionTabs: [
    {
      index: initialTabIndex,
      name: NEW_CONNECTION_TAB_ID,
    },
  ] as SessionTab[],
  currentTab: initialTabIndex,
  pushTab: (tab: SessionTab) =>
    set((state) => {
      if (tab.sessionLink) {
        const emptyIndex = state.sessionTabs.findIndex((t) => !t.sessionLink);
        if (emptyIndex !== -1) {
          const newTabs = [...state.sessionTabs];
          const replacedIndex = newTabs[emptyIndex].index;
          newTabs[emptyIndex] = tab;
          const newCurrentTab =
            state.currentTab === replacedIndex ? tab.index : state.currentTab;
          return { sessionTabs: newTabs, currentTab: newCurrentTab };
        }
      }
      return { sessionTabs: [...state.sessionTabs, tab] };
    }),
  delTab: (delTab: string, currentTab: string): string => {
    const tabIndex = get().sessionTabs.findIndex((item) => item.index === delTab);
    if (tabIndex === -1) {
      return "0";
    }
    let newTabs = get().sessionTabs.filter((item) => item.index !== delTab);

    let newTabValue = currentTab;
    if (currentTab === delTab) {
      const lastTab = newTabs.at(-1);
      if (lastTab) {
        newTabValue = lastTab.index;
      } else {
        newTabs = [
          {
            index: `${genTabIndex()}`,
            name: NEW_CONNECTION_TAB_ID,
          },
        ];
        newTabValue = newTabs[0].index;
      }
    }
    set({ sessionTabs: newTabs, currentTab: newTabValue });
    return newTabValue;
  },
  setName: (index: string, name: string) =>
    set((state) => ({
      sessionTabs: state.sessionTabs.map((tab) =>
        tab.index === index && isNewConnectionTabName(tab.name)
          ? { ...tab, name }
          : tab,
      ),
    })),
  setCurrentTab: (index: string) => set({ currentTab: index }),
  getByIndex: (index: string): SessionTab | undefined =>
    get().sessionTabs.find((val) => val.index === index),
  setSessionLink: (index: string, sessionLink: SessionLink) =>
    set((state) => ({
      sessionTabs: state.sessionTabs.map((tab) =>
        tab.index === index ? { ...tab, sessionLink } : tab,
      ),
    })),
  reorderTabs: (sourceIndex: number, destinationIndex: number) =>
    set((state) => {
      const newTabs = [...state.sessionTabs];
      const [removed] = newTabs.splice(sourceIndex, 1);
      newTabs.splice(destinationIndex, 0, removed);
      return { sessionTabs: newTabs };
    }),
  setSessionConnectionStatus: (linkID: string, status: ConnectionStatus) =>
    set((state) => ({
      sessionTabs: state.sessionTabs.map((tab) => {
        if (tab.sessionLink?.linkID !== linkID) return tab;
        return tabWithSessionConnectionStatus(tab, status) ?? tab;
      }),
    })),
  setTabConnectionStatus: (tabIndex: string, status: ConnectionStatus) =>
    set((state) => ({
      sessionTabs: state.sessionTabs.map((tab) =>
        tab.index === tabIndex ? tabWithConnectionStatus(tab, status) : tab,
      ),
    })),
}));

export interface ReloadSessionTabStoreType {
  reloadTab: {
    index: string;
    count: number;
  };
  doTabReload: (index: string) => void;
}

export const useReloadSessionTabStore = create<ReloadSessionTabStoreType>(
  (set, get) => ({
    reloadTab: {
      index: "",
      count: 0,
    },
    doTabReload: (index: string) => {
      set({
        reloadTab: { index, count: get().reloadTab.count + 1 },
      });
    },
  }),
);

