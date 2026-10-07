import type { LucideIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { isDesktop } from "@/lib/platform";
import { cn } from "@/lib/utils";

export type ToolbarAction = { label: string; icon: LucideIcon; onClick: () => void };

/**
 * Quiet controls in the top-right corner that brighten on hover. On desktop
 * they stay in the window's corner; on phones and tablets they scroll away
 * with the page.
 */
export function Toolbar({ actions }: { actions: ToolbarAction[] }) {
  return (
    <nav
      className={cn(
        "top-3 right-3 z-10 flex gap-1 opacity-50 transition-opacity focus-within:opacity-100 hover:opacity-100",
        isDesktop ? "fixed" : "absolute",
      )}
    >
      {actions.map(({ label, icon: Icon, onClick }) => (
        <Button key={label} variant="ghost" size="icon" title={label} aria-label={label} onClick={onClick}>
          <Icon />
        </Button>
      ))}
    </nav>
  );
}
