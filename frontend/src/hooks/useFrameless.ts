import { useEffect, useState } from "react";
import { Window } from "@wailsio/runtime";
import { ConfigService } from "../../bindings/github.com/ilaziness/vela/services";

export const windowMaximisedEvent = "vela-window-maximised";

export function toggleWindowMaximise(): void {
  Window.ToggleMaximise()
    .then(() => Window.IsMaximised())
    .then((maximised) => {
      window.dispatchEvent(
        new CustomEvent(windowMaximisedEvent, { detail: maximised }),
      );
    })
    .catch(() => {});
}

let cached: boolean | null = null;
let inflight: Promise<boolean> | null = null;

function fetchFrameless(): Promise<boolean> {
  if (cached !== null) {
    return Promise.resolve(cached);
  }
  if (!inflight) {
    inflight = ConfigService.IsFrameless()
      .then((value) => {
        cached = Boolean(value);
        return cached;
      })
      .finally(() => {
        if (cached === null) {
          inflight = null;
        }
      });
  }
  return inflight;
}

export function useFrameless(): boolean | null {
  const [frameless, setFrameless] = useState<boolean | null>(cached);

  useEffect(() => {
    let active = true;
    fetchFrameless()
      .then((value) => {
        if (active) {
          setFrameless(value);
        }
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  return frameless;
}

export function dragRegionSx(
  frameless: boolean | null,
): { "--wails-draggable": "drag" } | undefined {
  if (frameless !== true) {
    return undefined;
  }
  return { "--wails-draggable": "drag" };
}
