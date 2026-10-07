import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useSnapshot } from "@/hooks/use-snapshot";
import { api, type Group } from "@/lib/api";

type GroupsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function GroupsDialog({ open, onOpenChange }: GroupsDialogProps) {
  const { snapshot, mutate } = useSnapshot();
  const [newName, setNewName] = useState("");
  const [deleting, setDeleting] = useState<Group | null>(null);

  const groups = snapshot?.groups ?? [];
  const custom = groups.filter((g) => !g.isDefault);
  const siteCount = (id: number) => snapshot?.sites.filter((s) => s.groupId === id).length ?? 0;

  async function create(e: FormEvent) {
    e.preventDefault();
    if (await mutate(() => api.createGroup(newName))) setNewName("");
  }

  function move(index: number, delta: -1 | 1) {
    const ids = custom.map((g) => g.id);
    const target = index + delta;
    if (target < 0 || target >= ids.length) return;
    [ids[index], ids[target]] = [ids[target]!, ids[index]!];
    void mutate(() => api.reorderGroups(ids));
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>分组管理</DialogTitle>
          <DialogDescription>默认分组始终在最上方且不显示标题。删除分组后，其中的网站会移到默认分组。</DialogDescription>
        </DialogHeader>

        <ul className="grid gap-1.5">
          <li className="flex h-9 items-center justify-between rounded-md bg-muted px-3 text-sm">
            <span>默认分组</span>
            <span className="text-xs text-muted-foreground">{siteCount(groups.find((g) => g.isDefault)?.id ?? 1)} 个网站</span>
          </li>
          {custom.map((group, i) => (
            <li key={group.id} className="flex items-center gap-1">
              <GroupNameInput group={group} />
              <span className="w-14 shrink-0 text-right text-xs text-muted-foreground">{siteCount(group.id)} 个</span>
              <Button variant="ghost" size="icon-sm" aria-label="上移" disabled={i === 0} onClick={() => move(i, -1)}>
                <ArrowUpIcon />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="下移"
                disabled={i === custom.length - 1}
                onClick={() => move(i, 1)}
              >
                <ArrowDownIcon />
              </Button>
              <Button variant="ghost" size="icon-sm" aria-label="删除分组" onClick={() => setDeleting(group)}>
                <Trash2Icon className="text-destructive" />
              </Button>
            </li>
          ))}
        </ul>

        <form onSubmit={create} className="flex gap-2">
          <Input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="新分组名称" maxLength={100} />
          <Button type="submit" variant="secondary" disabled={!newName.trim()}>
            <PlusIcon />
            添加
          </Button>
        </form>

        <ConfirmDialog
          open={deleting !== null}
          title="删除分组"
          description={`确定删除“${deleting?.name ?? ""}”吗？其中的网站会移到默认分组。`}
          onCancel={() => setDeleting(null)}
          onConfirm={() => {
            if (deleting) void mutate(() => api.deleteGroup(deleting.id));
            setDeleting(null);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}

/** Renames on Enter or blur; Escape reverts. */
function GroupNameInput({ group }: { group: Group }) {
  const { mutate } = useSnapshot();
  const [name, setName] = useState(group.name);
  const [synced, setSynced] = useState(group.name);
  if (synced !== group.name) {
    setSynced(group.name);
    setName(group.name);
  }

  async function commit() {
    if (name.trim() === group.name) return setName(group.name);
    if (!(await mutate(() => api.renameGroup(group.id, name)))) setName(group.name);
  }

  return (
    <Input
      aria-label="分组名称"
      value={name}
      maxLength={100}
      className="h-9"
      onChange={(e) => setName(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
        if (e.key === "Escape") {
          e.preventDefault();
          setName(group.name);
        }
      }}
    />
  );
}
