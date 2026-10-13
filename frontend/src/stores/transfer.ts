import { create } from "zustand";
import { ProgressData } from "../../bindings/github.com/ilaziness/vela/services/models";

export interface TransferStore {
  transfers: Map<string, ProgressData[]>; // sessionID -> ProgressData[]
  addProgress: (progress: ProgressData) => void;
  removeProgress: (sessionID: string, id: string) => void;
  clearSession: (sessionID: string) => void;
  clearCompletedTransfers: (sessionID: string) => void;
}

function hasError(p: ProgressData): boolean {
  return Boolean(p.error && p.error.trim() !== "");
}

export const useTransferStore = create<TransferStore>((set) => ({
  transfers: new Map(),
  addProgress: (progress: ProgressData) => {
    set((state) => {
      const newTransfers = new Map(state.transfers);
      const sessionID = progress.sessionID;
      if (!newTransfers.has(sessionID)) {
        newTransfers.set(sessionID, []);
      }
      const sessionTransfers = newTransfers.get(sessionID)!;
      const existingIndex = sessionTransfers.findIndex((p) => p.id === progress.id);
      if (existingIndex >= 0) {
        const updatedTransfers = [...sessionTransfers];
        updatedTransfers[existingIndex] = progress;
        newTransfers.set(sessionID, updatedTransfers);
      } else {
        newTransfers.set(sessionID, [...sessionTransfers, progress]);
      }
      return { transfers: newTransfers };
    });
  },
  removeProgress: (sessionID: string, id: string) => {
    set((state) => {
      const newTransfers = new Map(state.transfers);
      const sessionTransfers = newTransfers.get(sessionID) || [];
      const filtered = sessionTransfers.filter((p) => p.id !== id);
      newTransfers.set(sessionID, filtered);
      return { transfers: newTransfers };
    });
  },
  clearSession: (sessionID: string) => {
    set((state) => {
      const newTransfers = new Map(state.transfers);
      newTransfers.delete(sessionID);
      return { transfers: newTransfers };
    });
  },
  clearCompletedTransfers: (sessionID: string) => {
    set((state) => {
      const newTransfers = new Map(state.transfers);
      const sessionTransfers = newTransfers.get(sessionID) || [];
      // Keep active and failed; only drop successful completions.
      const kept = sessionTransfers.filter((p) => !p.done || hasError(p));
      if (kept.length === 0) {
        newTransfers.delete(sessionID);
      } else {
        newTransfers.set(sessionID, kept);
      }
      return { transfers: newTransfers };
    });
  },
}));
