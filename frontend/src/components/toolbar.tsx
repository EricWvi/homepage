import type { LucideIcon } from "lucide-react";

import { Button } from "@/components/ui/button";

export type ToolbarAction = { label: string; icon: LucideIcon; onClick: () => void };

/** Quiet controls in the top-right corner that brighten on hover. */
export function Toolbar({ actions }: { actions: ToolbarAction[] }) {
  return (
    <nav className="fixed top-3 right-3 z-10 flex gap-1 opacity-50 transition-opacity focus-within:opacity-100 hover:opacity-100">
      {actions.map(({ label, icon: Icon, onClick }) => (
        <Button key={label} variant="ghost" size="icon" title={label} aria-label={label} onClick={onClick}>
          <Icon />
        </Button>
      ))}
    </nav>
  );
}
