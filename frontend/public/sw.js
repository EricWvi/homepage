// Service worker: makes the page open instantly and work fully offline.
//
//   page shell (/)   stale-while-revalidate; a new shell is only adopted
//                    once every asset it references is cached
//   /assets/*        cache-first; Vite content-hashes these files
//   /icons/*         cache-first; icon names are content hashes
//   /api/*           untouched; the app keeps its own snapshot copy
//   /auth/*          untouched; login redirects must reach the server
//
// Bump CACHE_VERSION only when this file's caching scheme changes.
const CACHE_VERSION = "v1";
const SHELL = `shell-${CACHE_VERSION}`;
const ASSETS = `assets-${CACHE_VERSION}`;
const ICONS = `icons-${CACHE_VERSION}`;
const SHELL_URL = "/";
const STATIC_FILES = ["/favicon.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil(updateShell().then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keep = new Set([SHELL, ASSETS, ICONS]);
      for (const name of await caches.keys()) {
        if (!keep.has(name)) await caches.delete(name);
      }
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || url.pathname.startsWith("/auth/")) return;

  if (request.mode === "navigate") {
    event.respondWith(serveShell(event));
  } else if (url.pathname.startsWith("/assets/")) {
    event.respondWith(cacheFirst(ASSETS, request));
  } else if (url.pathname.startsWith("/icons/")) {
    event.respondWith(cacheFirst(ICONS, request));
  } else if (STATIC_FILES.includes(url.pathname)) {
    event.respondWith(staleWhileRevalidate(SHELL, request, event));
  }
});

async function serveShell(event) {
  const cached = await caches.match(SHELL_URL, { cacheName: SHELL });
  if (cached) {
    event.waitUntil(updateShell().catch(() => {}));
    return cached;
  }
  try {
    await updateShell();
    return (await caches.match(SHELL_URL, { cacheName: SHELL })) ?? fetch(event.request);
  } catch {
    return fetch(event.request);
  }
}

// Fetch the latest shell and cache it together with everything it needs,
// so a cached shell can never point at an asset that is not available.
async function updateShell() {
  const res = await fetch(SHELL_URL, { cache: "no-cache" });
  if (!res.ok) throw new Error(`shell: ${res.status}`);
  const html = await res.clone().text();
  const assets = [...new Set([...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map((m) => m[1]))];

  const assetCache = await caches.open(ASSETS);
  await Promise.all(
    assets.map(async (path) => {
      if (!(await assetCache.match(path))) await assetCache.add(path);
    }),
  );

  const shellCache = await caches.open(SHELL);
  await shellCache.put(SHELL_URL, res);
  await Promise.all(STATIC_FILES.map((path) => shellCache.add(path).catch(() => {})));

  // Drop assets that no release we can still serve refers to.
  const wanted = new Set(assets.map((path) => new URL(path, self.location.origin).href));
  for (const req of await assetCache.keys()) {
    if (!wanted.has(req.url)) await assetCache.delete(req);
  }
}

async function cacheFirst(cacheName, request) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);
  if (cached) return cached;
  const res = await fetch(request);
  if (res.ok) await cache.put(request, res.clone());
  return res;
}

async function staleWhileRevalidate(cacheName, request, event) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);
  const update = fetch(request).then(async (res) => {
    if (res.ok) await cache.put(request, res.clone());
    return res;
  });
  if (cached) {
    event.waitUntil(update.catch(() => {}));
    return cached;
  }
  return update;
}
