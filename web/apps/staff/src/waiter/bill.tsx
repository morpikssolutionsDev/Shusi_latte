// BillSummary + OrderReviewList (COMPONENTS.md §3): review with the Customer and confirm (OP-03),
// Cancel with a reason (OP-07), Serve, Remake and Void under a PIN Override (OP-06). The Cashier
// shows the same page in its right pane (BR-CS-01); `module` picks where its links lead.
import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, Select, Textarea, cn, formatRupiah } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowLeftIcon, BanIcon, CheckIcon, PlusIcon, RotateCcwIcon, UtensilsIcon, XIcon } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { catalogQuery, errorText } from '../admin/api';
import { PinOverrideDialog } from '../components/pin-override';
import { actions, billQuery, type CancelReason, type ItemView, type StaffOrder } from './api';
import { ItemList, useMinutesAgo } from './notifications';
import { BillActions } from './pay';

const REASONS: CancelReason[] = ['customer_request', 'wrong_order', 'out_of_stock', 'other'];

type Pending = { kind: 'cancel' | 'remake' | 'void'; item: ItemView } | null;
export type StaffModule = 'waiter' | 'cashier';

export function BillPage({ me, billId, module = 'waiter', extra }: { me: Principal; billId: string; module?: StaffModule; extra?: ReactNode }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const bill = useQuery(billQuery(billId));
  const catalog = useQuery(catalogQuery);
  const ago = useMinutesAgo();
  const [pending, setPending] = useState<Pending>(null);
  const refresh = () => void qc.invalidateQueries({ queryKey: ['waiter'] });
  const run = (fn: () => Promise<unknown>, done?: string) =>
    fn().then(
      () => {
        if (done) toast.success(done);
        refresh();
      },
      (e) => {
        toast.error(errorText(e));
        refresh();
      },
    );
  if (!bill.data) return null;
  const b = bill.data;
  const tagName = (id: string) => catalog.data?.dietaryTags.find((x) => x.id === id)?.nameId ?? id;
  const tables = b.tables.map((x) => x.label).join(' + ');
  const takeaway = b.type === 'takeaway';
  const live = b.status === 'open' || b.status === 'settling';

  return (
    <div className="flex flex-col gap-5">
      <header className="flex items-center gap-3">
        {module === 'waiter' && (
          <Link to="/waiter/tables" className="grid size-10 place-items-center rounded-control hover:bg-accent" aria-label={t('waiter.tabs.tables')}>
            <ArrowLeftIcon className="size-5" aria-hidden />
          </Link>
        )}
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-bold">{tables}</h1>
          <p className="text-sm text-muted-foreground"><span className="font-mono">{b.number}</span> · {ago(b.openedAt)}</p>
        </div>
        {b.status === 'settling' && <Chip feedback="warning">{t('waiter.floor.settling')}</Chip>}
        {b.source === 'post_outage' && <Chip feedback="info">{t('cashier.outage.title')}</Chip>}
        {live && !takeaway && (
          <Button asChild variant="secondary">
            {module === 'waiter' ? (
              <Link to="/waiter/bills/$billId/new" params={{ billId }}><PlusIcon aria-hidden />{t('waiter.floor.newOrder')}</Link>
            ) : (
              <Link to="/cashier/bills/$billId/new" params={{ billId }}><PlusIcon aria-hidden />{t('waiter.floor.newOrder')}</Link>
            )}
          </Button>
        )}
      </header>

      {b.customers.length > 0 && (
        <Card className="flex flex-col gap-2 p-4">
          <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('waiter.bill.customers')}</h2>
          <ul className="flex flex-col gap-1.5">
            {b.customers.map((c) => (
              <li key={c.id} className="flex flex-wrap items-center gap-2">
                <span className="font-semibold">{c.name}</span>
                {c.restrictionTagIds.length > 0 && (
                  <span className="text-sm text-[var(--sl-color-dietary-warning-fg)]">
                    {t('waiter.bill.avoids')}: {c.restrictionTagIds.map(tagName).join(', ')}
                  </span>
                )}
                {c.dietaryNote && <span className="text-sm italic text-muted-foreground">“{c.dietaryNote}”</span>}
              </li>
            ))}
          </ul>
        </Card>
      )}

      <section className="flex flex-col gap-3">
        {b.orders.length === 0 && <p className="py-8 text-center text-muted-foreground">{t('waiter.bill.noOrders')}</p>}
        {[...b.orders].reverse().map((o) => (
          <OrderBlock key={o.id} order={o} me={me} module={module} takeaway={takeaway && live} run={run} onItem={(kind, item) => setPending({ kind, item })} />
        ))}
      </section>

      <Card className="flex flex-col gap-1.5 p-4">
        <h2 className="mb-1 text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('waiter.bill.totals')}</h2>
        <Row label={t('waiter.bill.subtotal')} value={b.totals.subtotal} />
        {b.totals.itemDiscount > 0 && <Row label={t('waiter.bill.itemDiscount')} value={-b.totals.itemDiscount} />}
        {b.totals.billDiscount > 0 && <Row label={t('waiter.bill.billDiscount')} value={-b.totals.billDiscount} />}
        <Row label={t('waiter.bill.service')} value={b.totals.service} />
        <Row label={t('waiter.bill.tax')} value={b.totals.tax} />
        <Row label={t('waiter.bill.total')} value={b.totals.total} strong />
      </Card>

      {/* The Cashier pane pins the balance and payment actions to its bottom edge (LAYOUT.md §3). */}
      <div className={module === 'cashier' ? 'sticky bottom-0 z-10 shadow-lg' : undefined}>
        <BillActions bill={b} />
      </div>
      {extra}

      <ItemActionDialog pending={pending} onClose={() => setPending(null)} onDone={refresh} />
    </div>
  );
}

function Row({ label, value, strong }: { label: string; value: number; strong?: boolean }) {
  return (
    <div className={cn('flex justify-between', strong ? 'border-t border-border-subtle pt-2 text-lg font-bold' : 'text-sm')}>
      <span>{label}</span>
      <span className="tabular">{value < 0 ? `−${formatRupiah(-value)}` : formatRupiah(value)}</span>
    </div>
  );
}

function OrderBlock({ order: o, me, module, takeaway, run, onItem }: {
  order: StaffOrder;
  me: Principal;
  module: StaffModule;
  /** An unpaid Takeaway: its Order is held until paid, so it is never confirmed by hand (TR-PAY-04). */
  takeaway: boolean;
  run: (fn: () => Promise<unknown>, done?: string) => Promise<void>;
  onItem: (kind: 'cancel' | 'remake' | 'void', item: ItemView) => void;
}) {
  const { t } = useTranslation();
  const mine = o.claimedBy?.id !== undefined && o.claimedBy.crewId === me.crewId;
  const reviewing = o.status === 'claimed' && mine;
  return (
    <Card className={cn('flex flex-col gap-3 p-4', reviewing && 'border-[var(--sl-color-status-pending-border)]')} data-order={o.seq}>
      <header className="flex flex-wrap items-center gap-2">
        <h2 className="font-bold">{t('waiter.bill.order', { n: o.seq })}</h2>
        <span className="text-sm text-muted-foreground">
          {o.customerName || '—'} · {o.source === 'client_menu' ? t('waiter.bill.viaMenu') : t('waiter.bill.viaStaff', { name: o.claimedBy?.name ?? '' })}
        </span>
        {o.claimedBy && <span className="text-sm text-muted-foreground">· {t('waiter.notify.claimedBy', { name: o.claimedBy.name.split(' ')[0] })}</span>}
        <Chip tone={o.status === 'claimed' ? 'pending' : (o.status as 'confirmed')} className="ml-auto">{t(`waiter.status.${o.status}`)}</Chip>
      </header>
      {reviewing && <p className="text-sm text-muted-foreground">{takeaway ? t('cashier.takeaway.held') : t('waiter.bill.reviewHint')}</p>}
      <ItemList
        items={o.items}
        actions={(i) => (
          <span className="flex shrink-0 gap-1">
            {(i.status === 'pending' || i.status === 'confirmed') && (o.status !== 'pending' || mine) && (
              <Button size="icon" variant="ghost" aria-label={`${t('waiter.bill.cancel')} ${i.productName.id}`} onClick={() => onItem('cancel', i)}><XIcon aria-hidden /></Button>
            )}
            {i.status === 'ready' && (
              <Button size="sm" onClick={() => void run(() => actions.serve([i.id]), t('waiter.notify.served'))}><UtensilsIcon aria-hidden />{t('waiter.bill.serve')}</Button>
            )}
            {i.status === 'served' && i.unitPrice > 0 && (
              <Button size="icon" variant="ghost" aria-label={`${t('waiter.bill.remake')} ${i.productName.id}`} onClick={() => onItem('remake', i)}><RotateCcwIcon aria-hidden /></Button>
            )}
            {(i.status === 'ready' || i.status === 'served') && (
              <Button size="icon" variant="ghost" aria-label={`${t('waiter.bill.void')} ${i.productName.id}`} onClick={() => onItem('void', i)}><BanIcon aria-hidden /></Button>
            )}
          </span>
        )}
      />
      <footer className="flex flex-wrap items-center gap-2">
        <span className="tabular text-sm font-semibold">{formatRupiah(o.subtotal)}</span>
        <span className="flex-1" />
        {o.status === 'pending' && (
          <Button size="lg" onClick={() => void run(() => actions.claim(o.id), t('waiter.notify.claimed'))}>{t('waiter.bill.claim')}</Button>
        )}
        {o.claimedBy && !mine && o.status !== 'cancelled' && o.status !== 'served' && (
          <Button variant="secondary" onClick={() => void run(() => actions.takeover(o.id))}>{t('waiter.bill.takeover')}</Button>
        )}
        {reviewing && (
          <>
            <Button asChild variant="secondary">
              {module === 'waiter' ? (
                <Link to="/waiter/orders/$orderId/add" params={{ orderId: o.id }} search={{ billId: o.billId }}><PlusIcon aria-hidden />{t('waiter.bill.addItems')}</Link>
              ) : (
                <Link to="/cashier/orders/$orderId/add" params={{ orderId: o.id }} search={{ billId: o.billId }}><PlusIcon aria-hidden />{t('waiter.bill.addItems')}</Link>
              )}
            </Button>
            {!takeaway && (
              <Button size="lg" onClick={() => void run(() => actions.confirm(o.id), t('waiter.bill.confirmed'))}><CheckIcon aria-hidden />{t('waiter.bill.confirm')}</Button>
            )}
          </>
        )}
      </footer>
    </Card>
  );
}

function ItemActionDialog({ pending, onClose, onDone }: { pending: Pending; onClose: () => void; onDone: () => void }) {
  const { t } = useTranslation();
  const [reason, setReason] = useState<CancelReason>('customer_request');
  const [text, setText] = useState('');
  const [askPin, setAskPin] = useState(false);
  const close = () => {
    setReason('customer_request');
    setText('');
    setAskPin(false);
    onClose();
  };
  const exec = useMutation({
    mutationFn: async (token?: string) => {
      if (!pending) return;
      if (pending.kind === 'cancel') return actions.cancel(pending.item.id, reason, text);
      if (pending.kind === 'remake') return actions.remake(pending.item.id, text, token!);
      return actions.void(pending.item.id, text, token!);
    },
    onSuccess: () => {
      toast.success(t(pending?.kind === 'cancel' ? 'waiter.cancel.done' : pending?.kind === 'remake' ? 'waiter.remake.done' : 'waiter.void.done'));
      onDone();
      close();
    },
    onError: (e) => {
      toast.error(errorText(e));
      setAskPin(false);
    },
  });
  if (!pending) return null;
  const name = `${pending.item.quantity}× ${pending.item.productName.id}`;
  const needsPin = pending.kind !== 'cancel';
  return (
    <>
      <Dialog
        open={!askPin}
        onOpenChange={(o) => !o && close()}
        title={t(`waiter.${pending.kind}.title`, { name })}
        description={pending.kind === 'cancel' ? undefined : t(`waiter.${pending.kind}.hint`)}
        footer={
          <>
            <Button variant="secondary" onClick={close}>{t('admin.common.cancel')}</Button>
            <Button
              variant={pending.kind === 'void' ? 'danger' : 'primary'}
              disabled={exec.isPending || (pending.kind === 'cancel' ? reason === 'other' && !text.trim() : !text.trim())}
              onClick={() => (needsPin ? setAskPin(true) : exec.mutate(undefined))}
            >
              {t(`waiter.${pending.kind}.submit`)}
            </Button>
          </>
        }
      >
        {pending.kind === 'cancel' ? (
          <div className="flex flex-col gap-3">
            <Field label={t('waiter.cancel.reason')} htmlFor="cancel-reason" hint={reason === 'out_of_stock' ? t('waiter.cancel.outOfStockHint') : undefined}>
              <Select id="cancel-reason" value={reason} onChange={(e) => setReason(e.target.value as CancelReason)}>
                {REASONS.map((r) => <option key={r} value={r}>{t(`waiter.cancel.${r}`)}</option>)}
              </Select>
            </Field>
            <Field label={t('waiter.cancel.note')} htmlFor="cancel-note">
              <Textarea id="cancel-note" rows={2} value={text} onChange={(e) => setText(e.target.value)} />
            </Field>
          </div>
        ) : (
          <Field label={t(`waiter.${pending.kind}.reason`)} htmlFor="action-reason">
            <Input id="action-reason" value={text} onChange={(e) => setText(e.target.value)} placeholder={pending.kind === 'remake' ? 'Kurang pedas' : 'Jatuh saat disajikan'} />
          </Field>
        )}
      </Dialog>
      {needsPin && (
        <PinOverrideDialog
          action={pending.kind === 'remake' ? 'remake' : 'void'}
          open={askPin}
          onClose={() => setAskPin(false)}
          onApproved={(token) => exec.mutate(token)}
        />
      )}
    </>
  );
}

