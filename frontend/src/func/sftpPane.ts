export enum PaneSide {
  Local = "local",
  Remote = "remote",
}

/** Shared fixed-layout styles for local/remote file tables. */
export const sftpTableSx = {
  tableLayout: "fixed" as const,
  width: "100%",
  "& .MuiTableCell-root": { padding: "4px 8px" },
};
/** Avoid Paper radius + TableContainer overflow clipping the header checkbox. */
export const sftpTableContainerSx = {
  boxShadow: "none",
  borderRadius: 0,
  overflow: "visible",
} as const;
export const sftpCol = {
  checkbox: { width: 36 },
  size: { width: 88 },
  modified: { width: 180 },
} as const;
export const sftpCheckboxSx = { p: 0 } as const;

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
