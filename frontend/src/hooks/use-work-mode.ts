import { useSyncExternalStore } from "react";

// Work mode belongs to this browser, not the account: it is kept in
// localStorage and follows changes made in other tabs.
const KEY = "homepage:work-mode";
const listeners = new Set<() => void>();

function read() {
  try {
    return localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  const onStorage = (e: StorageEvent) => e.key === KEY && listener();
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

function setWorkMode(on: boolean) {
  try {
    if (on) localStorage.setItem(KEY, "1");
    else localStorage.removeItem(KEY);
  } catch {
    // Storage disabled: the switch can't stick, so it stays off.
  }
  for (const listener of listeners) listener();
}

/** Whether this browser is in work mode, which hides some groups. Off by default. */
export function useWorkMode() {
  return [useSyncExternalStore(subscribe, read), setWorkMode] as const;
}
