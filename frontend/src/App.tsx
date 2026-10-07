import { FolderTreeIcon, ImagesIcon, LogOutIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { DomainsDialog } from "@/components/domains-dialog";
import { GroupsDialog } from "@/components/groups-dialog";
import { SearchBox } from "@/components/search-box";
import { SiteDialog, type SiteDialogTarget } from "@/components/site-dialog";
import { SiteGrid } from "@/components/site-grid";
import { Toolbar } from "@/components/toolbar";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Site } from "@/lib/api";

export function App() {
  const { snapshot, signedOut, signIn, signOut, mutate } = useSnapshot();
  const [siteDialog, setSiteDialog] = useState<SiteDialogTarget>(null);
  const [deleting, setDeleting] = useState<Site | null>(null);
  const [groupsOpen, setGroupsOpen] = useState(false);
  const [domainsOpen, setDomainsOpen] = useState(false);

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
