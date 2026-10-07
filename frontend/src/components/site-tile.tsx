import { SiteIcon } from "@/components/site-icon";
import type { Site } from "@/lib/api";

export function SiteTile({ site, icon }: { site: Site; icon: string | null | undefined }) {
  return (
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
}
