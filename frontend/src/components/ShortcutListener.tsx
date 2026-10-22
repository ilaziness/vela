import { useEffect } from "react";
import { Events } from "@wailsio/runtime";
import {
  EVENT_SHORTCUT,
  ShortcutAction,
  isEditableFocus,
  parseShortcutAction,
} from "../func/shortcuts";
import { useAIAssistantStore } from "../stores/aiAssistant";
import { useUIStore } from "../stores/ui";
import { useKeybindingsStore } from "../stores/keybindings";

/** 主窗 / 子主窗共用：处理面板类快捷键并加载快捷键说明。 */
export default function ShortcutListener() {
  useEffect(() => {
    void useKeybindingsStore.getState().load().catch(() => {});

    const unsubscribe = Events.On(
      EVENT_SHORTCUT,
      (event: { data?: unknown }) => {
        const action = parseShortcutAction(event.data);
        // 书签连接面板：搜索框聚焦时仍应能用快捷键关闭。
        if (action === ShortcutAction.PanelBookmarkConnect) {
          useUIStore.getState().toggleBookmarkConnect();
          return;
        }
        if (isEditableFocus()) return;
        if (action === ShortcutAction.PanelAI) {
          useAIAssistantStore.getState().toggleSidebarOpen();
          return;
        }
        if (action === ShortcutAction.PanelBookmarks) {
          useUIStore.getState().toggleBookmarkManage();
        }
      },
    );
    return () => {
      unsubscribe();
    };
  }, []);

  return null;
}
