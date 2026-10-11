import tlds from "tlds";

const knownTlds = new Set(tlds);

/**
 * The address to open when the search box holds a web address rather than
 * a query, following Chrome's address bar: an explicit http(s) URL, or a
 * host with an optional port and path, such as example.com, localhost:3000
 * or 192.168.1.1/admin. A domain counts only when it ends in a real
 * top-level domain, so "node.js" or "v1.2" are still searched. Domains open
 * over https; localhost and IP addresses over http. Anything else, including
 * text with spaces and no scheme, returns null.
 */
export function typedAddress(input: string): string | null {
  const text = input.trim();
  if (/^https?:\/\//i.test(text)) {
    try {
      const url = new URL(text);
      return url.hostname ? url.href : null;
    } catch {
      return null;
    }
  }

  if (/\s/.test(text)) return null;

  // [host] or host, an optional port, then the path, query or hash.
  const m = /^(\[[^\]]*\]|[^/?#:@[\]]+)(?::(\d{1,5}))?(?:[/?#].*)?$/.exec(text);
  if (!m) return null;
  const host = m[1]!.toLowerCase().replace(/\.$/, "");
  const port = m[2];

  let secure: boolean;
  if (host === "localhost" || host.endsWith(".localhost")) secure = false;
  else if (host.startsWith("[")) secure = false;
  else if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) {
    if (host.split(".").some((n) => Number(n) > 255)) return null;
    secure = false;
  } else {
    const labels = host.split(".");
    if (labels.length < 2 || labels.some((l) => !l)) return null;
    if (!knownTlds.has(labels.at(-1)!)) return null;
    secure = true;
  }
  if (port !== undefined && Number(port) > 65535) return null;

  try {
    return new URL(`${secure ? "https" : "http"}://${text}`).href;
  } catch {
    return null;
  }
}
