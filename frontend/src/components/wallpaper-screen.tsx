import { useCallback, useEffect, useRef, useState } from "react";

import { useSnapshot } from "@/hooks/use-snapshot";
import { api, wallpaperUrl, type Wallpaper } from "@/lib/api";
import { randomIndex } from "@/lib/random";
import { cn } from "@/lib/utils";

const CURSOR_HIDE_MS = 3000;
// Arrow presses in quick succession save only where the user stops.
const SAVE_DELAY_MS = 800;
const PAGE_KEYS = new Set(["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Enter", " ", "Tab", "PageUp", "PageDown", "Home", "End", "Escape"]);

type Layer = { key: number; id: number; loaded: boolean };

/**
 * Covers the page with one wallpaper, filling the screen like macOS
 * "Fill Screen". → shows a random other wallpaper, ← steps back through the
 * ones shown since the screen opened. A click anywhere, or leaving
 * fullscreen (Esc), closes it; nothing else does.
 */
export function WallpaperScreen({
  wallpapers,
  initialId,
  onClose,
}: {
  wallpapers: Wallpaper[];
  initialId: number | null;
  onClose: () => void;
}) {
  const { mutate } = useSnapshot();
  const [current, setCurrent] = useState(() =>
    wallpapers.some((w) => w.id === initialId) ? initialId! : wallpapers[randomIndex(wallpapers.length)]!.id,
  );
  const history = useRef<number[]>([]);
  const [layers, setLayers] = useState<Layer[]>([{ key: 0, id: current, loaded: false }]);
  const nextKey = useRef(1);
  const [cursorHidden, setCursorHidden] = useState(false);

  const show = (id: number) => {
    setCurrent(id);
    // Keep the last visible layer underneath so the new one fades in over it.
    setLayers((ls) => [...ls.filter((l) => l.loaded).slice(-1), { key: nextKey.current++, id, loaded: false }]);
  };

  // Remember the wallpaper on the server once the user settles on one, and
  // on closing. Offline, the save quietly fails and the old one is kept.
  const saved = useRef(initialId);
  const latest = useRef(current);
  latest.current = current;
  const save = useCallback(() => {
    const id = latest.current;
    if (saved.current === id) return;
    saved.current = id;
    void mutate(() => api.setCurrentWallpaper(id), { silent: true });
  }, [mutate]);
  useEffect(() => {
    const timer = window.setTimeout(save, SAVE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [current, save]);
  useEffect(() => save, [save]);

  const close = useRef(onClose);
  close.current = () => {
    if (document.fullscreenElement) void document.exitFullscreen().catch(() => {});
    onClose();
  };

  // Keys: arrows switch, Esc closes when not fullscreen (in fullscreen the
  // browser takes Esc to leave it, which closes the screen below).
  const stateRef = useRef({ current, wallpapers });
  stateRef.current = { current, wallpapers };
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const { current, wallpapers } = stateRef.current;
      // The page underneath must not react, scroll or move focus.
      e.stopPropagation();
      if (!e.metaKey && !e.ctrlKey && !e.altKey && PAGE_KEYS.has(e.key)) e.preventDefault();
      if (e.key === "ArrowRight" && wallpapers.length > 1) {
        const others = wallpapers.filter((w) => w.id !== current);
        history.current.push(current);
        show(others[randomIndex(others.length)]!.id);
      } else if (e.key === "ArrowLeft") {
        const previous = history.current.pop();
        if (previous !== undefined) show(previous);
      } else if (e.key === "Escape" && !document.fullscreenElement) {
        close.current();
      }
    };
    let entered = !!document.fullscreenElement;
    const onFullscreen = () => {
      if (document.fullscreenElement) entered = true;
      else if (entered) close.current();
    };
    window.addEventListener("keydown", onKeyDown, { capture: true });
    document.addEventListener("fullscreenchange", onFullscreen);
    return () => {
      window.removeEventListener("keydown", onKeyDown, { capture: true });
      document.removeEventListener("fullscreenchange", onFullscreen);
    };
  }, []);

  // Hide the pointer once it has rested for a few seconds.
  useEffect(() => {
    let timer = window.setTimeout(() => setCursorHidden(true), CURSOR_HIDE_MS);
    const onMove = () => {
      setCursorHidden(false);
      window.clearTimeout(timer);
      timer = window.setTimeout(() => setCursorHidden(true), CURSOR_HIDE_MS);
    };
    window.addEventListener("pointermove", onMove);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener("pointermove", onMove);
    };
  }, []);

  const byId = new Map(wallpapers.map((w) => [w.id, w]));
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="壁纸"
      className={cn("fixed inset-0 z-[100] overflow-hidden bg-black select-none", cursorHidden && "cursor-none")}
      onClick={() => close.current()}
    >
      {layers.map((layer, i) => {
        const w = byId.get(layer.id);
        if (!w) return null;
        const top = i === layers.length - 1;
        return (
          <img
            key={layer.key}
            src={wallpaperUrl(w.file)}
            alt=""
            draggable={false}
            decoding="async"
            className={cn(
              "absolute inset-0 size-full object-cover transition-opacity duration-700 ease-out",
              layer.loaded ? "opacity-100" : "opacity-0",
            )}
            onLoad={() => setLayers((ls) => ls.map((l) => (l.key === layer.key ? { ...l, loaded: true } : l)))}
            onTransitionEnd={() => top && setLayers((ls) => (ls.length > 1 ? ls.slice(-1) : ls))}
          />
        );
      })}
    </div>
  );
}
