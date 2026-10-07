import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Site } from "@/lib/api";

/** `{ site: null }` adds a new site, `{ site }` edits one, `null` is closed. */
export type SiteDialogTarget = { site: Site | null } | null;

type SiteDialogProps = {
  target: SiteDialogTarget;
  onClose: () => void;
};

export function SiteDialog({ target, onClose }: SiteDialogProps) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        {target && <SiteForm key={target.site?.id ?? "new"} site={target.site} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function SiteForm({ site, onDone }: { site: Site | null; onDone: () => void }) {
  const { snapshot, mutate } = useSnapshot();
  const groups = snapshot?.groups ?? [];
  const defaultGroup = groups.find((g) => g.isDefault);

  const [title, setTitle] = useState(site?.title ?? "");
  const [url, setUrl] = useState(site?.url ?? "");
  const [groupId, setGroupId] = useState(String(site?.groupId ?? defaultGroup?.id ?? 0));
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    const input = { title, url, groupId: Number(groupId) };
    const ok = await mutate(() => (site ? api.updateSite(site.id, input) : api.createSite(input)));
    setSaving(false);
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="grid gap-5">
      <DialogHeader>
        <DialogTitle>{site ? "编辑网站" : "添加网站"}</DialogTitle>
        <DialogDescription>图标跟随域名，可在“图标管理”中设置。</DialogDescription>
      </DialogHeader>

      <div className="grid gap-2">
        <Label htmlFor="site-title">标题</Label>
        <Input id="site-title" value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus maxLength={200} />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="site-url">链接</Label>
        <Input
          id="site-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://example.com"
          inputMode="url"
          autoComplete="url"
          required
        />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="site-group">分组</Label>
        <Select value={groupId} onValueChange={setGroupId}>
          <SelectTrigger id="site-group">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {groups.map((g) => (
              <SelectItem key={g.id} value={String(g.id)}>
                {g.isDefault ? "默认分组" : g.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          取消
        </Button>
        <Button type="submit" disabled={saving}>
          {site ? "保存" : "添加"}
        </Button>
      </DialogFooter>
    </form>
  );
}
