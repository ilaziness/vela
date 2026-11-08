import { useSSHTabsStore } from "../stores/ssh";
import { terminalInstances } from "../stores/terminalInstances";
import {
  CurrentSSHBindStatus,
  resolveSSHBindState,
} from "./aiContext";

/** Must match @xterm/addon-webgl WebglRenderer context attributes. */
const WEBGL_ATTRS: WebGLContextAttributes = {
  antialias: false,
  depth: false,
  preserveDrawingBuffer: true,
};

function webglCellCanvas(screen: Element): HTMLCanvasElement | undefined {
  for (const node of screen.children) {
    if (!(node instanceof HTMLCanvasElement)) continue;
    if (node.className.includes("-layer")) continue;
    if (node.getContext("webgl2", WEBGL_ATTRS)) return node;
  }
  return undefined;
}

/** PNG of the current connected tab's WebGL2 terminal, or null if not capturable. */
export async function captureCurrentTerminalPng(): Promise<File | null> {
  const { currentTab, getByIndex } = useSSHTabsStore.getState();
  const tab = getByIndex(currentTab);
  const linkId = tab?.sshInfo?.linkID;
  if (
    resolveSSHBindState(tab).status !== CurrentSSHBindStatus.Connected ||
    !linkId
  ) {
    return null;
  }

  const screen = terminalInstances.get(linkId)?.element?.querySelector(".xterm-screen");
  if (!screen) return null;

  const canvas = webglCellCanvas(screen);
  if (!canvas) return null;

  const blob = await new Promise<Blob | null>((resolve) => {
    canvas.toBlob(resolve, "image/png");
  });
  if (!blob) return null;
  return new File([blob], "terminal.png", { type: "image/png" });
}
