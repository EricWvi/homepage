import { useEffect, useRef } from "react";

// Movements shorter than this are a bumped desk or a resting hand, not use.
const MOVE_THRESHOLD_PX = 12;
const CHECK_MS = 1000;

/**
 * Calls onIdle once the user has left the page alone for waitMs while it is
 * the window they are looking at: the browser window has focus and this
 * tab is visible. Keys, clicks, the wheel and deliberate pointer movement
 * count as activity. Time spent in another window or tab, or with a dialog
 * or menu open, does not count towards idling.
 */
export function useIdle(waitMs: number, enabled: boolean, onIdle: () => void) {
  const onIdleRef = useRef(onIdle);
  onIdleRef.current = onIdle;

  useEffect(() => {
    if (!enabled || waitMs <= 0) return;
    let last = Date.now();
    let anchor: { x: number; y: number } | null = null;
    const active = () => (last = Date.now());
    const onMove = (e: PointerEvent) => {
      if (anchor && Math.hypot(e.clientX - anchor.x, e.clientY - anchor.y) < MOVE_THRESHOLD_PX) return;
      anchor = { x: e.clientX, y: e.clientY };
      active();
    };

    const timer = window.setInterval(() => {
      const watching = document.visibilityState === "visible" && document.hasFocus();
      const busy = document.querySelector("[role=dialog], [role=alertdialog], [role=menu]");
      if (!watching || busy) return active();
      if (Date.now() - last >= waitMs) {
        active();
        onIdleRef.current();
      }
    }, CHECK_MS);

    const events = ["keydown", "pointerdown", "wheel"] as const;
    for (const type of events) window.addEventListener(type, active, { passive: true, capture: true });
    window.addEventListener("pointermove", onMove, { passive: true, capture: true });
    return () => {
      window.clearInterval(timer);
      for (const type of events) window.removeEventListener(type, active, { capture: true });
      window.removeEventListener("pointermove", onMove, { capture: true });
    };
  }, [waitMs, enabled]);
}
