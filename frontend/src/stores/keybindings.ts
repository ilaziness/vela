import { create } from "zustand";
import {
  AppService,
  KeyBindingInfo,
  LogService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { parseCallServiceError } from "../func/service";

interface KeybindingsState {
  bindings: KeyBindingInfo[];
  loaded: boolean;
  load: () => Promise<void>;
}

export const useKeybindingsStore = create<KeybindingsState>((set, get) => ({
  bindings: [],
  loaded: false,

  load: async () => {
    if (get().loaded) {
      return;
    }
    try {
      const list = await AppService.ListKeyBindings();
      set({
        bindings: Array.isArray(list) ? list.filter(Boolean) : [],
        loaded: true,
      });
    } catch (err) {
      LogService.Error(
        "Failed to load keybindings: " + parseCallServiceError(err),
      );
    }
  },
}));
