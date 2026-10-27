// Post-outage Entry (OP-14, BR-CS-08, G-21): a paper Bill written during an outage is re-entered
// with its original times, items, Payments and serving Crew. The API prices it, keeps it out of
// the Kitchen and marks it post_outage; when the Payments fall short it answers with its total.
import { ApiError } from '@sushi-latte/api-client';
import { Button, Card, Field, Input, Select, cn, formatRupiah } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { PlusIcon, Trash2Icon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText } from '../admin/api';
import { floorQuery, staffMenuQuery } from '../waiter/api';
import { cashier } from './api';

type Method = 'cash' | 'qris' | 'card';
type ItemRow = { key: string; productId: string; variantId: string; quantity: number };
type PayRow = { key: string; method: Method; amount: string };

/** A datetime-local value in the device's zone (the outlet's, WIB). */
function local(ms: number) {
  const d = new Date(ms - new Date(ms).getTimezoneOffset() * 60_000);
  return d.toISOString().slice(0, 16);
}

export function OutagePage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const menu = useQuery(staffMenuQuery);
  const floor = useQuery(floorQuery);
  const [where, setWhere] = useState<'table' | 'takeaway'>('table');
  const [tableId, setTableId] = useState('');
  const [name, setName] = useState('');
  const [openedAt, setOpenedAt] = useState(() => local(Date.now() - 90 * 60_000));
  const [closedAt, setClosedAt] = useState(() => local(Date.now() - 60 * 60_000));
  const [servedBy, setServedBy] = useState('');
  const [items, setItems] = useState<ItemRow[]>([]);
  const [payments, setPayments] = useState<PayRow[]>([{ key: crypto.randomUUID(), method: 'cash', amount: '' }]);
  const [systemTotal, setSystemTotal] = useState<number | null>(null);
  const products = (menu.data?.products ?? []).filter((p) => p.status !== 'archived');

  const submit = useMutation({
    mutationFn: () => {
      const closed = new Date(closedAt).getTime();
      return cashier.outage({
        tableId: where === 'table' ? tableId : undefined,
        takeawayName: where === 'takeaway' ? name : undefined,
        openedAt: new Date(openedAt).getTime(),
        closedAt: closed,
        servedBy,
        items: items.map((i) => {
          // Required single-choice Modifiers take their first option, as on the paper default.
          const p = products.find((x) => x.id === i.productId);
          const modifierIds = (p?.modifierGroups ?? []).filter((g) => g.minSelect > 0).flatMap((g) => g.modifiers.slice(0, g.minSelect).map((m) => m.id!));
          return { variantId: i.variantId, quantity: i.quantity, modifierIds, note: '' };
        }),
        payments: payments.filter((p) => Number(p.amount) > 0).map((p) => ({ method: p.method, amount: Number(p.amount), takenAt: closed })),
      });
    },
    onSuccess: (res) => {
      toast.success(t('cashier.outage.done'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
      void navigate({ to: '/cashier', search: { bill: res.billId } });
    },
    onError: (e) => {
      if (e instanceof ApiError && typeof e.extra.total === 'number') setSystemTotal(e.extra.total);
      toast.error(errorText(e));
    },
  });

  const addItem = (productId: string) => {
    const p = products.find((x) => x.id === productId);
    const v = p?.variants.find((x) => x.isDefault) ?? p?.variants[0];
    if (p && v) setItems([...items, { key: crypto.randomUUID(), productId, variantId: v.id!, quantity: 1 }]);
  };
  const ready = (where === 'table' ? !!tableId : !!name.trim()) && servedBy.trim() && items.length > 0 && payments.some((p) => Number(p.amount) > 0);

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto flex max-w-3xl flex-col gap-5 p-6">
        <div>
          <h1 className="text-xl font-bold">{t('cashier.outage.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('cashier.outage.hint')}</p>
        </div>

        <Card className="grid grid-cols-2 gap-4 p-4">
          <fieldset className="col-span-2">
            <legend className="mb-1.5 text-sm font-semibold">{t('cashier.outage.where')}</legend>
            <div className="grid grid-cols-2 gap-2" role="radiogroup">
              {(['table', 'takeaway'] as const).map((w) => (
                <button key={w} type="button" role="radio" aria-checked={where === w} onClick={() => setWhere(w)}
                  className={cn('h-12 rounded-control border font-semibold', where === w ? 'border-foreground bg-foreground text-background' : 'border-border')}>
                  {w === 'table' ? t('cashier.outage.table') : t('cashier.bills.takeaway')}
                </button>
              ))}
            </div>
          </fieldset>
          {where === 'table' ? (
            <Field label={t('cashier.outage.table')} htmlFor="outage-table">
              <Select id="outage-table" value={tableId} onChange={(e) => setTableId(e.target.value)}>
                <option value="">—</option>
                {floor.data?.map((x) => <option key={x.id} value={x.id}>{x.label}</option>)}
              </Select>
            </Field>
          ) : (
            <Field label={t('cashier.outage.takeawayName')} htmlFor="outage-name">
              <Input id="outage-name" value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
          )}
          <Field label={t('cashier.outage.servedBy')} htmlFor="outage-crew">
            <Input id="outage-crew" value={servedBy} onChange={(e) => setServedBy(e.target.value.toUpperCase())} placeholder="W001" className="font-mono" />
          </Field>
          <Field label={t('cashier.outage.openedAt')} htmlFor="outage-opened">
            <Input id="outage-opened" type="datetime-local" value={openedAt} onChange={(e) => setOpenedAt(e.target.value)} />
          </Field>
          <Field label={t('cashier.outage.closedAt')} htmlFor="outage-closed">
            <Input id="outage-closed" type="datetime-local" value={closedAt} onChange={(e) => setClosedAt(e.target.value)} />
          </Field>
        </Card>

        <Card className="flex flex-col gap-3 p-4">
          <h2 className="font-bold">{t('cashier.outage.items')}</h2>
          <ul className="flex flex-col gap-2">
            {items.map((i) => {
              const p = products.find((x) => x.id === i.productId)!;
              return (
                <li key={i.key} className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 truncate font-semibold">{p.nameId}</span>
                  {p.variants.length > 1 && (
                    <Select aria-label={p.nameId} className="w-40" value={i.variantId} onChange={(e) => setItems(items.map((x) => (x.key === i.key ? { ...x, variantId: e.target.value } : x)))}>
                      {p.variants.map((v) => <option key={v.id} value={v.id}>{v.nameId}</option>)}
                    </Select>
                  )}
                  <Input aria-label={t('cashier.outage.quantity')} type="number" min={1} max={50} className="tabular w-20 text-right" value={i.quantity}
                    onChange={(e) => setItems(items.map((x) => (x.key === i.key ? { ...x, quantity: Math.max(1, Math.min(50, Number(e.target.value) || 1)) } : x)))} />
                  <Button size="icon" variant="ghost" aria-label={t('cashier.outage.remove')} onClick={() => setItems(items.filter((x) => x.key !== i.key))}><Trash2Icon aria-hidden /></Button>
                </li>
              );
            })}
          </ul>
          <Select aria-label={t('cashier.outage.addItem')} value="" onChange={(e) => addItem(e.target.value)}>
            <option value="">+ {t('cashier.outage.addItem')}</option>
            {menu.data?.categories.map((c) => (
              <optgroup key={c.id} label={c.nameId}>
                {products.filter((p) => p.categoryId === c.id).map((p) => <option key={p.id} value={p.id}>{p.nameId}</option>)}
              </optgroup>
            ))}
          </Select>
        </Card>

        <Card className="flex flex-col gap-3 p-4">
          <h2 className="font-bold">{t('cashier.outage.payments')}</h2>
          {payments.map((p) => (
            <div key={p.key} className="flex items-center gap-2">
              <Select aria-label={t('cashier.outage.method')} className="w-36" value={p.method} onChange={(e) => setPayments(payments.map((x) => (x.key === p.key ? { ...x, method: e.target.value as Method } : x)))}>
                {(['cash', 'qris', 'card'] as const).map((m) => <option key={m} value={m}>{t(`waiter.pay.${m}`)}</option>)}
              </Select>
              <Input aria-label={t('cashier.outage.amount')} inputMode="numeric" className="tabular flex-1 text-right font-bold" value={p.amount ? formatRupiah(Number(p.amount)) : ''}
                onChange={(e) => setPayments(payments.map((x) => (x.key === p.key ? { ...x, amount: e.target.value.replace(/\D/g, '').slice(0, 10) } : x)))} />
              {payments.length > 1 && (
                <Button size="icon" variant="ghost" aria-label={t('cashier.outage.remove')} onClick={() => setPayments(payments.filter((x) => x.key !== p.key))}><Trash2Icon aria-hidden /></Button>
              )}
            </div>
          ))}
          <Button variant="outline" className="self-start" onClick={() => setPayments([...payments, { key: crypto.randomUUID(), method: 'qris', amount: '' }])}>
            <PlusIcon aria-hidden />{t('cashier.outage.addPayment')}
          </Button>
          {systemTotal !== null && (
            <p className="flex justify-between rounded-control bg-[var(--sl-color-feedback-warning-bg)] px-3 py-2 text-sm font-semibold text-[var(--sl-color-feedback-warning-fg)]" role="status">
              <span>{t('cashier.outage.total')}</span><span className="tabular">{formatRupiah(systemTotal)}</span>
            </p>
          )}
        </Card>

        <Button size="xl" disabled={!ready || submit.isPending} onClick={() => submit.mutate()}>{t('cashier.outage.submit')}</Button>
      </div>
    </div>
  );
}
