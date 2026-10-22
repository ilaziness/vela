import { create } from "zustand";
import { Events } from "@wailsio/runtime";
import {
  AppService,
  KeyBindingInfo,
  LogService,
} from "../../bindings/github.com/ilaziness/vela/services";
import { parseCallServiceError } from "../func/service";

const EVENT_KEYBINDINGS = "eventKeybindings";

interface KeybindingsState {
  bindings: KeyBindingInfo[];
  loaded: boolean;
  load: (force?: boolean) => Promise<void>;
  reload: () => Promise<void>;
}

let eventBound = false;

function ensureEventListener(reload: () => Promise<void>) {
  if (eventBound) {
    return;
  }
  eventBound = true;
  Events.On(EVENT_KEYBINDINGS, () => {
    void reload();
  });
}

export const useKeybindingsStore = create<KeybindingsState>((set, get) => ({
  bindings: [],
  loaded: false,

  load: async (force = false) => {
    ensureEventListener(() => get().reload());
    if (get().loaded && !force) {
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
      throw err;
    }
  },

  reload: async () => {
    await get().load(true);
  },
}));
