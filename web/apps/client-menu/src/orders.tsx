// OrderStatusTracker (COMPONENTS.md §2): the Customer's own Orders with per-item status and the
// cup-fill Estimated Wait (BR-CM-08, M9, M10); edit or Cancel before Claim (BR-CM-14).
import { api, idempotencyKey, unwrap } from '@sushi-latte/api-client';
import { Button, Chip, formatRupiah, imageUrl, statusIcons, type Tone } from '@sushi-latte/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowLeftIcon, Clock3Icon, PencilIcon, Trash2Icon, XIcon } from 'lucide-react';
import { AnimatePresence, motion } from 'motion/react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { problemOf, type CustomerView, type OrderView } from './api';
import { CupFill } from './cup';
import { BottomSheet, Stepper, WarningBadge, ease, useName } from './kit';
import { CallWaiterButton, DemoBanner, LanguageToggle } from './shell';

function useNow(ms = 1000) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(timer);
  }, [ms]);
  return now;
}

/** M9: latte level = elapsed ÷ Estimated Wait, never above 95 % until Ready. */
function progress(o: OrderView, now: number): number {
  switch (o.status) {
    case 'ready':
    case 'served':
      return 1;
    case 'pending':
    case 'claimed':
      return 0.06;
  }
  if (!o.confirmedAt || !o.estimatedWait) return 0.1;
  const total = (o.estimatedWait.asOf - o.confirmedAt) / 1000 + o.estimatedWait.seconds;
  if (total <= 0) return 0.95;
  return Math.min(0.95, Math.max(0.1, (now - o.confirmedAt) / 1000 / total));
}

export function OrdersScreen({ code, me }: { code: string; me: CustomerView }) {
  const { t } = useTranslation();
  const now = useNow();
  const live = me.orders.filter((o) => o.status !== 'cancelled');
  const focus = [...live].reverse().find((o) => o.status !== 'served') ?? live.at(-1);
  const [editing, setEditing] = useState<OrderView | null>(null);
  const [cancelling, setCancelling] = useState<OrderView | null>(null);

  // M10: vibrate once when the focused Order turns Ready.
  const lastStatus = useRef(focus?.status);
  useEffect(() => {
    if (focus?.status === 'ready' && lastStatus.current !== 'ready') navigator.vibrate?.([30, 60, 30]);
    lastStatus.current = focus?.status;
  }, [focus?.status]);

  const level = focus ? progress(focus, now) : 0;
  const headline = focus ? statusLine(focus.status, t) : t('orders.empty');

  return (
    <div className="mx-auto min-h-dvh max-w-[480px] pb-16">
      <DemoBanner />
      <header className="sticky top-0 z-30 flex items-center gap-2 bg-background/95 px-4 py-3 backdrop-blur">
        <Link to="/$code" params={{ code }} className="grid size-10 place-items-center rounded-full bg-sunken" aria-label={t('common.back')}>
          <ArrowLeftIcon className="size-5" aria-hidden />
        </Link>
        <div className="min-w-0 flex-1">
          <h1 className="font-display text-lg font-bold">{t('orders.title')}</h1>
          <p className="text-xs text-text-secondary">
            {me.bill.tables.map((x) => x.label).join(' + ')} · <span className="font-mono">{t('orders.yourBill', { number: me.bill.number })}</span>
          </p>
        </div>
        <LanguageToggle />
      </header>

      <section className="flex flex-col items-center gap-2 px-6 pt-4 text-center">
        <div
          role="progressbar"
          aria-label={t('orders.progress')}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(level * 100)}
          aria-valuetext={headline}
          className="w-44"
        >
          <CupFill level={level} ready={focus?.status === 'ready' || focus?.status === 'served'} />
        </div>
        <p className="font-display text-xl font-bold" aria-live="polite">{headline}</p>
        {focus?.estimatedWait && focus.status !== 'ready' && (
          <p className="tabular text-sm text-text-secondary">{t('orders.estimate', { n: focus.estimatedWait.minutes })}</p>
        )}
        {focus?.waiter && <WaiterIntro waiter={focus.waiter} />}
        <div className="mt-2 flex gap-2">
          <CallWaiterButton me={me} />
          <Link to="/$code" params={{ code }} className="inline-flex h-10 items-center rounded-full bg-primary px-4 text-sm font-semibold text-primary-foreground">
            {t('orders.backToMenu')}
          </Link>
        </div>
      </section>

      <ul className="mt-6 flex flex-col gap-3 px-4">
        <AnimatePresence initial={false}>
          {[...me.orders].reverse().map((o) => (
            <motion.li key={o.id} layout initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={ease.expressive}>
              <OrderCard order={o} now={now} onEdit={() => setEditing(o)} onCancel={() => setCancelling(o)} />
            </motion.li>
          ))}
        </AnimatePresence>
      </ul>

      <EditOrderSheet order={editing} customerId={me.customer.id} onClose={() => setEditing(null)} />
      <CancelOrderSheet order={cancelling} customerId={me.customer.id} onClose={() => setCancelling(null)} />
    </div>
  );
}

/** BR-CM-18: the claiming Waiter's first name and Crew portrait, initial when there is none. */
function WaiterIntro({ waiter }: { waiter: NonNullable<OrderView['waiter']> }) {
  const { t } = useTranslation();
  return (
    <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={ease.expressive}
      className="mt-2 flex w-full items-center gap-3 rounded-[20px] bg-card p-3 text-left shadow-[var(--sl-product-card-shadow)]">
      {waiter.imageKey ? (
        <img src={imageUrl(waiter.imageKey, 'sq320')} alt="" className="size-12 shrink-0 rounded-full object-cover" />
      ) : (
        <span aria-hidden className="grid size-12 shrink-0 place-items-center rounded-full bg-sunken font-display text-lg font-semibold">
          {waiter.firstName.charAt(0).toUpperCase()}
        </span>
      )}
      <p className="text-sm">{t('orders.waiterIntro', { name: waiter.firstName })}</p>
    </motion.div>
  );
}

function statusLine(status: OrderView['status'], t: (k: string) => string) {
  switch (status) {
    case 'pending':
      return t('orders.waitingWaiter');
    case 'claimed':
      return t('orders.claimed');
    case 'ready':
      return t('orders.ready');
    case 'served':
      return t('orders.served');
    case 'cancelled':
      return t('status.cancelled');
  }
  return t('orders.preparing');
}

const toneOf = (s: string): Tone => (s === 'claimed' ? 'pending' : (s as Tone));

/** BR-CM-16: an item repeats no status its Order already shows; a Claimed Order's items are still Pending. */
const differs = (item: string, order: OrderView['status']) => item !== order && !(order === 'claimed' && item === 'pending');

function OrderCard({ order: o, now, onEdit, onCancel }: { order: OrderView; now: number; onEdit: () => void; onCancel: () => void }) {
  const { t } = useTranslation();
  const { bi } = useName();
  const StatusIcon = statusIcons[toneOf(o.status) as keyof typeof statusIcons];
  return (
    <article className="rounded-[20px] bg-card p-4 shadow-[var(--sl-product-card-shadow)]" data-order={o.seq}>
      <header className="mb-2 flex items-center justify-between gap-2">
        <h2 className="font-semibold">{t('orders.order', { n: o.seq })}</h2>
        <Chip tone={toneOf(o.status)} icon={StatusIcon && <StatusIcon aria-hidden />}>{t(`status.${o.status}`)}</Chip>
      </header>
      <ReadyBy order={o} now={now} />
      <ul className="flex flex-col gap-2">
        {o.items.map((it) => {
          const ItemIcon = statusIcons[it.status as keyof typeof statusIcons];
          const gone = it.status === 'cancelled' || it.status === 'voided';
          return (
            <li key={it.id} className="flex flex-col gap-1">
              <div className="flex items-start gap-2 text-sm">
                <span className="tabular w-7 shrink-0 font-semibold">×{it.quantity}</span>
                <span className={gone ? 'flex-1 text-text-secondary line-through' : 'flex-1'}>
                  {bi(it.productName)}
                  {it.variantName.en && <span className="text-text-secondary"> · {bi(it.variantName)}</span>}
                  {it.modifiers.length > 0 && <span className="block text-xs text-text-secondary">{it.modifiers.map((m) => bi(m.name)).join(', ')}</span>}
                </span>
                {differs(it.status, o.status) ? (
                  <Chip tone={it.status as Tone} icon={ItemIcon && <ItemIcon aria-hidden />} className="h-6 px-2">{t(`status.${it.status}`)}</Chip>
                ) : (
                  <span className="tabular text-sm">{formatRupiah(it.lineTotal)}</span>
                )}
              </div>
              {it.warnings.length > 0 && !gone && <div className="flex flex-wrap gap-1 pl-9">{it.warnings.map((w) => <WarningBadge key={w.id} tag={w} />)}</div>}
            </li>
          );
        })}
      </ul>
      <footer className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-border-subtle pt-3">
        <span className="tabular whitespace-nowrap text-sm font-semibold">{formatRupiah(o.subtotal)}</span>
        {o.editable ? (
          <div className="ml-auto flex gap-1">
            <Button size="sm" variant="ghost" onClick={onCancel}><XIcon aria-hidden />{t('orders.cancel')}</Button>
            <Button size="sm" variant="secondary" onClick={onEdit}><PencilIcon aria-hidden />{t('orders.edit')}</Button>
          </div>
        ) : (
          o.status === 'claimed' && <span className="text-xs text-text-secondary">{t('orders.askWaiter')}</span>
        )}
      </footer>
    </article>
  );
}

/** BR-CM-17: the Estimated Wait as a clock time; overdue reads softly, never "late". */
function ReadyBy({ order: o, now }: { order: OrderView; now: number }) {
  const { t, i18n } = useTranslation();
  let text: string | null = null;
  if (o.status === 'pending' || o.status === 'claimed') text = t('orders.estimateLater');
  else if ((o.status === 'confirmed' || o.status === 'preparing') && o.estimatedWait) {
    const at = o.estimatedWait.asOf + o.estimatedWait.seconds * 1000;
    const left = Math.ceil((at - now) / 60_000);
    const time = new Date(at).toLocaleTimeString(i18n.language === 'en' ? 'en-GB' : 'id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' });
    text = left > 0 ? t('orders.readyBy', { time, n: left }) : t('orders.longer');
  }
  if (!text) return null;
  return (
    <p className="tabular mb-3 flex items-center gap-1.5 text-sm text-text-secondary">
      <Clock3Icon className="size-4 shrink-0" aria-hidden />
      {text}
    </p>
  );
}

function EditOrderSheet({ order, customerId, onClose }: { order: OrderView | null; customerId: string; onClose: () => void }) {
  const { t } = useTranslation();
  const { bi } = useName();
  const qc = useQueryClient();
  const [lines, setLines] = useState<{ id: string; variantId: string; quantity: number; note: string }[]>([]);
  useEffect(() => {
    if (order) setLines(order.items.filter((i) => i.status === 'pending').map((i) => ({ id: i.id, variantId: i.variantId, quantity: i.quantity, note: i.note })));
  }, [order]);
  const save = useMutation({
    mutationFn: async () =>
      unwrap(await api.PATCH('/public/orders/{id}', { params: { path: { id: order!.id } }, body: { items: lines }, headers: { 'Idempotency-Key': idempotencyKey() } })),
    onSuccess: (res) => {
      qc.setQueryData(['me', customerId], res);
      toast.success(t('orders.saved'));
      onClose();
    },
    onError: (e) => toast.error(problemOf(e)?.message ?? t('errors.generic')),
  });
  // Keep the last Order while the sheet animates out after saving.
  const last = useRef(order);
  if (order) last.current = order;
  const items = last.current?.items ?? [];
  return (
    <BottomSheet
      open={!!order}
      onClose={onClose}
      title={last.current ? t('orders.editTitle', { n: last.current.seq }) : ''}
      footer={<Button size="xl" className="w-full" disabled={save.isPending} onClick={() => save.mutate()}>{t('orders.save')}</Button>}
    >
      <ul className="flex flex-col divide-y divide-border-subtle">
        {lines.map((l) => {
          const it = items.find((x) => x.id === l.id);
          if (!it) return null;
          return (
            <li key={l.id} className="flex items-center gap-3 py-3">
              <span className="min-w-0 flex-1 text-sm font-semibold">{bi(it.productName)} <span className="font-normal text-text-secondary">· {bi(it.variantName)}</span></span>
              <Stepper value={l.quantity} onChange={(n) => setLines(lines.map((x) => (x.id === l.id ? { ...x, quantity: n } : x)))} label={t('product.quantity')} />
              <button type="button" className="grid size-10 place-items-center rounded-full text-text-secondary" aria-label={t('cart.remove')} onClick={() => setLines(lines.filter((x) => x.id !== l.id))}>
                <Trash2Icon className="size-4" aria-hidden />
              </button>
            </li>
          );
        })}
      </ul>
    </BottomSheet>
  );
}

function CancelOrderSheet({ order, customerId, onClose }: { order: OrderView | null; customerId: string; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const cancel = useMutation({
    mutationFn: async () => unwrap(await api.DELETE('/public/orders/{id}', { params: { path: { id: order!.id } }, headers: { 'Idempotency-Key': idempotencyKey() } })),
    onSuccess: (res) => {
      qc.setQueryData(['me', customerId], res);
      toast.success(t('orders.cancelled'));
      onClose();
    },
    onError: (e) => toast.error(problemOf(e)?.message ?? t('errors.generic')),
  });
  return (
    <BottomSheet open={!!order} onClose={onClose} title={order ? t('orders.cancelTitle', { n: order.seq }) : ''}>
      <p className="mb-4 text-text-secondary">{t('orders.cancelBody')}</p>
      <div className="flex flex-col gap-2 pb-2">
        <Button size="lg" disabled={cancel.isPending} onClick={() => cancel.mutate()}>{t('orders.cancelConfirm')}</Button>
        <Button size="lg" variant="secondary" onClick={onClose}>{t('orders.keep')}</Button>
      </div>
    </BottomSheet>
  );
}
