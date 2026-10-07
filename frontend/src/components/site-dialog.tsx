import { ArrowDownIcon, ArrowUpIcon, PlusIcon, XIcon } from "lucide-react";
import { useRef, useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Site } from "@/lib/api";
import { defaultLinkTitle, editableLink } from "@/lib/site-links";

/** `{ site: null }` adds a new site, `{ site }` edits one, `null` is closed. */
export type SiteDialogTarget = { site: Site | null } | null;

type SiteDialogProps = {
  target: SiteDialogTarget;
  onClose: () => void;
};

export function SiteDialog({ target, onClose }: SiteDialogProps) {
  // Keep rendering the last target while the dialog animates out, so the
  // content doesn't vanish and collapse the dialog mid-animation.
  const [shown, setShown] = useState(target);
  if (target !== null && target !== shown) setShown(target);

  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        {shown && <SiteForm key={shown.site?.id ?? "new"} site={shown.site} onDone={onClose} />}
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
  const [links, setLinks] = useState(() =>
    (site?.links ?? []).map((l, i) => ({
      key: i,
      // A derived title stays empty so it follows the path when that is edited.
      title: l.title === defaultLinkTitle(l.url) ? "" : l.title,
      url: editableLink(l.url, site?.url ?? ""),
    })),
  );
  const nextKey = useRef(links.length);
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    const input = {
      title,
      url,
      groupId: Number(groupId),
      links: links.filter((l) => l.title.trim() || l.url.trim()).map(({ title, url }) => ({ title, url })),
    };
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
      <LinksEditor
        links={links}
        onChange={setLinks}
        onAdd={() => setLinks([...links, { key: nextKey.current++, title: "", url: "" }])}
      />

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

type LinkRow = { key: number; title: string; url: string };

/** Other pages on the site's domain, entered as a path or a full URL. */
function LinksEditor({
  links,
  onChange,
  onAdd,
}: {
  links: LinkRow[];
  onChange: (links: LinkRow[]) => void;
  onAdd: () => void;
}) {
  const update = (i: number, patch: Partial<LinkRow>) =>
    onChange(links.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  const move = (i: number, step: number) => {
    const next = [...links];
    [next[i], next[i + step]] = [next[i + step]!, next[i]!];
    onChange(next);
  };
  const lastKey = links.at(-1)?.key;

  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between">
        <Label>子链接</Label>
        <Button type="button" variant="ghost" size="sm" onClick={onAdd}>
          <PlusIcon />
          添加
        </Button>
      </div>
      {links.length === 0 ? (
        <p className="text-xs text-muted-foreground">同一网站下常用的页面，例如 GitHub 上的仓库。</p>
      ) : (
        <ul className="-m-1 grid max-h-64 gap-1.5 overflow-y-auto p-1">
          {links.map((l, i) => (
            <li key={l.key} className="flex items-center gap-1">
              <Input
                aria-label="子链接路径"
                value={l.url}
                onChange={(e) => update(i, { url: e.target.value })}
                placeholder="路径，如 eric/palace"
                autoFocus={l.key === lastKey && !l.url && !l.title}
                className="h-9 min-w-0 flex-1"
              />
              <Input
                aria-label="子链接标题"
                value={l.title}
                onChange={(e) => update(i, { title: e.target.value })}
                placeholder="标题（可选）"
                maxLength={200}
                className="h-9 w-28"
              />
              <Button type="button" variant="ghost" size="icon-sm" aria-label="上移" disabled={i === 0} onClick={() => move(i, -1)}>
                <ArrowUpIcon />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="下移"
                disabled={i === links.length - 1}
                onClick={() => move(i, 1)}
              >
                <ArrowDownIcon />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="删除子链接"
                onClick={() => onChange(links.filter((_, j) => j !== i))}
              >
                <XIcon />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
