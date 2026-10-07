import type { Snapshot } from "@/lib/api";

// The last snapshot is kept in localStorage so the page renders on the
// very first frame, before (or without) any network request.
const KEY = "homepage:snapshot:v1";

export function readCachedSnapshot(): Snapshot | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return null;
    const snap = JSON.parse(raw) as Snapshot;
    return Array.isArray(snap.groups) && Array.isArray(snap.sites) && Array.isArray(snap.domains) ? snap : null;
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
