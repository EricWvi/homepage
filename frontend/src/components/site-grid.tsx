import type { ReactNode } from "react";

import { SiteTile } from "@/components/site-tile";
import type { Group, Site, Snapshot } from "@/lib/api";

type SiteGridProps = {
  snapshot: Snapshot;
  /** Leaves out the groups hidden at work. */
  workMode?: boolean;
  /** Wraps each tile, e.g. to attach a context menu. */
  renderTile?: (site: Site, tile: ReactNode) => ReactNode;
};

/**
 * The default group comes first with no heading; custom groups follow in a
 * single column, each with its name above its own grid.
 */
export function SiteGrid({ snapshot, workMode = false, renderTile = (_, tile) => tile }: SiteGridProps) {
  const icons = new Map(snapshot.domains.map((d) => [d.domain, d.icon]));
  const byGroup = new Map<number, Site[]>();
  for (const site of snapshot.sites) {
    const list = byGroup.get(site.groupId) ?? [];
    list.push(site);
    byGroup.set(site.groupId, list);
  }

  const tiles = (sites: Site[]) => (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(88px,1fr))] justify-items-center gap-x-2 gap-y-3">
      {sites.map((site) => (
        <div key={site.id}>{renderTile(site, <SiteTile site={site} icon={icons.get(site.domain)} />)}</div>
      ))}
    </div>
  );

  return (
    <div className="flex flex-col gap-8">
      {snapshot.groups.map((group: Group) => {
        if (workMode && group.hiddenAtWork) return null;
        const sites = byGroup.get(group.id) ?? [];
        if (group.isDefault) return sites.length > 0 ? <section key={group.id}>{tiles(sites)}</section> : null;
        return (
          <section key={group.id} aria-label={group.name}>
            <h2 className="mb-2 px-3 text-[15px] font-semibold text-foreground/75">{group.name}</h2>
            {sites.length > 0 ? tiles(sites) : <p className="px-3 text-xs text-muted-foreground">暂无网站</p>}
          </section>
        );
      })}
    </div>
  );
}
