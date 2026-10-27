// Kitchen / Bar display (OP-05, COMPONENTS.md §4, LAYOUT.md §3): 1920×1080 landscape, dark
// kitchen surface, ticket columns oldest left with Remakes first, Ready items in the last column.
import { api, idempotencyKey, unwrap, type Schemas } from '@sushi-latte/api-client';
import { Button, Dialog, Field, Select, Textarea, cn, dietaryIcons } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangleIcon, XIcon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText } from '../admin/api';
import { useRealtime } from '../realtime';

type Ticket = Schemas['KitchenTicket'];
type Item = Schemas['KitchenItem'];
type CancelReason = Schemas['CancelReason'];

const queueQuery = (area: string) => ({
  queryKey: ['kitchen', area],
  queryFn: async () => unwrap(await api.GET('/prep-areas/{id}/queue', { params: { path: { id: area } } })),
  refetchInterval: 30_000,
});

const idem = () => ({ 'Idempotency-Key': idempotencyKey() });

/** Server-aligned clock: tickets time against the API's clock, not the TV's. */
function useServerNow(serverNow: number | undefined) {
  const offset = useRef(0);
  if (serverNow) offset.current = serverNow - Date.now();
  const [now, setNow] = useState(Date.now() + offset.current);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now() + offset.current), 1000);
    return () => clearInterval(timer);
  }, []);
  return now;
}

const mmss = (ms: number) => {
  const s = Math.max(0, Math.floor(ms / 1000));
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
};

export function KitchenDisplay({ area }: { area: string }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const queue = useQuery(queueQuery(area));
  const prepAreaId = queue.data?.prepArea.id;
  useRealtime([prepAreaId ? `prep-area:${prepAreaId}` : '', 'menu'], () => void qc.invalidateQueries({ queryKey: ['kitchen', area] }), !!prepAreaId);
  const now = useServerNow(queue.data?.now);
  const [cancel, setCancel] = useState<Item | null>(null);
  // How many tickets are scrolled out of view to the right.
  const scroller = useRef<HTMLElement>(null);
  const [hidden, setHidden] = useState(0);
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const measure = () => {
      const cards = [...el.querySelectorAll('article[data-ticket]')];
      const edge = el.getBoundingClientRect().right;
      setHidden(cards.filter((c) => c.getBoundingClientRect().left > edge - 40).length);
    };
    measure();
    el.addEventListener('scroll', measure);
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => {
      el.removeEventListener('scroll', measure);
      ro.disconnect();
    };
  }, [queue.data]);

  // A new ticket gets a 3 s highlight (MOTION.md §7).
  const seen = useRef<Set<string> | null>(null);
  const [fresh, setFresh] = useState<Set<string>>(new Set());
  useEffect(() => {
    if (!queue.data) return;
    const keys = queue.data.tickets.map((x) => x.key);
    if (seen.current) {
      const added = keys.filter((k) => !seen.current!.has(k));
      if (added.length) {
        setFresh(new Set(added));
        setTimeout(() => setFresh(new Set()), 3000);
      }
    }
    seen.current = new Set(keys);
  }, [queue.data]);

  const step = useMutation({
    mutationFn: async ({ kind, ids }: { kind: 'start' | 'ready'; ids: string[] }) =>
      unwrap(await api.POST(kind === 'start' ? '/order-items/start' : '/order-items/ready', { body: { itemIds: ids }, headers: idem() })),
    onSettled: () => void qc.invalidateQueries({ queryKey: ['kitchen', area] }),
    onError: (e) => toast.error(errorText(e)),
  });

  const q = queue.data;
  const preparing = q?.tickets.reduce((n, x) => n + x.items.filter((i) => i.status === 'preparing').length, 0) ?? 0;
  const confirmed = q?.tickets.reduce((n, x) => n + x.items.filter((i) => i.status === 'confirmed').length, 0) ?? 0;
  const ready = q?.ready.reduce((n, x) => n + x.items.length, 0) ?? 0;

  return (
    <div data-surface="kitchen" data-theme="dark" className="relative flex h-[calc(100dvh-28px)] flex-col bg-background text-foreground">
      <header className="flex h-[72px] shrink-0 items-center gap-6 border-b border-border px-6">
        <h1 className="text-xl font-bold">{q ? t('kitchen.title', { area: q.prepArea.nameId }) : '…'}</h1>
        <span className="text-base text-muted-foreground">{t('kitchen.counts.new')} <b className="tabular text-foreground">{confirmed}</b></span>
        <span className="text-base text-muted-foreground">{t('kitchen.counts.preparing')} <b className="tabular text-foreground">{preparing}</b></span>
        <span className="text-base text-muted-foreground">{t('kitchen.counts.ready')} <b className="tabular text-foreground">{ready}</b></span>
        <span className="tabular ml-auto text-xl font-bold">{new Date(now).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' })}</span>
      </header>
      <div className="flex min-h-0 flex-1">
        <main ref={scroller} className="relative flex min-w-0 flex-1 gap-6 overflow-x-auto p-6">
          {q && q.tickets.length === 0 && <p className="m-auto text-lg text-muted-foreground">{t('kitchen.empty')}</p>}
          {q?.tickets.map((ticket) => (
            <TicketCard key={ticket.key} ticket={ticket} now={now} fresh={fresh.has(ticket.key)} busy={step.isPending}
              onStart={(ids) => step.mutate({ kind: 'start', ids })} onReady={(ids) => step.mutate({ kind: 'ready', ids })} onCancel={setCancel} />
          ))}
        </main>
        {hidden > 0 && (
          // LAYOUT.md §3: overflow tickets scroll, with a visible "+N more".
          <span className="pointer-events-none absolute right-[360px] top-1/2 z-10 -translate-y-1/2 rounded-l-control bg-[var(--sl-color-status-pending-solid)] px-3 py-2 text-base font-bold text-[var(--sl-color-status-pending-on-solid)]">
            {t('kitchen.more', { count: hidden })}
          </span>
        )}
        {q && (
          <aside className="flex w-[340px] shrink-0 flex-col gap-3 overflow-y-auto border-l border-border p-4" aria-label={t('kitchen.counts.ready')}>
            <h2 className="text-base font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('kitchen.counts.ready')}</h2>
            {q.ready.length === 0 && <p className="text-sm text-muted-foreground">—</p>}
            {q.ready.map((ticket) => <ReadyCard key={ticket.key} ticket={ticket} now={now} reminder={q.pickupReminderSeconds} />)}
          </aside>
        )}
      </div>
      {cancel && <CancelDialog item={cancel} onClose={() => setCancel(null)} onDone={() => void qc.invalidateQueries({ queryKey: ['kitchen', area] })} />}
    </div>
  );
}

function TicketCard({ ticket: k, now, fresh, busy, onStart, onReady, onCancel }: {
  ticket: Ticket;
  now: number;
  fresh: boolean;
  busy: boolean;
  onStart: (ids: string[]) => void;
  onReady: (ids: string[]) => void;
  onCancel: (item: Item) => void;
}) {
  const { t } = useTranslation();
  const elapsed = now - k.confirmedAt;
  const ratio = elapsed / (k.prepTimeSeconds * 1000);
  const timer = ratio >= 1 ? 'late' : ratio >= 0.8 ? 'warning' : 'on-time';
  const toStart = k.items.filter((i) => i.status === 'confirmed').map((i) => i.id);
  const toReady = k.items.filter((i) => i.status === 'preparing').map((i) => i.id);
  const status = toReady.length ? 'preparing' : 'confirmed';
  return (
    <article
      data-ticket={k.tables.join(' + ')}
      data-remake={k.remake || undefined}
      className={cn(
        'flex w-[360px] shrink-0 flex-col overflow-hidden rounded-card border-2 bg-card transition-shadow',
        k.remake ? 'border-[var(--sl-kitchen-ticket-remake-border)]' : timer === 'late' ? 'border-[var(--sl-kitchen-ticket-late-border)]' : 'border-border',
        fresh && 'ring-4 ring-[var(--sl-color-status-pending-solid)]',
      )}
    >
      {/* The label colour is the status's own on-solid shade (COLOR.md: 950 on 400 in the dark Kitchen). */}
      <header className="flex items-center gap-3 px-4 py-3"
        style={{ background: `var(--sl-color-status-${status}-solid)`, color: `var(--sl-color-status-${status}-on-solid)` }}>
        <div className="min-w-0 flex-1">
          <p className="truncate text-lg font-bold">{k.tables.join(' + ')} <span className="font-normal">{t('kitchen.order', { n: k.seq })}</span></p>
          <p className="truncate text-sm">{[k.waiter?.split(' ')[0], k.customer].filter(Boolean).join(' · ')}</p>
        </div>
        {k.remake && <span className="rounded bg-white px-2 py-0.5 text-sm font-black text-black">{t('kitchen.remake')}</span>}
      </header>
      <div className="flex items-center gap-3 whitespace-nowrap border-b border-border px-4 py-2">
        <span className="tabular text-2xl font-bold" style={{ color: `var(--sl-color-timer-${timer})` }} role="timer">{mmss(elapsed)}</span>
        <span className="tabular text-sm text-muted-foreground">/ {mmss(k.prepTimeSeconds * 1000)}</span>
        {timer === 'late' && <span className="ml-auto text-sm font-black" style={{ color: 'var(--sl-color-timer-late)' }}>{t('kitchen.late')}</span>}
      </div>
      <ul className="flex flex-1 flex-col divide-y divide-border overflow-y-auto">
        {k.items.map((i) => (
          <li key={i.id} className="flex flex-col gap-1.5 px-4 py-3" data-item={i.name}>
            <div className="flex items-start gap-2">
              <span className={cn('flex-1 text-base font-bold leading-tight', i.status === 'preparing' && 'text-[var(--sl-color-status-preparing-solid)]')}>
                {i.quantity}× {i.name}
                {i.variant && <span className="font-normal text-muted-foreground"> · {i.variant}</span>}
              </span>
              <button type="button" onClick={() => onCancel(i)} aria-label={`${t('kitchen.cancel')} ${i.name}`} className="grid size-11 shrink-0 place-items-center rounded-control text-muted-foreground hover:bg-accent">
                <XIcon className="size-5" aria-hidden />
              </button>
            </div>
            {i.modifiers.length > 0 && <p className="pl-6 text-sm text-muted-foreground">{i.modifiers.join(', ')}</p>}
            {i.note && <p className="pl-6 text-sm italic">“{i.note}”</p>}
            {i.remakeReason && <p className="pl-6 text-sm font-semibold">{t('kitchen.remake')}: {i.remakeReason}</p>}
            {i.warnings.map((w) => {
              const Icon = dietaryIcons[w.code] ?? AlertTriangleIcon;
              return (
                // BR-KT-05: solid Plum strip (DietaryBadge "kitchen" variant).
                <p key={w.id} className="flex items-center gap-2 rounded px-2 py-1 text-sm font-black uppercase"
                  style={{ background: 'var(--sl-color-dietary-warning-solid)', color: 'var(--sl-color-dietary-warning-on-solid)' }}>
                  <Icon className="size-4" aria-hidden />
                  {t('kitchen.avoids', { tag: w.nameId.toUpperCase() })}
                </p>
              );
            })}
          </li>
        ))}
      </ul>
      {k.dietaryNote && <p className="border-t border-border px-4 py-2 text-sm"><b>{t('kitchen.note')}:</b> {k.dietaryNote}</p>}
      <footer className="grid grid-cols-2 gap-2 border-t border-border p-3">
        <Button size="xl" variant="secondary" disabled={busy || !toStart.length} onClick={() => onStart(toStart)}>{t('kitchen.start')}</Button>
        <Button size="xl" disabled={busy || !toReady.length} onClick={() => onReady(toReady)}
          className="bg-[var(--sl-color-status-ready-solid)] text-[var(--sl-color-status-ready-on-solid)] hover:bg-[var(--sl-color-status-ready-solid)]">{t('kitchen.ready')}</Button>
      </footer>
    </article>
  );
}

function ReadyCard({ ticket: k, now, reminder }: { ticket: Ticket; now: number; reminder: number }) {
  const { t } = useTranslation();
  const since = now - (k.items[0]!.readyAt ?? now);
  const waiting = since >= reminder * 1000;
  return (
    <article className={cn('shrink-0 rounded-card border-2 bg-card p-3', waiting ? 'border-[var(--sl-color-timer-warning)]' : 'border-border')} data-ready={k.tables.join(' + ')}>
      <p className="flex items-center gap-2 text-base font-bold">
        {k.tables.join(' + ')}
        <span className="tabular ml-auto text-sm font-normal text-muted-foreground">{mmss(since)}</span>
      </p>
      <p className="text-sm">{k.items.map((i) => `${i.quantity}× ${i.name}`).join(', ')}</p>
      {waiting && <p className="mt-1 text-sm font-bold" style={{ color: 'var(--sl-color-timer-warning)' }}>{t('kitchen.waitingPickup')}</p>}
    </article>
  );
}

const REASONS: CancelReason[] = ['out_of_stock', 'wrong_order', 'customer_request', 'other'];

function CancelDialog({ item, onClose, onDone }: { item: Item; onClose: () => void; onDone: () => void }) {
  const { t } = useTranslation();
  const [reason, setReason] = useState<CancelReason>('out_of_stock');
  const [note, setNote] = useState('');
  const cancel = useMutation({
    mutationFn: async () => unwrap(await api.POST('/order-items/{id}/kitchen-cancel', { params: { path: { id: item.id } }, body: { reason, note }, headers: idem() })),
    onSuccess: () => {
      toast.success(t('waiter.cancel.done'));
      onDone();
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('kitchen.cancelTitle', { name: `${item.quantity}× ${item.name}` })}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button>
        <Button variant="danger" disabled={cancel.isPending || (reason === 'other' && !note.trim())} onClick={() => cancel.mutate()}>{t('waiter.cancel.submit')}</Button>
      </>}>
      <div className="flex flex-col gap-3">
        <Field label={t('waiter.cancel.reason')} htmlFor="k-reason" hint={reason === 'out_of_stock' ? t('waiter.cancel.outOfStockHint') : undefined}>
          <Select id="k-reason" value={reason} onChange={(e) => setReason(e.target.value as CancelReason)}>
            {REASONS.map((r) => <option key={r} value={r}>{t(`waiter.cancel.${r}`)}</option>)}
          </Select>
        </Field>
        <Field label={t('waiter.cancel.note')} htmlFor="k-note">
          <Textarea id="k-note" rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
      </div>
    </Dialog>
  );
}
