import type { Snapshot } from "@/lib/api";

// The last snapshot is kept in localStorage so the page renders on the
// very first frame, before (or without) any network request.
const KEY = "homepage:snapshot:v6";
// Set after an explicit sign-out so the page shows a "signed out" screen
// instead of bouncing straight back through single sign-on.
const SIGNED_OUT_KEY = "homepage:signed-out";

export function readCachedSnapshot(): Snapshot | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return null;
    const snap = JSON.parse(raw) as Snapshot;
    return Array.isArray(snap.groups) &&
      Array.isArray(snap.sites) &&
      Array.isArray(snap.domains) &&
      Array.isArray(snap.wallpapers) &&
      snap.user
      ? snap
      : null;
  } catch {
    return null;
  }
}

export function writeCachedSnapshot(snap: Snapshot) {
  try {
    localStorage.setItem(KEY, JSON.stringify(snap));
  } catch {
    // Storage full or disabled: the page still works, just not offline.
  }
}

/** Forgets everything stored for the signed-in user on this device. */
export async function clearUserData() {
  try {
    localStorage.removeItem(KEY);
  } catch {
    // ignore
  }
  if ("caches" in window) {
    for (const name of await caches.keys()) {
      if (name.startsWith("icons-") || name.startsWith("wallpapers-")) await caches.delete(name);
    }
  }
}

export function isSignedOut() {
  try {
    return localStorage.getItem(SIGNED_OUT_KEY) === "1";
  } catch {
    return false;
  }
}

export function setSignedOut(value: boolean) {
  try {
    if (value) localStorage.setItem(SIGNED_OUT_KEY, "1");
    else localStorage.removeItem(SIGNED_OUT_KEY);
  } catch {
    // ignore
  }
}

/**
 * Drops cached wallpaper images that are no longer in the user's library,
 * whether they were deleted here or on another device. Called with each
 * fresh snapshot from the server.
 */
export async function pruneWallpaperCache(snap: Snapshot) {
  if (!("caches" in window)) return;
  const keep = new Set(snap.wallpapers.flatMap((w) => [w.file, w.thumb]));
  try {
    for (const name of await caches.keys()) {
      if (!name.startsWith("wallpapers-")) continue;
      const cache = await caches.open(name);
      for (const req of await cache.keys()) {
        const file = new URL(req.url).pathname.split("/").pop() ?? "";
        if (!keep.has(file)) await cache.delete(req);
      }
    }
  } catch {
    // Storage unavailable: nothing to prune.
  }
}
