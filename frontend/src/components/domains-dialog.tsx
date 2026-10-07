import { ImageOffIcon, Trash2Icon, UploadIcon } from "lucide-react";
import { useRef, useState } from "react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { SiteIcon } from "@/components/site-icon";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Domain } from "@/lib/api";

const ACCEPT = "image/png,image/jpeg,image/gif,image/webp,image/x-icon,image/svg+xml,.ico,.svg";

type DomainsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/**
 * Icons belong to domains, not sites. Every domain ever added stays here
 * until it is deleted explicitly, even after its last site is gone.
 */
export function DomainsDialog({ open, onOpenChange }: DomainsDialogProps) {
  const { snapshot, mutate } = useSnapshot();
  const fileInput = useRef<HTMLInputElement>(null);
  const [uploadFor, setUploadFor] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<Domain | null>(null);
  const domains = snapshot?.domains ?? [];

  function pick(domain: string) {
    setUploadFor(domain);
    fileInput.current?.click();
  }

  function upload(file: File | undefined) {
    if (file && uploadFor) void mutate(() => api.setIcon(uploadFor, file));
    setUploadFor(null);
    if (fileInput.current) fileInput.current.value = "";
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>图标管理</DialogTitle>
          <DialogDescription>同一域名下的网站共用一个图标。删除网站不会删除这里的域名。</DialogDescription>
        </DialogHeader>

        <input
          ref={fileInput}
          type="file"
          accept={ACCEPT}
          className="hidden"
          onChange={(e) => upload(e.target.files?.[0])}
        />

        {domains.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">添加网站后，域名会出现在这里。</p>
        ) : (
          <ul className="-mx-2 grid gap-1">
            {domains.map((d) => (
              <li key={d.domain} className="flex items-center gap-3 rounded-lg px-2 py-1.5 hover:bg-accent/60">
                <SiteIcon domain={d.domain} icon={d.icon} label={d.domain} className="size-10 rounded-xl text-[0.6rem]" />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm">{d.domain}</div>
                  <div className="text-xs text-muted-foreground">
                    {d.siteCount > 0 ? `${d.siteCount} 个网站` : "没有网站使用"}
                    {d.icon ? "" : " · 未设置图标"}
                  </div>
                </div>
                <Button variant="ghost" size="icon-sm" title={d.icon ? "更换图标" : "上传图标"} onClick={() => pick(d.domain)}>
                  <UploadIcon />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title="移除图标"
                  disabled={!d.icon}
                  onClick={() => void mutate(() => api.clearIcon(d.domain))}
                >
                  <ImageOffIcon />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  title={d.siteCount > 0 ? "仍有网站使用该域名，无法删除" : "删除域名"}
                  disabled={d.siteCount > 0}
                  onClick={() => setDeleting(d)}
                >
                  <Trash2Icon className="text-destructive" />
                </Button>
              </li>
            ))}
          </ul>
        )}

        <ConfirmDialog
          open={deleting !== null}
          title="删除域名"
          description={`确定删除 ${deleting?.domain ?? ""} 及其图标吗？`}
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            if (deleting) void mutate(() => api.deleteDomain(deleting.domain));
            setDeleting(null);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
