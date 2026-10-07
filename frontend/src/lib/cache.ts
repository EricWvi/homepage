import type { Snapshot } from "@/lib/api";

// The last snapshot is kept in localStorage so the page renders on the
// very first frame, before (or without) any network request.
const KEY = "homepage:snapshot:v3";
// Set after an explicit sign-out so the page shows a "signed out" screen
// instead of bouncing straight back through single sign-on.
const SIGNED_OUT_KEY = "homepage:signed-out";

export function readCachedSnapshot(): Snapshot | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return null;
    const snap = JSON.parse(raw) as Snapshot;
    return Array.isArray(snap.groups) && Array.isArray(snap.sites) && Array.isArray(snap.domains) && snap.user
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
      if (name.startsWith("icons-")) await caches.delete(name);
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
