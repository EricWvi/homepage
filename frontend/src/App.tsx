import { SiteGrid } from "@/components/site-grid";
import { useSnapshot } from "@/hooks/use-snapshot";

export function App() {
  const { snapshot } = useSnapshot();
  const isEmpty = snapshot !== null && snapshot.sites.length === 0 && snapshot.groups.length <= 1;

  return (
    <main className="mx-auto w-full max-w-[780px] px-4 pt-[12vh] pb-24 sm:px-6">
      {snapshot && !isEmpty && <SiteGrid snapshot={snapshot} />}
      {isEmpty && <p className="pt-16 text-center text-sm text-muted-foreground">还没有收藏的网站</p>}
    </main>
  );
}
