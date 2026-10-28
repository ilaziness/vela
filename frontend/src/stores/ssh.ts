import { create } from "zustand";
import { SSHLinkInfo, SSHTab, ConnectionStatus } from "../types/ssh";
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

export interface SSHTabs {
  sshTabs: SSHTab[];
  currentTab: string;
  pushTab: (tab: SSHTab) => void;
  delTab: (delTab: string, currentTab: string) => string; // 返回需要激活的标签
  setName: (index: string, name: string) => void;
  setCurrentTab: (index: string) => void;
  getByIndex: (index: string) => SSHTab | undefined;
  setSSHInfo: (index: string, sshInfo: SSHLinkInfo) => void;
  reorderTabs: (sourceIndex: number, destinationIndex: number) => void;
  /**
   * Terminal / WebSocket path. Matches by session id; does not clear linkID.
   * No-op if no tab owns the id, or Disconnected while tab is Connecting.
   */
  setSessionConnectionStatus: (
    linkID: string,
    status: ConnectionStatus,
  ) => void;
  /**
   * Tab lifecycle (connect / fail / clean exit). Disconnected clears linkID.
   */
  setTabConnectionStatus: (
    tabIndex: string,
    status: ConnectionStatus,
  ) => void;
}

const initialTabIndex = genTabIndex();

export const useSSHTabsStore = create<SSHTabs>((set, get) => ({
  sshTabs: [
    {
      index: initialTabIndex,
      name: NEW_CONNECTION_TAB_ID,
    },
  ] as SSHTab[],
  currentTab: initialTabIndex,
  pushTab: (tab: SSHTab) =>
    set((state: any) => {
      if (tab.sshInfo) {
        const emptyIndex = state.sshTabs.findIndex((t: SSHTab) => !t.sshInfo);
        if (emptyIndex !== -1) {
          const newTabs = [...state.sshTabs];
          const replacedIndex = newTabs[emptyIndex].index;
          newTabs[emptyIndex] = tab;
          const newCurrentTab =
            state.currentTab === replacedIndex ? tab.index : state.currentTab;
          return { sshTabs: newTabs, currentTab: newCurrentTab };
        }
      }
      return { sshTabs: [...state.sshTabs, tab] };
    }),
  delTab: (delTab: string, currentTab: string): string => {
    const tabIndex = get().sshTabs.findIndex((item) => item.index === delTab);
    if (tabIndex === -1) {
      return "0";
    }
    // 删除指定索引的标签
    let newSshTabs = get().sshTabs.filter((item) => item.index !== delTab);

    // 如果删除的是当前激活的标签，需要切换到另一个标签
    let newTabValue = currentTab;
    if (currentTab === delTab) {
      const lastTab = newSshTabs.at(-1);
      if (lastTab) {
        newTabValue = lastTab.index;
      } else {
        newSshTabs = [
          {
            index: `${genTabIndex()}`,
            name: NEW_CONNECTION_TAB_ID,
          },
        ];
        newTabValue = newSshTabs[0].index;
      }
    }
    set({ sshTabs: newSshTabs, currentTab: newTabValue });
    return newTabValue;
  },
  setName: (index: string, name: string) =>
    set((state) => ({
      sshTabs: state.sshTabs.map((tab) =>
        tab.index === index && isNewConnectionTabName(tab.name)
          ? { ...tab, name }
          : tab,
      ),
    })),
  setCurrentTab: (index: string) => set({ currentTab: index }),
  getByIndex: (index: string): SSHTab | undefined =>
    get().sshTabs.find((val) => {
      return val.index === index;
    }),
  setSSHInfo: (index: string, sshInfo: SSHLinkInfo) =>
    set((state) => ({
      sshTabs: state.sshTabs.map((tab) =>
        tab.index === index ? { ...tab, sshInfo } : tab,
      ),
    })),
  reorderTabs: (sourceIndex: number, destinationIndex: number) =>
    set((state) => {
      const newTabs = [...state.sshTabs];
      const [removed] = newTabs.splice(sourceIndex, 1);
      newTabs.splice(destinationIndex, 0, removed);
      return { sshTabs: newTabs };
    }),
  setSessionConnectionStatus: (linkID: string, status: ConnectionStatus) =>
    set((state) => ({
      sshTabs: state.sshTabs.map((tab) => {
        if (tab.sshInfo?.linkID !== linkID) return tab;
        return tabWithSessionConnectionStatus(tab, status) ?? tab;
      }),
    })),
  setTabConnectionStatus: (tabIndex: string, status: ConnectionStatus) =>
    set((state) => ({
      sshTabs: state.sshTabs.map((tab) =>
        tab.index === tabIndex ? tabWithConnectionStatus(tab, status) : tab,
      ),
    })),
}));

export interface ReloadSSHTabStoreType {
  reloadTab: {
    index: string;
    count: number;
  };
  doTabReload: (index: string) => void;
}

export const useReloadSSHTabStore = create<ReloadSSHTabStoreType>(
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
