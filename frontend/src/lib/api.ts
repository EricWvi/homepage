export type Group = {
  id: number;
  name: string;
  isDefault: boolean;
  position: number;
};

export type Site = {
  id: number;
  title: string;
  url: string;
  domain: string;
  groupId: number;
  position: number;
  /** Other pages on the same domain, in the user's order. */
  links: SiteLink[];
};

export type SiteLink = {
  title: string;
  url: string;
};

export type Domain = {
  domain: string;
  /** Hashed icon file name, or null when the user has not uploaded one. */
  icon: string | null;
  siteCount: number;
};

export type Wallpaper = {
  id: number;
  /** Hashed file names under /wallpapers/. */
  file: string;
  thumb: string;
  width: number;
  height: number;
};

export type User = {
  name: string;
  email: string;
};

/** The signed-in user's complete state. Every mutation responds with a fresh one. */
export type Snapshot = {
  groups: Group[];
  sites: Site[];
  domains: Domain[];
  /** Newest first. */
  wallpapers: Wallpaper[];
  /** The wallpaper last shown, or null. */
  currentWallpaperId: number | null;
  user: User;
  /** How long the page waits for input before showing wallpapers. */
  idleWaitSeconds: number;
};

export type SiteInput = {
  title: string;
  url: string;
  /** 0 selects the default group. */
  groupId: number;
  /** Replaces the site's links. A url may be a path on the site; an empty title is derived from it. */
  links: SiteLink[];
};

export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request(method: string, path: string, body?: unknown): Promise<Snapshot> {
  const init: RequestInit = { method, cache: "no-store" };
  if (body instanceof Blob) {
    init.body = body;
  } else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers = { "Content-Type": "application/json" };
  }
  let res: Response;
  try {
    res = await fetch(path, init);
  } catch {
    throw new ApiError(0, navigator.onLine ? "无法连接服务器" : "当前处于离线状态，无法保存修改");
  }
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const message = (data as { error?: string } | null)?.error ?? `请求失败（${res.status}）`;
    throw new ApiError(res.status, message);
  }
  return data as Snapshot;
}

const domainPath = (domain: string) => `/api/domains/${encodeURIComponent(domain)}`;

export const api = {
  snapshot: () => request("GET", "/api/snapshot"),

  createSite: (input: SiteInput) => request("POST", "/api/sites", input),
  updateSite: (id: number, input: SiteInput) => request("PUT", `/api/sites/${id}`, input),
  deleteSite: (id: number) => request("DELETE", `/api/sites/${id}`),

  createGroup: (name: string) => request("POST", "/api/groups", { name }),
  renameGroup: (id: number, name: string) => request("PUT", `/api/groups/${id}`, { name }),
  deleteGroup: (id: number) => request("DELETE", `/api/groups/${id}`),
  reorderGroups: (ids: number[]) => request("PUT", "/api/groups/order", { ids }),

  setIcon: (domain: string, file: Blob) => request("PUT", `${domainPath(domain)}/icon`, file),
  clearIcon: (domain: string) => request("DELETE", `${domainPath(domain)}/icon`),
  deleteDomain: (domain: string) => request("DELETE", domainPath(domain)),

  addWallpaper: (file: Blob) => request("POST", "/api/wallpapers", file),
  deleteWallpaper: (id: number) => request("DELETE", `/api/wallpapers/${id}`),
  setCurrentWallpaper: (id: number) => request("PUT", "/api/wallpapers/current", { id }),
};

export const iconUrl = (name: string) => `/icons/${name}`;
export const wallpaperUrl = (name: string) => `/wallpapers/${name}`;

/** Ends this browser's session on the server. */
export async function logout() {
  let res: Response;
  try {
    res = await fetch("/auth/logout", { method: "POST", cache: "no-store" });
  } catch {
    throw new ApiError(0, "当前处于离线状态，无法退出登录");
  }
  if (!res.ok) throw new ApiError(res.status, "退出登录失败");
}
