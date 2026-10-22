import { create } from "zustand";

interface UIState {
  bookmarkManageOpen: boolean;
  setBookmarkManageOpen: (open: boolean) => void;
  toggleBookmarkManage: () => void;

  bookmarkConnectOpen: boolean;
  setBookmarkConnectOpen: (open: boolean) => void;
  toggleBookmarkConnect: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  bookmarkManageOpen: false,
  setBookmarkManageOpen: (open) => set({ bookmarkManageOpen: open }),
  toggleBookmarkManage: () =>
    set((state) => ({ bookmarkManageOpen: !state.bookmarkManageOpen })),

  bookmarkConnectOpen: false,
  setBookmarkConnectOpen: (open) => set({ bookmarkConnectOpen: open }),
  toggleBookmarkConnect: () =>
    set((state) => ({ bookmarkConnectOpen: !state.bookmarkConnectOpen })),
}));
