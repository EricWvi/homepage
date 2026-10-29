import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App";
import { Toaster } from "@/components/ui/sonner";
import { SnapshotProvider } from "@/hooks/use-snapshot";
import { installOverlayScrollbars } from "@/lib/overlay-scrollbars";
import { registerServiceWorker } from "@/lib/service-worker";

import "./index.css";

installOverlayScrollbars();
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <SnapshotProvider>
      <App />
      <Toaster position="bottom-center" />
    </SnapshotProvider>
  </StrictMode>,
);

registerServiceWorker();
