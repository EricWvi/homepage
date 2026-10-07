import type { Site, SiteLink } from "@/lib/api";
import { linkPath } from "@/lib/site-links";

export type Suggestion = {
  key: string;
  site: Site;
  link: SiteLink | null;
  url: string;
};

const withoutScheme = (url: string) => url.replace(/^https?:\/\//, "");

/**
 * Sites and site links whose title or URL contain every word of the query,
 * best matches first: the title starts with the query, then contains it,
 * then the words only match elsewhere. Ties keep page order.
 */
export function suggest(sites: Site[], query: string, limit = 8): Suggestion[] {
  const q = query.trim().toLocaleLowerCase();
  if (!q) return [];
  const words = q.split(/\s+/);

  const scored: { s: Suggestion; score: number }[] = [];
  const consider = (site: Site, link: SiteLink | null) => {
    const title = (link?.title ?? site.title).toLocaleLowerCase();
    const haystack = [site.title, link?.title ?? "", withoutScheme(link?.url ?? site.url)].join(" ").toLocaleLowerCase();
    if (!words.every((w) => haystack.includes(w))) return;
    const score = title.startsWith(q) ? 0 : title.includes(q) ? 1 : 2;
    const url = link?.url ?? site.url;
    scored.push({ s: { key: `${site.id}:${link ? url : ""}`, site, link, url }, score });
  };
  for (const site of sites) {
    consider(site, null);
    for (const link of site.links) consider(site, link);
  }
  return scored
    .sort((a, b) => a.score - b.score) // stable: ties keep page order
    .slice(0, limit)
    .map(({ s }) => s);
}

/** The secondary line under a suggestion. */
export function suggestionDetail({ site, link, url }: Suggestion): string {
  return link ? `${site.title} · ${linkPath(url)}` : withoutScheme(url).replace(/\/$/, "");
}
