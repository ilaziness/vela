import React, { useEffect, useEffectEvent, useState, memo } from "react";
import { Box } from "@mui/material";
import type { Terminal as TerminalLib } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import type { WebglAddon } from "@xterm/addon-webgl";
import type { SearchAddon } from "@xterm/addon-search";
import { Browser, Events } from "@wailsio/runtime";
import {
  LogService,
  SSHService,
  ConfigService,
  AppService,
  CommandService,
} from "../../bindings/github.com/ilaziness/vexo/services";
import useTerminalStore from "../stores/terminal";
import Loading from "./Loading";
import TerminalContextMenu from "./TerminalContextMenu";
import TerminalSearchBar from "./TerminalSearchBar";
import TerminalCommandHints, {
  TERMINAL_HINTS_MAX,
  type HintAnchor,
} from "./TerminalCommandHints";
import { terminalInstances } from "../stores/terminalInstances";
import { parseCallServiceError, sleep } from "../func/service";
import { applyShellInput, matchCommandHints } from "../func/shellLineBuf";
import { useSSHTabsStore } from "../stores/ssh";
import { useMessageStore } from "../stores/message";
import { ConnectionStatus } from "../types/ssh";
import { useAIAssistantStore } from "../stores/aiAssistant";
import {
  EVENT_SHORTCUT,
  ShortcutAction,
  isEditableFocus,
  parseShortcutAction,
  toShellCodeBlock,
} from "../func/shortcuts";
import type { CommandHistory } from "../types/command";

const EVENT_SESSION_LOG_STOPPED = "eventSSHSessionLogStopped";

type XtermModules = {
  Terminal: typeof import("@xterm/xterm").Terminal;
  FitAddon: typeof import("@xterm/addon-fit").FitAddon;
  WebglAddon: typeof import("@xterm/addon-webgl").WebglAddon;
  WebLinksAddon: typeof import("@xterm/addon-web-links").WebLinksAddon;
  Unicode11Addon: typeof import("@xterm/addon-unicode11").Unicode11Addon;
  ImageAddon: typeof import("@xterm/addon-image").ImageAddon;
  LigaturesAddon: typeof import("@xterm/addon-ligatures").LigaturesAddon;
  ClipboardAddon: typeof import("@xterm/addon-clipboard").ClipboardAddon;
  SearchAddon: typeof import("@xterm/addon-search").SearchAddon;
  AttachAddon: typeof import("@xterm/addon-attach").AttachAddon;
};

let xtermModulesPromise: Promise<XtermModules> | null = null;

function loadXtermModules(): Promise<XtermModules> {
  if (!xtermModulesPromise) {
    xtermModulesPromise = Promise.all([
      import("@xterm/xterm"),
      import("@xterm/addon-fit"),
      import("@xterm/addon-webgl"),
      import("@xterm/addon-web-links"),
      import("@xterm/addon-unicode11"),
      import("@xterm/addon-image"),
      import("@xterm/addon-ligatures"),
      import("@xterm/addon-clipboard"),
      import("@xterm/addon-search"),
      import("@xterm/addon-attach"),
      import("@xterm/xterm/css/xterm.css"),
    ])
      .then(
        ([
          xterm,
          fit,
          webgl,
          webLinks,
          unicode11,
          image,
          ligatures,
          clipboard,
          search,
          attach,
        ]) => ({
          Terminal: xterm.Terminal,
          FitAddon: fit.FitAddon,
          WebglAddon: webgl.WebglAddon,
          WebLinksAddon: webLinks.WebLinksAddon,
          Unicode11Addon: unicode11.Unicode11Addon,
          ImageAddon: image.ImageAddon,
          LigaturesAddon: ligatures.LigaturesAddon,
          ClipboardAddon: clipboard.ClipboardAddon,
          SearchAddon: search.SearchAddon,
          AttachAddon: attach.AttachAddon,
        }),
      )
      .catch((err) => {
        // Allow subsequent mounts to retry after a failed dynamic import.
        xtermModulesPromise = null;
        throw err;
      });
  }
  return xtermModulesPromise;
}

const isWebgl2Supported = (() => {
  let isSupported = globalThis.WebGL2RenderingContext ? undefined : false;
  return () => {
    if (isSupported === undefined) {
      const canvas = document.createElement("canvas");
      const gl = canvas.getContext("webgl2", {
        depth: false,
        antialias: false,
      });
      isSupported = gl instanceof globalThis.WebGL2RenderingContext;
    }
    return isSupported;
  };
})();

// Terminal 组件，封装 xterm.js
function Terminal(props: {
  readonly linkID: string;
  readonly isActive: boolean;
  readonly onLoggingChange?: (logging: boolean) => void;
  readonly onReady?: () => void;
}) {
  const [isInitializing, setIsInitializing] = useState(true);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchFocusNonce, setSearchFocusNonce] = useState(0);
  const [searchAddon, setSearchAddon] = useState<SearchAddon | null>(null);
  const [logging, setLogging] = useState(false);
  const [hintItems, setHintItems] = useState<string[]>([]);
  const [hintActiveIndex, setHintActiveIndex] = useState(0);
  const [hintAnchor, setHintAnchor] = useState<HintAnchor | null>(null);
  const termRef = React.useRef<HTMLDivElement>(null);
  const term = React.useRef<TerminalLib>(null);
  const termFit = React.useRef<FitAddon>(null);
  const webglRef = React.useRef<WebglAddon>(null);
  const resizeTimeout = React.useRef<number | NodeJS.Timeout | null>(null);
  const wsRef = React.useRef<WebSocket>(null);
  const isActiveRef = React.useRef(props.isActive);
  isActiveRef.current = props.isActive;
  const linkIDRef = React.useRef(props.linkID);
  linkIDRef.current = props.linkID;
  const loggingRef = React.useRef(logging);
  loggingRef.current = logging;
  const lineBufRef = React.useRef("");
  const historyCacheRef = React.useRef<string[]>([]);
  const hintItemsRef = React.useRef<string[]>([]);
  const hintActiveIndexRef = React.useRef(0);
  const acceptingHintRef = React.useRef(false);
  const historyRefreshTimerRef = React.useRef<number | NodeJS.Timeout | null>(
    null,
  );
  const [contextMenu, setContextMenu] = useState<{
    mouseX: number;
    mouseY: number;
  } | null>(null);
  const setConnectionStatus = useSSHTabsStore(
    (state) => state.setConnectionStatus,
  );
  const { errorMessage, successMessage } = useMessageStore();
  const onLoggingChangeRef = React.useRef(props.onLoggingChange);
  onLoggingChangeRef.current = props.onLoggingChange;
  const onReadyRef = React.useRef(props.onReady);
  onReadyRef.current = props.onReady;

  const setLoggingState = (next: boolean) => {
    setLogging(next);
    onLoggingChangeRef.current?.(next);
  };

  const openSearch = () => {
    setSearchOpen(true);
    setSearchFocusNonce((n) => n + 1);
  };

  const closeSearch = () => {
    setSearchOpen(false);
    searchAddon?.clearDecorations();
    term.current?.focus();
  };

  const readCursorAnchor = (): HintAnchor | null => {
    const t = term.current;
    const host = termRef.current?.parentElement;
    if (!t?.element || !host) {
      return null;
    }
    try {
      const core = t as unknown as {
        _core?: {
          _renderService?: {
            dimensions?: { css?: { cell?: { width: number; height: number } } };
          };
        };
      };
      const cell = core._core?._renderService?.dimensions?.css?.cell;
      if (!cell?.width || !cell?.height) {
        return null;
      }
      const { cursorX, cursorY } = t.buffer.active;
      const termRect = t.element.getBoundingClientRect();
      const hostRect = host.getBoundingClientRect();
      return {
        left: termRect.left - hostRect.left + cursorX * cell.width,
        top: termRect.top - hostRect.top + (cursorY + 1) * cell.height,
        cellHeight: cell.height,
      };
    } catch {
      return null;
    }
  };

  const clearHints = () => {
    hintItemsRef.current = [];
    hintActiveIndexRef.current = 0;
    setHintItems([]);
    setHintActiveIndex(0);
    setHintAnchor(null);
  };

  const updateHints = (prefix: string) => {
    const items = matchCommandHints(
      historyCacheRef.current,
      prefix,
      TERMINAL_HINTS_MAX,
    );
    const anchor = items.length > 0 ? readCursorAnchor() : null;
    // No anchor → hide list and do not capture keys.
    if (items.length === 0 || !anchor) {
      clearHints();
      return;
    }
    // Prefer first completable (longer) item; exact match stays visible above.
    const active = items[0] === prefix ? 1 : 0;
    hintItemsRef.current = items;
    hintActiveIndexRef.current = active;
    setHintItems(items);
    setHintActiveIndex(active);
    setHintAnchor(anchor);
  };

  const acceptHint = (command: string) => {
    const prefix = lineBufRef.current;
    if (!command.startsWith(prefix)) {
      return;
    }
    // Exact match: already typed; just dismiss the list.
    if (command === prefix) {
      clearHints();
      term.current?.focus();
      return;
    }
    const suffix = command.slice(prefix.length);
    acceptingHintRef.current = true;
    term.current?.paste(suffix);
    acceptingHintRef.current = false;
    lineBufRef.current = command;
    updateHints(command);
    term.current?.focus();
  };

  const loadHistoryCache = async () => {
    try {
      const rows = (await CommandService.GetCommandHistory()) as CommandHistory[];
      historyCacheRef.current = rows.map((r) => r.command).filter(Boolean);
    } catch (err) {
      LogService.Error(
        `load command history for hints failed: ${parseCallServiceError(err)}`,
      );
    }
  };

  const handleContextMenu = (event: React.MouseEvent) => {
    event.preventDefault();
    setContextMenu(
      contextMenu === null
        ? {
            mouseX: event.clientX + 2,
            mouseY: event.clientY - 6,
          }
        : null,
    );
  };

  const handleClose = () => {
    setContextMenu(null);
  };

  const handleStartLogging = async () => {
    if (!props.linkID) {
      return;
    }
    try {
      const [path, started] = await SSHService.StartSessionLog(props.linkID);
      if (!started) {
        return;
      }
      setLoggingState(true);
      successMessage(`会话日志已开始记录：${path}`);
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const handleStopLogging = async () => {
    if (!props.linkID) {
      setLoggingState(false);
      return;
    }
    try {
      await SSHService.StopSessionLog(props.linkID);
      setLoggingState(false);
      successMessage("会话日志已停止记录");
    } catch (err) {
      errorMessage(parseCallServiceError(err));
    }
  };

  const handleTerminalShortcut = useEffectEvent((action: string) => {
    if (isEditableFocus()) return;

    const id = linkIDRef.current;
    const instance = terminalInstances.get(id);

    switch (action) {
      case ShortcutAction.TerminalFind:
        openSearch();
        break;
      case ShortcutAction.TerminalCopy: {
        const selection = instance?.getSelection() ?? "";
        if (selection) {
          void navigator.clipboard.writeText(selection);
        }
        break;
      }
      case ShortcutAction.TerminalPaste:
        void (async () => {
          try {
            const text = await navigator.clipboard.readText();
            if (!text) {
              return;
            }
            const term = terminalInstances.get(linkIDRef.current);
            term?.paste(text);
            term?.focus();
          } catch (err) {
            console.error("Failed to read clipboard contents: ", err);
          }
        })();
        break;
      case ShortcutAction.TerminalAddToChat: {
        const selection = instance?.getSelection() ?? "";
        if (selection) {
          useAIAssistantStore
            .getState()
            .appendToComposer(toShellCodeBlock(selection));
        }
        break;
      }
      case ShortcutAction.TerminalToggleLog:
        if (loggingRef.current) {
          void handleStopLogging();
        } else {
          void handleStartLogging();
        }
        break;
      case ShortcutAction.TerminalClear:
        instance?.clear();
        break;
      default:
        break;
    }
  });

  useEffect(() => {
    setLoggingState(false);
  }, [props.linkID]);

  useEffect(() => {
    const unsubLog = Events.On(EVENT_SESSION_LOG_STOPPED, (event: any) => {
      try {
        const raw =
          typeof event.data === "string" ? JSON.parse(event.data) : event.data;
        if (raw?.id && raw.id === linkIDRef.current) {
          setLoggingState(false);
          if (raw.reason === "write_error") {
            useMessageStore
              .getState()
              .errorMessage("会话日志写入失败，已停止记录");
          }
        }
      } catch (e) {
        console.error("Invalid session log stopped payload", e);
      }
    });
    return () => {
      unsubLog();
    };
  }, []);

  const applyThemeVars = (theme: any) => {
    if (termRef.current) {
      termRef.current.style.setProperty("--term-bg", theme.background);
      termRef.current.style.setProperty("--term-fg", theme.foreground);
    }
  };

  const loadAddon = (mods: XtermModules) => {
    LogService.Debug("loadAddon");
    termFit.current = new mods.FitAddon();
    term.current?.loadAddon(termFit.current);
    term.current?.loadAddon(
      new mods.WebLinksAddon((event, uri) => {
        event.preventDefault();
        Browser.OpenURL(uri);
      }),
    );
    const addon = new mods.SearchAddon();
    term.current?.loadAddon(addon);
    setSearchAddon(addon);
    term.current?.loadAddon(new mods.Unicode11Addon());
    term.current && (term.current.unicode.activeVersion = "11");
    term.current?.loadAddon(new mods.ImageAddon());
    term.current?.loadAddon(new mods.ClipboardAddon());

    if (isWebgl2Supported()) {
      const webglAddon = new mods.WebglAddon();
      webglAddon.onContextLoss(() => {
        console.warn("WebGL context lost. Falling back to DOM rendering.");
        webglAddon.dispose();
      });
      webglRef.current = webglAddon;
      term.current?.loadAddon(webglAddon);
    } else {
      term.current?.loadAddon(new mods.LigaturesAddon());
    }
  };

  const onResize = ({ cols, rows }) => {
    LogService.Debug(`Terminal resized to ${cols}x${rows}`);
    SSHService.Resize(props.linkID, cols, rows);
  };

  const initTerminal = useEffectEvent(
    async (mountedRef: { current: boolean }) => {
      if (term.current) {
        return;
      }
      LogService.Debug("Initializing terminal for link ID: " + props.linkID);
      try {
        const [config, mods] = await Promise.all([
          ConfigService.ReadConfig(),
          loadXtermModules(),
        ]);
        if (!mountedRef.current) return;
        const settings = config?.Terminal || useTerminalStore.getState();
        LogService.Debug(`Terminal setting ${JSON.stringify(settings)}`);
        if (term.current) return;
        if (!termRef.current) {
          LogService.Error(
            `Terminal DOM node missing for link ID: ${props.linkID}`,
          );
          setIsInitializing(false);
          return;
        }
        // 获取当前终端主题
        const terminalTheme = useTerminalStore.getState().getCurrentTheme();
        applyThemeVars(terminalTheme);
        term.current = new mods.Terminal({
          allowProposedApi: true,
          cursorBlink: true,
          cursorStyle: "block",
          fontFamily: settings.fontFamily,
          fontSize: settings.fontSize,
          lineHeight: settings.lineHeight,
          rightClickSelectsWord: true,
          theme: terminalTheme,
        });
        loadAddon(mods);
        term.current.open(termRef.current);
        terminalInstances.set(props.linkID, term.current);
        await sleep(50);
        if (!mountedRef.current) return;
        termFit.current?.fit();

        const cols = term.current?.cols || 80;
        const rows = term.current?.rows || 24;
        const wsAddr = await AppService.GetWSAddr();
        if (!mountedRef.current) return;
        const wsUrl = `ws://${wsAddr}/ws/terminal?id=${props.linkID}&cols=${cols}&rows=${rows}`;
        LogService.Debug(
          `Connecting to WebSocket at ${wsUrl} for terminal ${props.linkID}`,
        );
        const ws = new WebSocket(wsUrl);
        ws.binaryType = "arraybuffer";
        wsRef.current = ws;
        const attachAddon = new mods.AttachAddon(ws);
        term.current?.loadAddon(attachAddon);

        lineBufRef.current = "";
        clearHints();
        void loadHistoryCache();

        term.current.attachCustomKeyEventHandler((ev) => {
          if (ev.type !== "keydown") {
            return true;
          }
          const items = hintItemsRef.current;
          if (items.length === 0) {
            return true;
          }
          if (ev.key === "ArrowUp") {
            ev.preventDefault();
            setHintActiveIndex((i) => {
              const next = i <= 0 ? items.length - 1 : i - 1;
              hintActiveIndexRef.current = next;
              return next;
            });
            return false;
          }
          if (ev.key === "ArrowDown") {
            ev.preventDefault();
            setHintActiveIndex((i) => {
              const next = i >= items.length - 1 ? 0 : i + 1;
              hintActiveIndexRef.current = next;
              return next;
            });
            return false;
          }
          if (ev.key === "Escape") {
            ev.preventDefault();
            clearHints();
            return false;
          }
          if (ev.key === "Tab" || ev.key === "Enter") {
            const selected = items[hintActiveIndexRef.current];
            if (selected) {
              // Select into terminal only (exact = dismiss; longer = paste suffix).
              // Never send here — user presses Enter again to run.
              ev.preventDefault();
              acceptHint(selected);
              return false;
            }
          }
          return true;
        });

        term.current.onData((data) => {
          if (acceptingHintRef.current) {
            // paste during accept already applied to lineBuf
            return;
          }
          const { buf, submitted } = applyShellInput(lineBufRef.current, data);
          lineBufRef.current = buf;
          if (submitted) {
            // Refresh from backend so secrets skipped server-side never enter the hint cache.
            clearHints();
            if (historyRefreshTimerRef.current) {
              clearTimeout(historyRefreshTimerRef.current);
            }
            historyRefreshTimerRef.current = globalThis.setTimeout(() => {
              historyRefreshTimerRef.current = null;
              if (mountedRef.current) {
                void loadHistoryCache();
              }
            }, 300);
            return;
          }
          updateHints(buf);
        });

        ws.onopen = async () => {
          if (!mountedRef.current) return;
          LogService.Debug("WebSocket connected for terminal " + props.linkID);
          setIsInitializing(false);
          setConnectionStatus(props.linkID, ConnectionStatus.Connected);
          onReadyRef.current?.();

          term.current?.focus();
          term.current?.onResize(onResize);
          sleep(1000).then(() => {
            if (!mountedRef.current) return;
            LogService.Debug("Fitting terminal after WebSocket connection");
            termFit.current?.fit();
          });
        };

        ws.onerror = (error) => {
          if (!mountedRef.current) return;
          // WebSocket error event is an Event object, use message property if available
          const errorMessage =
            "message" in error
              ? (error as any).message
              : "Unknown WebSocket error";
          LogService.Error(
            `WebSocket error for terminal ${props.linkID}: ${errorMessage}`,
          );
          term.current?.write(
            `\r\n*** WebSocket error: ${errorMessage} ***\r\n`,
          );
          setIsInitializing(false);
        };

        ws.onclose = () => {
          if (!mountedRef.current) return;
          LogService.Debug(`WebSocket closed for terminal ${props.linkID}`);
          setConnectionStatus(props.linkID, ConnectionStatus.Disconnected);
          setIsInitializing(false);
          term.current?.write(`\r\n*** SSH connection closed ***\r\n`);
        };
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        LogService.Error(
          `Failed to initialize terminal ${props.linkID}: ${message}`,
        );
        if (!mountedRef.current) return;
        if (term.current) {
          term.current.write(
            `\r\n*** Failed to initialize terminal: ${message} ***\r\n`,
          );
        }
        setIsInitializing(false);
        setConnectionStatus(props.linkID, ConnectionStatus.Disconnected);
      }
    },
  );

  // 监听终端主题变化，动态更新终端主题
  useEffect(() => {
    const unsubscribe = useTerminalStore.subscribe((state, prevState) => {
      if (state.theme !== prevState.theme && term.current) {
        const newTheme = state.getCurrentTheme();
        term.current.options.theme = newTheme;
        applyThemeVars(newTheme);
        term.current.refresh(0, term.current.rows - 1);
        LogService.Debug(`Terminal theme updated to: ${state.theme}`);
      }
    });

    return unsubscribe;
  }, []);

  useEffect(() => {
    const unsubscribe = Events.On(
      EVENT_SHORTCUT,
      (event: { data?: unknown }) => {
        if (!isActiveRef.current) return;
        const action = parseShortcutAction(event.data);
        if (!action || !action.startsWith("terminal.")) return;
        handleTerminalShortcut(action);
      },
    );
    return () => {
      unsubscribe();
    };
  }, []);

  // Wails AcceleratorKeyPressed 在 PutHandled(true) 后仍会 PutHandled(false)，
  // Ctrl+Shift+V 会同时走我们的粘贴与浏览器默认粘贴，导致重复并残留选区高亮。
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!isActiveRef.current || isEditableFocus()) return;
      if (!(e.ctrlKey || e.metaKey) || !e.shiftKey) return;
      if (e.key.toLowerCase() !== "v") return;
      e.preventDefault();
      e.stopPropagation();
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
    };
  }, []);

  useEffect(() => {
    const mountedRef = { current: true };

    initTerminal(mountedRef).then(() => {
      if (!mountedRef.current) return;
    });

    const observer = new ResizeObserver(() => {
      if (resizeTimeout.current) {
        clearTimeout(resizeTimeout.current);
      }
      resizeTimeout.current = globalThis.setTimeout(() => {
        termFit.current?.fit();
      }, 100);
    });
    if (termRef.current) {
      observer.observe(termRef.current);
    }

    return () => {
      mountedRef.current = false;
      LogService.Debug(`Terminal component unmounting ${props.linkID}`);
      terminalInstances.remove(props.linkID);
      setSearchOpen(false);
      setSearchAddon(null);
      clearHints();
      lineBufRef.current = "";
      if (historyRefreshTimerRef.current) {
        clearTimeout(historyRefreshTimerRef.current);
        historyRefreshTimerRef.current = null;
      }
      try {
        wsRef.current?.close();
        webglRef.current?.dispose();
        term.current?.dispose();
        term.current = null;
      } catch (e) {
        console.error("Error disposing terminal:", e);
      }
      if (resizeTimeout.current) {
        clearTimeout(resizeTimeout.current);
      }
      observer.disconnect();
    };
  }, [props.linkID]);

  return (
    <Box
      sx={{
        width: "100%",
        height: "100%",
        display: "flex",
        position: "relative",
      }}
    >
      <Box
        ref={termRef}
        onContextMenu={handleContextMenu}
        sx={{
          flex: 1,
          width: "100%",
          height: "100%",
          backgroundColor: "var(--term-bg)",
          paddingLeft: "3px",
        }}
      />
      <TerminalSearchBar
        open={searchOpen}
        searchAddon={searchAddon}
        focusNonce={searchFocusNonce}
        onClose={closeSearch}
      />
      <TerminalCommandHints
        items={hintItems}
        activeIndex={hintActiveIndex}
        anchor={hintAnchor}
        onSelect={acceptHint}
        onHover={(index) => {
          hintActiveIndexRef.current = index;
          setHintActiveIndex(index);
        }}
      />
      <TerminalContextMenu
        contextMenu={contextMenu}
        onClose={handleClose}
        linkID={props.linkID}
        onFind={openSearch}
        logging={logging}
        onStartLogging={() => {
          void handleStartLogging();
        }}
        onStopLogging={() => {
          void handleStopLogging();
        }}
      />
      {isInitializing && (
        <Box
          sx={{
            width: "100%",
            height: "100%",
            position: "absolute",
            top: 0,
            left: 0,
          }}
        >
          <Loading message="Initializing terminal..." />
        </Box>
      )}
    </Box>
  );
}

export default memo(Terminal);
