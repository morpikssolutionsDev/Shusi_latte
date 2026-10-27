// Direct entry by a Waiter or Cashier (OP-04, BR-WT-03), and adding lines to a Claimed Order
// while reviewing it (OP-03 H3), and a Cashier's Takeaway (BR-CS-02), held until paid. Sold Out Variants are greyed; a ticked Dietary Restriction
// shows Dietary Warnings on the tiles exactly like the Client Menu (BR-R-D1).
import { Button, Card, Dialog, DietaryBadge, Field, Input, cn, dietaryIcons, formatRupiah } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { CheckIcon, MinusIcon, PlusIcon, SearchIcon, Trash2Icon } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText, type Product } from '../admin/api';
import { cashier } from '../cashier/api';
import { actions, staffMenuQuery, type CartItem } from './api';

type Line = CartItem & { key: string; product: Product };

export function EntryPage({ target, title, back, onCreated }: {
  target: { tableId: string } | { billId: string } | { orderId: string } | { takeaway: true };
  title: string;
  back: () => void;
  /** Takeaway only: opens the new Bill. */
  onCreated?: (billId: string) => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const menu = useQuery(staffMenuQuery);
  const [lines, setLines] = useState<Line[]>([]);
  const [category, setCategory] = useState<string>();
  const [query, setQuery] = useState('');
  const [name, setName] = useState('');
  const [restriction, setRestriction] = useState<string[]>([]);
  const [picking, setPicking] = useState<Product | null>(null);
  const isNew = !('orderId' in target);
  const takeaway = 'takeaway' in target;

  const products = useMemo(() => {
    const active = (menu.data?.products ?? []).filter((p) => p.status === 'active');
    const q = query.trim().toLowerCase();
    if (q) return active.filter((p) => p.nameId.toLowerCase().includes(q) || p.nameEn.toLowerCase().includes(q));
    const cat = category ?? menu.data?.categories[0]?.id;
    return active.filter((p) => p.categoryId === cat);
  }, [menu.data, category, query]);

  const submit = useMutation({
    mutationFn: async () => {
      const items = lines.map(({ key: _k, product: _p, ...l }) => l);
      if ('orderId' in target) return actions.addItems(target.orderId, items);
      if ('takeaway' in target) return cashier.takeaway({ customerName: name, restrictionTagIds: restriction, items });
      return actions.direct({ ...target, customerName: name, restrictionTagIds: restriction, items });
    },
    onSuccess: (res) => {
      toast.success(takeaway ? t('cashier.takeaway.created') : isNew ? t('waiter.entry.sent') : t('waiter.entry.added'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
      if (takeaway && res && 'billId' in res) onCreated?.(res.billId);
      else back();
    },
    onError: (e) => toast.error(errorText(e)),
  });

  const warn = (tagIds: string[]) => (menu.data?.dietaryTags ?? []).filter((x) => restriction.includes(x.id!) && tagIds.includes(x.id!));
  const add = (p: Product, variantId: string, modifierIds: string[], quantity: number, note: string) =>
    setLines((ls) => [...ls, { key: crypto.randomUUID(), product: p, variantId, modifierIds, quantity, note }]);

  const quickAdd = (p: Product) => {
    const available = p.variants.filter((v) => !v.soldOut);
    if (available.length === 1 && p.modifierGroups.every((g) => g.minSelect === 0)) add(p, available[0]!.id!, [], 1, '');
    else setPicking(p);
  };

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-bold">{title}</h1>
      {isNew && (
        <Card className="flex flex-col gap-3 p-4">
          <Field label={takeaway ? t('cashier.takeaway.name') : t('waiter.entry.name')} htmlFor="entry-name" hint={takeaway ? t('cashier.takeaway.hint') : undefined}>
            <Input id="entry-name" value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <fieldset>
            <legend className="mb-1.5 text-sm font-semibold">{t('waiter.entry.restriction')}</legend>
            <div className="flex flex-wrap gap-1.5">
              {menu.data?.dietaryTags.filter((x) => x.active).map((x) => {
                const on = restriction.includes(x.id!);
                const Icon = dietaryIcons[x.code];
                return (
                  <button key={x.id} type="button" aria-pressed={on} onClick={() => setRestriction(on ? restriction.filter((r) => r !== x.id) : [...restriction, x.id!])}
                    className={cn('inline-flex h-10 items-center gap-1.5 rounded-chip border px-3 text-sm font-semibold',
                      on ? 'border-[var(--sl-color-dietary-warning-border)] bg-[var(--sl-color-dietary-warning-bg)] text-[var(--sl-color-dietary-warning-fg)]' : 'border-border bg-card')}>
                    {Icon && <Icon className="size-4" aria-hidden />}
                    {x.nameId}
                  </button>
                );
              })}
            </div>
          </fieldset>
        </Card>
      )}

      <div className="relative">
        <SearchIcon className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
        <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t('waiter.entry.search')} className="pl-9" aria-label={t('waiter.entry.search')} />
      </div>
      {!query && (
        <div className="flex gap-1.5 overflow-x-auto pb-1">
          {menu.data?.categories.filter((c) => c.active).map((c) => {
            const on = (category ?? menu.data?.categories[0]?.id) === c.id;
            return (
              <button key={c.id} type="button" onClick={() => setCategory(c.id)} aria-pressed={on}
                className={cn('h-10 shrink-0 rounded-control px-4 text-sm font-semibold', on ? 'bg-foreground text-background' : 'bg-sunken')}>
                {c.nameId}
              </button>
            );
          })}
        </div>
      )}
      <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {products.map((p) => {
          const soldOut = p.variants.every((v) => v.soldOut);
          const w = warn(p.dietaryTagIds);
          return (
            <li key={p.id}>
              <button type="button" disabled={soldOut} onClick={() => quickAdd(p)} data-product={p.nameEn}
                className={cn('flex h-full min-h-20 w-full flex-col items-start gap-1 rounded-card border border-border bg-card p-3 text-left', soldOut && 'opacity-50')}>
                <span className="font-semibold leading-tight">{p.nameId}</span>
                <span className="tabular text-sm text-muted-foreground">{soldOut ? t('admin.nav.soldOut') : formatRupiah(Math.min(...p.variants.filter((v) => !v.soldOut).map((v) => v.price)))}</span>
                {w.length > 0 && <DietaryBadge>{w.map((x) => x.nameId).join(', ')}</DietaryBadge>}
              </button>
            </li>
          );
        })}
      </ul>

      <Card className="sticky bottom-20 flex flex-col gap-2 p-4 shadow-lg">
        {lines.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('waiter.entry.empty')}</p>
        ) : (
          <ul className="flex max-h-48 flex-col divide-y divide-border-subtle overflow-y-auto">
            {lines.map((l) => {
              const v = l.product.variants.find((x) => x.id === l.variantId);
              const mods = l.product.modifierGroups.flatMap((g) => g.modifiers).filter((m) => l.modifierIds?.includes(m.id!));
              const w = warn([...l.product.dietaryTagIds, ...mods.flatMap((m) => m.dietaryTagIds)]);
              return (
                <li key={l.key} className="flex items-center gap-2 py-2">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-semibold">{l.product.nameId}{l.product.variants.length > 1 && ` · ${v?.nameId}`}</p>
                    {mods.length > 0 && <p className="text-xs text-muted-foreground">{mods.map((m) => m.nameId).join(', ')}</p>}
                    {w.length > 0 && <DietaryBadge className="mt-1">{w.map((x) => x.nameId).join(', ')}</DietaryBadge>}
                  </div>
                  <Button size="icon" variant="ghost" aria-label="−1" onClick={() => setLines(lines.map((x) => (x.key === l.key ? { ...x, quantity: Math.max(1, x.quantity - 1) } : x)))}><MinusIcon aria-hidden /></Button>
                  <span className="tabular w-6 text-center font-bold">{l.quantity}</span>
                  <Button size="icon" variant="ghost" aria-label="+1" onClick={() => setLines(lines.map((x) => (x.key === l.key ? { ...x, quantity: Math.min(20, x.quantity + 1) } : x)))}><PlusIcon aria-hidden /></Button>
                  <Button size="icon" variant="ghost" aria-label={t('waiter.bill.cancel')} onClick={() => setLines(lines.filter((x) => x.key !== l.key))}><Trash2Icon aria-hidden /></Button>
                </li>
              );
            })}
          </ul>
        )}
        <div className="flex items-center gap-3">
          <span className="tabular flex-1 text-lg font-bold">{lines.reduce((n, l) => n + l.quantity, 0)} item</span>
          <Button variant="secondary" onClick={back}>{t('admin.common.cancel')}</Button>
          <Button size="lg" disabled={!lines.length || submit.isPending || (takeaway && !name.trim())} onClick={() => submit.mutate()}>
            <CheckIcon aria-hidden />
            {takeaway ? t('cashier.takeaway.submit') : isNew ? t('waiter.entry.submit') : t('waiter.entry.add')}
          </Button>
        </div>
      </Card>

      {picking && <PickDialog product={picking} onClose={() => setPicking(null)} onAdd={(v, m, q, n) => { add(picking, v, m, q, n); setPicking(null); }} />}
    </div>
  );
}

function PickDialog({ product: p, onClose, onAdd }: { product: Product; onClose: () => void; onAdd: (variantId: string, modifierIds: string[], quantity: number, note: string) => void }) {
  const { t } = useTranslation();
  const [variantId, setVariantId] = useState(p.variants.find((v) => v.isDefault && !v.soldOut)?.id ?? p.variants.find((v) => !v.soldOut)?.id);
  const [picked, setPicked] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(p.modifierGroups.map((g) => [g.id!, g.minSelect === 1 && g.maxSelect === 1 && g.modifiers[0] ? [g.modifiers[0].id!] : []])));
  const [qty, setQty] = useState(1);
  const [note, setNote] = useState('');
  const unmet = p.modifierGroups.some((g) => (picked[g.id!]?.length ?? 0) < g.minSelect);
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={p.nameId}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button>
        <Button disabled={!variantId || unmet} onClick={() => onAdd(variantId!, Object.values(picked).flat(), qty, note.trim())}>{t('waiter.entry.add')}</Button>
      </>}>
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap gap-1.5" role="radiogroup">
          {p.variants.map((v) => (
            <button key={v.id} type="button" role="radio" aria-checked={v.id === variantId} disabled={v.soldOut} onClick={() => setVariantId(v.id)}
              className={cn('h-11 rounded-control border px-3 text-sm font-semibold', v.id === variantId ? 'border-foreground bg-foreground text-background' : 'border-border', v.soldOut && 'line-through opacity-50')}>
              {v.nameId} · {formatRupiah(v.price)}
            </button>
          ))}
        </div>
        {p.modifierGroups.map((g) => (
          <fieldset key={g.id}>
            <legend className="mb-1 text-sm font-semibold">{g.nameId} <span className="font-normal text-muted-foreground">({g.minSelect}–{g.maxSelect})</span></legend>
            <div className="flex flex-wrap gap-1.5">
              {g.modifiers.map((m) => {
                const on = picked[g.id!]?.includes(m.id!) ?? false;
                return (
                  <button key={m.id} type="button" aria-pressed={on}
                    onClick={() => setPicked((cur) => {
                      const now = cur[g.id!] ?? [];
                      if (g.maxSelect === 1) return { ...cur, [g.id!]: on && g.minSelect === 0 ? [] : [m.id!] };
                      return { ...cur, [g.id!]: on ? now.filter((x) => x !== m.id) : now.length < g.maxSelect ? [...now, m.id!] : now };
                    })}
                    className={cn('h-10 rounded-control border px-3 text-sm', on ? 'border-foreground bg-foreground text-background' : 'border-border')}>
                    {m.nameId}{m.price > 0 && ` +${formatRupiah(m.price)}`}
                  </button>
                );
              })}
            </div>
          </fieldset>
        ))}
        <Field label={t('waiter.cancel.note')} htmlFor="pick-note"><Input id="pick-note" value={note} onChange={(e) => setNote(e.target.value)} /></Field>
        <div className="flex items-center gap-2">
          <Button size="icon" variant="secondary" aria-label="−1" onClick={() => setQty(Math.max(1, qty - 1))}><MinusIcon aria-hidden /></Button>
          <span className="tabular w-8 text-center text-lg font-bold">{qty}</span>
          <Button size="icon" variant="secondary" aria-label="+1" onClick={() => setQty(Math.min(20, qty + 1))}><PlusIcon aria-hidden /></Button>
        </div>
      </div>
    </Dialog>
  );
}

export function useBack(to: string, params?: Record<string, string>) {
  const navigate = useNavigate();
  return () => void navigate({ to, params } as never);
}
