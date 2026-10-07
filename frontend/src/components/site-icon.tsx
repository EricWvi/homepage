import { useState } from "react";

import { iconUrl } from "@/lib/api";
import { cn } from "@/lib/utils";

type SiteIconProps = {
  /** Domain the icon belongs to; also seeds the fallback colour. */
  domain: string;
  icon: string | null | undefined;
  /** Text whose first character is shown when there is no icon. */
  label: string;
  className?: string;
};

/** A Safari-style rounded square: the uploaded icon, or a lettered tile. */
export function SiteIcon({ domain, icon, label, className }: SiteIconProps) {
  const [failed, setFailed] = useState<string | null>(null);
  const showImage = icon && failed !== icon;
  return (
    <div
      className={cn(
        "relative flex size-16 shrink-0 items-center justify-center overflow-hidden rounded-2xl shadow-[0_1px_3px_rgb(0_0_0/0.12)] select-none dark:brightness-90",
        showImage ? "bg-tile" : "text-white/95",
        className,
      )}
      style={showImage ? undefined : { backgroundColor: fallbackColor(domain) }}
    >
      {showImage ? (
        <img
          src={iconUrl(icon)}
          alt=""
          draggable={false}
          decoding="async"
          className="size-full object-cover"
          onError={() => setFailed(icon)}
        />
      ) : (
        <span className="text-[1.75em] leading-none font-semibold">{firstLetter(label)}</span>
      )}
    </div>
  );
}

function firstLetter(label: string) {
  const [first] = Array.from(label.trim());
  return (first ?? "?").toLocaleUpperCase();
}

// Muted, evenly-lit colours so lettered tiles stay calm in both themes.
function fallbackColor(seed: string) {
  let hash = 0;
  for (const ch of seed) hash = (hash * 31 + ch.charCodeAt(0)) | 0;
  const hue = Math.abs(hash) % 360;
  return `oklch(0.62 0.075 ${hue})`;
}
