// Cashier split view (LAYOUT.md §3): Bills list (40 %) · Bill detail with the PaymentPad (60 %).
// The detail is the Waiter's BillPage, so the Cashier has every Waiter Bill and Payment
// capability (BR-CS-01); closed Bills add the cash Refund (BR-CS-03, BR-R-20).
import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, cn, formatRupiah, formatTime } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { BellRingIcon, HourglassIcon, SearchIcon, ShoppingBagIcon, Undo2Icon } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText, settingsQuery } from '../admin/api';
import { PinOverrideDialog } from '../components/pin-override';
import { billQuery } from '../waiter/api';
import { BillPage } from '../waiter/bill';
import { cashier, cashierBillsQuery, type BillBrief } from './api';

export function CashierBillsPage({ me, billId }: { me: Principal; billId?: string }) {
  const { t } = useTranslation();
  const bills = useQuery(cashierBillsQuery);
  const [query, setQuery] = useState('');
  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const all = (bills.data ?? []).filter(
      (b) => !q || b.number.toLowerCase().includes(q) || b.tables.some((x) => x.label.toLowerCase().includes(q)) || b.customers.some((c) => c.toLowerCase().includes(q)),
    );
    return {
      open: all.filter((b) => b.status === 'open' || b.status === 'settling'),
      closed: all.filter((b) => b.status === 'closed' || b.status === 'unpaid').reverse(),
    };
  }, [bills.data, query]);

  return (
    <div className="grid h-full grid-cols-[2fr_3fr]">
      <aside className="flex min-h-0 flex-col border-r border-border-subtle">
        <div className="flex gap-2 p-3">
          <div className="relative flex-1">
            <SearchIcon className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
            <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t('cashier.bills.search')} aria-label={t('cashier.bills.search')} className="pl-9" />
          </div>
          <Button asChild>
            <Link to="/cashier/takeaway"><ShoppingBagIcon aria-hidden />{t('cashier.bills.newTakeaway')}</Link>
          </Button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-6">
          <BillGroup title={t('cashier.bills.open')} bills={groups.open} selected={billId} />
          <BillGroup title={t('cashier.bills.closed')} bills={groups.closed} selected={billId} />
          {bills.data && bills.data.length === 0 && <p className="py-16 text-center text-muted-foreground">{t('cashier.bills.empty')}</p>}
        </div>
      </aside>
      <section className="min-h-0 overflow-y-auto px-6 py-4" aria-live="polite">
        {billId ? (
          <BillPage key={billId} me={me} billId={billId} module="cashier" extra={<RefundCard billId={billId} />} />
        ) : (
          <p className="grid h-full place-items-center text-muted-foreground">{t('cashier.bills.pick')}</p>
        )}
      </section>
    </div>
  );
}

function BillGroup({ title, bills, selected }: { title: string; bills: BillBrief[]; selected?: string }) {
  const { t } = useTranslation();
  if (bills.length === 0) return null;
  return (
    <section className="mt-2 flex flex-col gap-1.5">
      <h2 className="px-1 pt-2 text-xs font-bold uppercase tracking-[0.12em] text-muted-foreground">{title} · {bills.length}</h2>
      <ul className="flex flex-col gap-1.5">
        {bills.map((b) => {
          const done = b.status === 'closed' || b.status === 'unpaid';
          return (
            <li key={b.id}>
              <Link to="/cashier" search={{ bill: b.id }} data-bill={b.number}
                className={cn('flex min-h-16 items-center gap-3 rounded-control border px-3 py-2',
                  selected === b.id ? 'border-foreground bg-accent' : 'border-border-subtle bg-card hover:bg-accent', done && 'text-muted-foreground')}>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-semibold text-foreground">{b.tables.map((x) => x.label).join(' + ') || t('cashier.bills.takeaway')}</p>
                  <p className="truncate text-sm"><span className="font-mono">{b.number}</span> · {formatTime(b.openedAt)}{b.customers.length > 0 && ` · ${b.customers.join(', ')}`}</p>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">
                  <span className="tabular font-semibold text-foreground">{formatRupiah(b.total)}</span>
                  <span className="flex gap-1">
                    {b.status === 'settling' && <Chip feedback="warning" icon={<HourglassIcon aria-hidden />}>{t('waiter.floor.settling')}</Chip>}
                    {b.status === 'unpaid' && <Chip feedback="danger">{t('cashier.bills.unpaid')}</Chip>}
                    {b.unclaimed > 0 && <Chip tone="pending">{b.unclaimed}</Chip>}
                    {b.readyItems > 0 && <Chip tone="ready" icon={<BellRingIcon aria-hidden />}>{b.readyItems}</Chip>}
                  </span>
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/** Cash Refund on a closed Bill: within the cap an Outlet Admin approves, above it Management (BR-R-20). */
function RefundCard({ billId }: { billId: string }) {
  const { t } = useTranslation();
  const bill = useQuery(billQuery(billId));
  const settings = useQuery(settingsQuery);
  const [open, setOpen] = useState(false);
  if (!bill.data || bill.data.status !== 'closed') return null;
  const b = bill.data;
  const refundable = b.billing.paid - b.refunded;
  return (
    <Card className="flex items-center gap-3 p-4">
      <div className="flex-1 text-sm">
        <p className="font-semibold">{t('cashier.refund.paid', { amount: formatRupiah(b.billing.paid) })}</p>
        {b.refunded > 0 && <p className="text-muted-foreground">{t('cashier.refund.refunded', { amount: formatRupiah(b.refunded) })}</p>}
      </div>
      <Button variant="outline" disabled={refundable <= 0} onClick={() => setOpen(true)}><Undo2Icon aria-hidden />{t('cashier.refund.action')}</Button>
      {open && settings.data && (
        <RefundDialog billId={billId} number={b.number} max={refundable} cap={settings.data.pricing.refundCap} onClose={() => setOpen(false)} />
      )}
    </Card>
  );
}

function RefundDialog({ billId, number, max, cap, onClose }: { billId: string; number: string; max: number; cap: number; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [amount, setAmount] = useState('');
  const [reason, setReason] = useState('');
  const [askPin, setAskPin] = useState(false);
  const value = Number(amount || 0);
  const above = value > cap;
  const refund = useMutation({
    mutationFn: (token: string) => cashier.refund(billId, value, reason, token),
    onSuccess: () => {
      toast.success(t('cashier.refund.done'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
      onClose();
    },
    onError: (e) => {
      setAskPin(false);
      toast.error(errorText(e));
    },
  });
  return (
    <>
      <Dialog open={!askPin} onOpenChange={(o) => !o && onClose()} title={t('cashier.refund.title', { bill: number })} description={t('cashier.refund.hint', { cap: formatRupiah(cap) })}
        footer={
          <>
            <Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button>
            <Button variant="danger" disabled={value <= 0 || value > max || !reason.trim()} onClick={() => setAskPin(true)}>{t('cashier.refund.action')} · {formatRupiah(value)}</Button>
          </>
        }>
        <div className="flex flex-col gap-3">
          <Field label={t('cashier.refund.amount')} htmlFor="refund-amount" hint={above ? t('cashier.refund.aboveCap') : t('cashier.refund.max', { amount: formatRupiah(max) })}>
            <Input id="refund-amount" inputMode="numeric" className="tabular text-right text-lg font-bold" value={amount ? formatRupiah(Number(amount)) : ''}
              onChange={(e) => setAmount(e.target.value.replace(/\D/g, '').slice(0, 9))} />
          </Field>
          <Field label={t('cashier.refund.reason')} htmlFor="refund-reason">
            <Input id="refund-reason" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Pesanan salah" />
          </Field>
        </div>
      </Dialog>
      <PinOverrideDialog action={above ? 'refund_above_cap' : 'refund_within_cap'} open={askPin} onClose={() => setAskPin(false)} onApproved={(tok) => refund.mutate(tok)} />
    </>
  );
}
