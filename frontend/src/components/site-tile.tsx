import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from "react";

import { SiteIcon } from "@/components/site-icon";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { Site } from "@/lib/api";
import { linkPath } from "@/lib/site-links";

// Resting on a tile's badge opens its links; a short grace period lets the
// pointer travel from the badge into the list. Brushing past opens nothing.
const HOVER_OPEN_MS = 400;
const HOVER_CLOSE_MS = 150;

export function SiteTile({ site, icon }: { site: Site; icon: string | null | undefined }) {
  const link = (
    <a
      href={site.url}
      title={`${site.title}\n${site.url}`}
      className="group flex w-[88px] flex-col items-center gap-2 rounded-xl px-1 py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
    >
      <SiteIcon
        domain={site.domain}
        icon={icon}
        label={site.title}
        className="transition-transform duration-150 group-hover:scale-[1.04] group-active:scale-95"
      />
      <span className="w-full truncate text-center text-xs text-foreground/80">{site.title}</span>
    </a>
  );
  return site.links.length > 0 ? <TileWithLinks site={site}>{link}</TileWithLinks> : link;
}

/**
 * Adds a count badge and a list of the site's links, anchored to the badge.
 * Hovering the badge opens the list without taking focus and closes it when
 * the pointer leaves; clicking the badge opens it for the keyboard and keeps
 * it open until dismissed.
 */
function TileWithLinks({ site, children }: { site: Site; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const openRef = useRef(open);
  openRef.current = open;
  const mode = useRef<"hover" | "click">("hover");
  const timer = useRef<number | undefined>(undefined);
  const content = useRef<HTMLDivElement>(null);

  const cancel = () => window.clearTimeout(timer.current);
  useEffect(() => cancel, []);

  const enter = (e: PointerEvent) => {
    if (e.pointerType !== "mouse") return;
    cancel();
    if (openRef.current) return;
    timer.current = window.setTimeout(() => {
      mode.current = "hover";
      setOpen(true);
    }, HOVER_OPEN_MS);
  };
  const leave = (e: PointerEvent) => {
    if (e.pointerType !== "mouse") return;
    cancel();
    if (openRef.current && mode.current === "click") return;
    timer.current = window.setTimeout(() => setOpen(false), HOVER_CLOSE_MS);
  };

  const links = () => Array.from(content.current?.querySelectorAll("a") ?? []);
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const all = links();
    const current = all.indexOf(document.activeElement as HTMLAnchorElement);
    const step = e.key === "ArrowDown" ? 1 : -1;
    all.at(current === -1 ? (step > 0 ? 0 : -1) : (current + step) % all.length)?.focus();
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        cancel();
        setOpen(next);
      }}
    >
      <div
        className="relative"
        onContextMenu={() => {
          cancel();
          setOpen(false);
        }}
      >
        {children}
        <PopoverTrigger asChild>
          <button
            type="button"
            onPointerEnter={enter}
            onPointerLeave={leave}
            aria-label={`${site.title}的 ${site.links.length} 个子链接`}
            title="子链接"
            className="absolute top-13.5 right-1.5 flex h-5 min-w-5 items-center justify-center rounded-full border bg-card px-1 text-[11px] leading-none font-medium text-foreground/70 shadow-sm outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60"
            onClick={(e) => {
              // Already open from hovering: keep it open and hand it to the keyboard.
              if (open && mode.current === "hover") {
                e.preventDefault();
                links()[0]?.focus();
              }
              mode.current = "click";
            }}
          >
            {site.links.length}
          </button>
        </PopoverTrigger>
      </div>
      <PopoverContent
        ref={content}
        side="bottom"
        align="start"
        collisionPadding={8}
        className="w-64"
        onPointerEnter={enter}
        onPointerLeave={leave}
        onKeyDown={onKeyDown}
        onOpenAutoFocus={(e) => {
          // Hovering leaves focus alone; a click starts on the first link.
          e.preventDefault();
          if (mode.current === "click") links()[0]?.focus();
        }}
        onCloseAutoFocus={(e) => mode.current === "hover" && e.preventDefault()}
      >
        <p className="truncate px-2 pt-1 pb-1.5 text-xs text-muted-foreground">{site.title}</p>
        {site.links.map((l, i) => {
          const path = linkPath(l.url);
          return (
            <a
              key={i}
              href={l.url}
              title={l.url}
              className="flex flex-col gap-0.5 rounded-sm px-2 py-1.5 outline-none hover:bg-accent focus-visible:bg-accent"
            >
              <span className="truncate text-sm">{l.title}</span>
              {path !== l.title && <span className="truncate text-xs text-muted-foreground">{path}</span>}
            </a>
          );
        })}
      </PopoverContent>
    </Popover>
  );
}
