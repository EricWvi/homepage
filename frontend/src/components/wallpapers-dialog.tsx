import { CheckIcon, ImagePlusIcon, XIcon } from "lucide-react";
import { useRef, useState } from "react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, wallpaperUrl, type Wallpaper } from "@/lib/api";
import { cn } from "@/lib/utils";

const ACCEPT = "image/jpeg,image/png,image/webp";

/** The user's wallpapers as thumbnails: upload, delete, pick the current one. */
export function WallpapersDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { snapshot, mutate } = useSnapshot();
  const wallpapers = snapshot?.wallpapers ?? [];
  const current = snapshot?.currentWallpaperId ?? null;
  const fileInput = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<{ done: number; total: number } | null>(null);
  const [deleting, setDeleting] = useState<Wallpaper | null>(null);

  async function upload(files: File[]) {
    setUploading({ done: 0, total: files.length });
    for (const [i, file] of files.entries()) {
      await mutate(() => api.addWallpaper(file));
      setUploading({ done: i + 1, total: files.length });
    }
    setUploading(null);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>壁纸库</DialogTitle>
          <DialogDescription>
            支持 JPEG、PNG、WebP，单张不超过 40 MB。点击壁纸设为当前壁纸；全屏时按 → 随机切换，← 回到上一张。
          </DialogDescription>
        </DialogHeader>

        {wallpapers.length === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">还没有壁纸</p>
        ) : (
          <ul className="grid max-h-[60vh] grid-cols-2 gap-3 overflow-y-auto p-1 sm:grid-cols-3">
            {wallpapers.map((w) => (
              <li key={w.id} className="group relative">
                <button
                  type="button"
                  title={`${w.width} × ${w.height}`}
                  aria-label={`设为当前壁纸（${w.width} × ${w.height}）`}
                  aria-pressed={w.id === current}
                  onClick={() => void mutate(() => api.setCurrentWallpaper(w.id))}
                  className={cn(
                    "block aspect-video w-full overflow-hidden rounded-lg bg-muted outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                    w.id === current && "ring-2 ring-primary ring-offset-2 ring-offset-popover",
                  )}
                >
                  <img
                    src={wallpaperUrl(w.thumb)}
                    alt=""
                    loading="lazy"
                    decoding="async"
                    draggable={false}
                    className="size-full object-cover"
                  />
                </button>
                {w.id === current && (
                  <span className="pointer-events-none absolute bottom-2 left-2 flex size-5 items-center justify-center rounded-full bg-primary text-primary-foreground">
                    <CheckIcon className="size-3.5" />
                  </span>
                )}
                <Button
                  variant="secondary"
                  size="icon-sm"
                  aria-label="删除壁纸"
                  onClick={() => setDeleting(w)}
                  className="absolute top-1.5 right-1.5 size-7 rounded-full opacity-0 shadow-sm transition-opacity group-focus-within:opacity-100 group-hover:opacity-100"
                >
                  <XIcon />
                </Button>
              </li>
            ))}
          </ul>
        )}

        <div className="flex justify-end">
          <input
            ref={fileInput}
            type="file"
            accept={ACCEPT}
            multiple
            hidden
            onChange={(e) => {
              const files = Array.from(e.target.files ?? []);
              e.target.value = "";
              if (files.length) void upload(files);
            }}
          />
          <Button onClick={() => fileInput.current?.click()} disabled={uploading !== null}>
            <ImagePlusIcon />
            {uploading ? `上传中 ${uploading.done}/${uploading.total}` : "上传壁纸"}
          </Button>
        </div>

        <ConfirmDialog
          open={deleting !== null}
          title="删除壁纸"
          description="确定从壁纸库删除这张壁纸吗？"
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            if (deleting) void mutate(() => api.deleteWallpaper(deleting.id));
            setDeleting(null);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
