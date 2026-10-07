import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { api, type Snapshot } from "@/lib/api";
import { readCachedSnapshot, writeCachedSnapshot } from "@/lib/cache";

type SnapshotContext = {
  /** null only on the very first visit, before the first response. */
  snapshot: Snapshot | null;
  /** Runs a mutation and adopts the snapshot it returns. Resolves false on failure. */
  mutate: (action: () => Promise<Snapshot>) => Promise<boolean>;
};

const Context = createContext<SnapshotContext | null>(null);

export function SnapshotProvider({ children }: { children: ReactNode }) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(readCachedSnapshot);

  const adopt = useCallback((snap: Snapshot) => {
    setSnapshot(snap);
    writeCachedSnapshot(snap);
  }, []);

  // Revalidate in the background on load, when the tab comes back into
  // view and when the network returns.
  useEffect(() => {
    let inflight = false;
    const refresh = () => {
      if (inflight || document.visibilityState !== "visible") return;
      inflight = true;
      api
        .snapshot()
        .then(adopt)
        .catch(() => {}) // offline: keep showing the cached copy
        .finally(() => (inflight = false));
    };
    refresh();
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("online", refresh);
    return () => {
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("online", refresh);
    };
  }, [adopt]);

  const mutate = useCallback(
    async (action: () => Promise<Snapshot>) => {
      try {
        adopt(await action());
        return true;
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "操作失败");
        return false;
      }
    },
    [adopt],
  );

  const value = useMemo(() => ({ snapshot, mutate }), [snapshot, mutate]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useSnapshot() {
  const ctx = useContext(Context);
  if (!ctx) throw new Error("useSnapshot must be used inside SnapshotProvider");
  return ctx;
}
