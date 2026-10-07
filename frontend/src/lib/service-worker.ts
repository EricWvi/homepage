// The service worker only runs in production builds; in development it
// would serve stale modules over Vite's dev server.
export function registerServiceWorker() {
  if (!import.meta.env.PROD || !("serviceWorker" in navigator)) return;
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch((err) => console.warn("service worker:", err));
  });
}
