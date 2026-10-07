/**
 * The part of a link worth showing next to its site: the path, query and
 * hash without the origin, e.g. "eric/palace" for github.com/eric/palace.
 * A link to the site root shows its host.
 */
export function linkPath(url: string): string {
  try {
    const u = new URL(url);
    const rest = (u.pathname + u.search + u.hash).replace(/^\/+/, "");
    return rest || u.host;
  } catch {
    return url;
  }
}

/** How a link is shown in the editor: a path when it is on the site's origin. */
export function editableLink(url: string, siteUrl: string): string {
  try {
    if (new URL(url).origin === new URL(siteUrl).origin) return linkPath(url);
  } catch {
    // fall through to the full URL
  }
  return url;
}

/**
 * The title the server gives a link saved without one: the last two path
 * segments, or the host for the site root. Mirrors store.linkTitle.
 */
export function defaultLinkTitle(url: string): string {
  try {
    const u = new URL(url);
    const segs = u.pathname.split("/").filter(Boolean).map(decodeURIComponent);
    return segs.length ? segs.slice(-2).join("/") : u.hostname;
  } catch {
    return "";
  }
}
