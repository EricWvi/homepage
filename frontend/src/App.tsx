import {
  FolderTreeIcon,
  GalleryHorizontalEndIcon,
  ImagesIcon,
  LogOutIcon,
  MonitorPlayIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { DomainsDialog } from "@/components/domains-dialog";
import { GroupsDialog } from "@/components/groups-dialog";
import { SearchBox } from "@/components/search-box";
import { SiteDialog, type SiteDialogTarget } from "@/components/site-dialog";
import { SiteGrid } from "@/components/site-grid";
import { Toolbar, type ToolbarAction } from "@/components/toolbar";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { WallpaperScreen } from "@/components/wallpaper-screen";
import { WallpapersDialog } from "@/components/wallpapers-dialog";
import { useIdle } from "@/hooks/use-idle";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Site } from "@/lib/api";
import { isDesktop } from "@/lib/platform";

export function App() {
  const { snapshot, signedOut, signIn, signOut, mutate } = useSnapshot();
  const [siteDialog, setSiteDialog] = useState<SiteDialogTarget>(null);
  const [deleting, setDeleting] = useState<Site | null>(null);
  const [groupsOpen, setGroupsOpen] = useState(false);
  const [domainsOpen, setDomainsOpen] = useState(false);
  const [wallpapersOpen, setWallpapersOpen] = useState(false);
  const [wallpaperShown, setWallpaperShown] = useState(false);

  const wallpaperCount = snapshot?.wallpapers.length ?? 0;
  useIdle(
    (snapshot?.idleWaitSeconds ?? 0) * 1000,
    isDesktop && wallpaperCount > 0 && !wallpaperShown,
    () => setWallpaperShown(true), // no click to allow fullscreen: fill the window instead
  );
  const showWallpaper = () => {
    if (wallpaperCount === 0) {
      toast.info("壁纸库还是空的，先上传几张壁纸吧");
      return;
    }
    // Fullscreen needs this click, so request it before anything else.
    void document.documentElement.requestFullscreen().catch(() => {});
    setWallpaperShown(true);
  };

  const isEmpty = snapshot !== null && snapshot.sites.length === 0 && snapshot.groups.length <= 1;
  const addSite = () => setSiteDialog({ site: null });

  if (signedOut) {
    return (
      <main className="flex min-h-dvh flex-col items-center justify-center gap-4 text-sm text-muted-foreground">
        <p>已退出登录</p>
        <Button variant="outline" onClick={signIn}>
          登录
        </Button>
      </main>
    );
  }
  if (!snapshot) return null; // first visit: waiting for the server or the login redirect

  const user = snapshot.user.name || snapshot.user.email;

  return (
    <>
      <Toolbar
        actions={[
          { label: "添加网站", icon: PlusIcon, onClick: addSite },
          { label: "分组管理", icon: FolderTreeIcon, onClick: () => setGroupsOpen(true) },
          { label: "图标管理", icon: ImagesIcon, onClick: () => setDomainsOpen(true) },
          ...(isDesktop
            ? ([
                { label: "壁纸库", icon: GalleryHorizontalEndIcon, onClick: () => setWallpapersOpen(true) },
                { label: "壁纸", icon: MonitorPlayIcon, onClick: showWallpaper },
              ] satisfies ToolbarAction[])
            : []),
          { label: user ? `退出登录（${user}）` : "退出登录", icon: LogOutIcon, onClick: () => void signOut() },
        ]}
      />

      <main className="mx-auto w-full max-w-[780px] px-4 pt-[12vh] pb-24 sm:px-6">
        <SearchBox />
        {!isEmpty && (
          <SiteGrid
            snapshot={snapshot}
            renderTile={(site, tile) => (
              <ContextMenu>
                <ContextMenuTrigger asChild>
                  <div>{tile}</div>
                </ContextMenuTrigger>
                <ContextMenuContent>
                  <ContextMenuItem onSelect={() => setSiteDialog({ site })}>
                    <PencilIcon />
                    编辑
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem variant="destructive" onSelect={() => setDeleting(site)}>
                    <Trash2Icon />
                    删除
                  </ContextMenuItem>
                </ContextMenuContent>
              </ContextMenu>
            )}
          />
        )}
        {isEmpty && (
          <div className="flex flex-col items-center gap-4 pt-16 text-sm text-muted-foreground">
            <p>还没有收藏的网站</p>
            <Button variant="outline" onClick={addSite}>
              <PlusIcon />
              添加网站
            </Button>
          </div>
        )}
      </main>

      <SiteDialog target={siteDialog} onClose={() => setSiteDialog(null)} />
      <GroupsDialog open={groupsOpen} onOpenChange={setGroupsOpen} />
      <DomainsDialog open={domainsOpen} onOpenChange={setDomainsOpen} />
      {isDesktop && <WallpapersDialog open={wallpapersOpen} onOpenChange={setWallpapersOpen} />}
      {isDesktop && wallpaperShown && wallpaperCount > 0 && (
        <WallpaperScreen
          wallpapers={snapshot.wallpapers}
          initialId={snapshot.currentWallpaperId}
          onClose={() => setWallpaperShown(false)}
        />
      )}
      <ConfirmDialog
        open={deleting !== null}
        title="删除网站"
        description={`确定删除“${deleting?.title ?? ""}”吗？该域名的图标会保留。`}
        onCancel={() => setDeleting(null)}
        onConfirm={() => {
          if (deleting) void mutate(() => api.deleteSite(deleting.id));
          setDeleting(null);
        }}
      />
    </>
  );
}
