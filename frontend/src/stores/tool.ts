import { create } from "zustand";
import { Tool } from "../types/tool";
import { ToolService } from "../../bindings/github.com/ilaziness/vela/services";

interface ToolStore {
  tools: Tool[];
  isLoading: boolean;
  error: string | null;
  fetchTools: () => Promise<void>;
  getToolById: (id: string) => Tool | undefined;
}

export const useToolStore = create<ToolStore>((set, get) => ({
  tools: [],
  isLoading: false,
  error: null,

  fetchTools: async () => {
    set({ isLoading: true, error: null });
    try {
      const tools = await ToolService.GetTools();
      set({ tools: tools || [], isLoading: false });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : "Failed to fetch tools",
        isLoading: false,
      });
    }
  },

  getToolById: (id: string) => {
    return get().tools.find((tool) => tool.id === id);
  },
}));
