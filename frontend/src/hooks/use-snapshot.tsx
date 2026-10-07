import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { api, ApiError, logout, type Snapshot } from "@/lib/api";
import {
  clearUserData,
  isSignedOut,
  pruneWallpaperCache,
  readCachedSnapshot,
  setSignedOut,
  writeCachedSnapshot,
} from "@/lib/cache";
import { isDesktop } from "@/lib/platform";

type SnapshotContext = {
  /** null on the very first visit before the first response, and after signing out. */
  snapshot: Snapshot | null;
  /** True after the user signed out on this device. */
  signedOut: boolean;
  /**
   * Runs a mutation and adopts the snapshot it returns. Resolves false on
   * failure, which is reported in a toast unless `silent`. An `optimistic`
   * snapshot is shown meanwhile and rolled back if the mutation fails.
   */
  mutate: (
    action: () => Promise<Snapshot>,
    options?: { silent?: boolean; optimistic?: Snapshot },
  ) => Promise<boolean>;
  signIn: () => void;
  signOut: () => Promise<void>;
};

const Context = createContext<SnapshotContext | null>(null);

const goToLogin = () => window.location.assign("/auth/login");

export function SnapshotProvider({ children }: { children: ReactNode }) {
  const [signedOut, setSignedOutState] = useState(isSignedOut);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(() => (signedOut ? null : readCachedSnapshot()));
  const latest = useRef(snapshot);
  latest.current = snapshot;

  // Every snapshot adopted here is fresh from the server.
  const adopt = useCallback((snap: Snapshot) => {
    setSnapshot(snap);
    writeCachedSnapshot(snap);
    if (isDesktop) void pruneWallpaperCache(snap);
  }, []);

  // The session is gone (signed out elsewhere, or never existed): drop the
  // previous user's data, then either sign in or stay on the signed-out screen.
  const unauthorized = useCallback(() => {
    setSnapshot(null);
    void clearUserData();
    if (isSignedOut()) setSignedOutState(true);
    else goToLogin();
  }, []);

  // Revalidate in the background on load, when the tab comes back into
  // view and when the network returns.
  useEffect(() => {
    if (signedOut) return;
    let inflight = false;
    const refresh = () => {
      if (inflight || document.visibilityState !== "visible") return;
      inflight = true;
      api
        .snapshot()
        .then(adopt)
        .catch((err) => {
          if (err instanceof ApiError && err.status === 401) unauthorized();
          // otherwise offline: keep showing the cached copy
        })
        .finally(() => (inflight = false));
    };
    refresh();
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("online", refresh);
    return () => {
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("online", refresh);
    };
  }, [adopt, unauthorized, signedOut]);

  const mutate = useCallback(
    async (action: () => Promise<Snapshot>, { silent = false, optimistic }: { silent?: boolean; optimistic?: Snapshot } = {}) => {
      const before = latest.current;
      if (optimistic) setSnapshot(optimistic);
      try {
        adopt(await action());
        return true;
      } catch (err) {
        if (optimistic) setSnapshot((current) => (current === optimistic ? before : current));
        if (err instanceof ApiError && err.status === 401) {
          unauthorized();
          return false;
        }
        if (!silent) toast.error(err instanceof Error ? err.message : "操作失败");
        return false;
      }
    },
    [adopt, unauthorized],
  );

  const signIn = useCallback(() => {
    setSignedOut(false);
    goToLogin();
  }, []);

  const signOut = useCallback(async () => {
    try {
      await logout();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "退出登录失败");
      return;
    }
    setSignedOut(true);
    await clearUserData();
    setSnapshot(null);
    setSignedOutState(true);
  }, []);

  const value = useMemo(
    () => ({ snapshot, signedOut, mutate, signIn, signOut }),
    [snapshot, signedOut, mutate, signIn, signOut],
  );
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useSnapshot() {
  const ctx = useContext(Context);
  if (!ctx) throw new Error("useSnapshot must be used inside SnapshotProvider");
  return ctx;
}
