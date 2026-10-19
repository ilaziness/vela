export enum PaneSide {
  Local = "local",
  Remote = "remote",
}

export const sftpDragMime = "application/x-vela-sftp";

export interface PaneDragData {
  side: PaneSide;
  paths: string[];
}

export function dragStartsOnControl(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    !!target.closest(".MuiCheckbox-root, input, button, a")
  );
}

export function writePaneDrag(
  dataTransfer: DataTransfer,
  data: PaneDragData,
): void {
  dataTransfer.setData(sftpDragMime, JSON.stringify(data));
  dataTransfer.effectAllowed = "copy";
}

export function readPaneDrag(dataTransfer: DataTransfer): PaneDragData | null {
  const raw = dataTransfer.getData(sftpDragMime);
  if (!raw) return null;
  try {
    const data = JSON.parse(raw) as PaneDragData;
    if (
      !data ||
      (data.side !== PaneSide.Local && data.side !== PaneSide.Remote) ||
      !Array.isArray(data.paths)
    ) {
      return null;
    }
    const paths = data.paths.filter(
      (path) => typeof path === "string" && path !== "",
    );
    if (paths.length === 0) return null;
    return { side: data.side, paths };
  } catch {
    return null;
  }
}

export function allowPaneDragOver(event: {
  preventDefault: () => void;
  dataTransfer: DataTransfer;
}): void {
  event.preventDefault();
  event.dataTransfer.dropEffect = "copy";
}
