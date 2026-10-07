import { DndContext, MouseSensor, closestCenter, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, rectSortingStrategy, useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { generateKeyBetween } from "fractional-indexing";
import type { ReactNode } from "react";

import { SiteTile } from "@/components/site-tile";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Group, type Site, type Snapshot } from "@/lib/api";

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
    <SortableTiles snapshot={snapshot} sites={sites}>
      {(site) => renderTile(site, <SiteTile site={site} icon={icons.get(site.domain)} />)}
    </SortableTiles>
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

/**
 * One group's tiles, reordered by dragging with the mouse. Each group is its
 * own drag context, so sites never leave their group. A drop takes effect
 * at once: the moved site gets a fractional index key between its new
 * neighbours, the same key the server computes.
 */
function SortableTiles({
  snapshot,
  sites,
  children,
}: {
  snapshot: Snapshot;
  sites: Site[];
  children: (site: Site) => ReactNode;
}) {
  const { mutate } = useSnapshot();
  // A small threshold keeps plain clicks working as links.
  const sensors = useSensors(useSensor(MouseSensor, { activationConstraint: { distance: 6 } }));
  function onDragEnd({ active, over }: DragEndEvent) {
    suppressClick();
    if (!over || active.id === over.id) return;
    const from = sites.findIndex((s) => s.id === active.id);
    const to = sites.findIndex((s) => s.id === over.id);
    const moved = arrayMove(sites, from, to);
    const prev = moved[to - 1];
    const next = moved[to + 1];
    const sortKey = generateKeyBetween(prev?.sortKey ?? null, next?.sortKey ?? null);
    const optimistic = {
      ...snapshot,
      sites: snapshot.sites.map((s) => (s.id === active.id ? { ...s, sortKey } : s)).sort(compareSites),
    };
    void mutate(() => api.moveSite(Number(active.id), prev?.id ?? null), { optimistic });
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragCancel={suppressClick}
      onDragEnd={onDragEnd}
    >
      <SortableContext items={sites.map((s) => s.id)} strategy={rectSortingStrategy}>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(88px,1fr))] justify-items-center gap-x-2 gap-y-3">
          {sites.map((site) => (
            <SortableTile key={site.id} id={site.id}>
              {children(site)}
            </SortableTile>
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}

function SortableTile({ id, children }: { id: number; children: ReactNode }) {
  const { setNodeRef, listeners, transform, transition, isDragging } = useSortable({ id });
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      className={isDragging ? "relative z-10 opacity-80" : undefined}
      style={{ transform: CSS.Translate.toString(transform), transition }}
    >
      {children}
    </div>
  );
}

/**
 * Releasing a drag fires a click that would open the link under the
 * pointer. dnd-kit stops its propagation at the document, which also keeps
 * it from React, but leaves the default action, so cancel it on the window
 * first. The click follows mouseup in the same task; drop the guard after.
 */
function suppressClick() {
  const block = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };
  window.addEventListener("click", block, { capture: true, once: true });
  window.setTimeout(() => window.removeEventListener("click", block, { capture: true }));
}

// The server's order: by group, then sort key compared bytewise, then id.
function compareSites(a: Site, b: Site) {
  if (a.groupId !== b.groupId) return a.groupId - b.groupId;
  if (a.sortKey !== b.sortKey) return a.sortKey < b.sortKey ? -1 : 1;
  return a.id - b.id;
}
