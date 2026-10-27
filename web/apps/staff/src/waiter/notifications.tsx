// Notifications tab: ClaimNotification cards (COMPONENTS.md §3), Waiter calls, Ready items to
// serve and the Waiter's Claims waiting for confirmation (OP-03, OP-06).
import { Button, Card, Chip, DietaryBadge, cn, dietaryIcons, domainIcons, statusIcons, type Tone } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { AlertTriangleIcon, ChevronRightIcon, CircleSlashIcon, SirenIcon } from 'lucide-react';
import { AnimatePresence, motion } from 'motion/react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText } from '../admin/api';
import { actions, feedQuery, type CallView, type ItemView, type StaffOrder } from './api';

export function useMinutesAgo() {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 15_000);
    return () => clearInterval(timer);
  }, []);
  const { t } = useTranslation();
  return (ms: number) => {
    const n = Math.floor((now - ms) / 60_000);
    return n < 1 ? t('waiter.notify.justNow') : t('waiter.notify.ago', { n });
  };
}

const tablesOf = (o: { tables: { label: string }[] }) => o.tables.map((x) => x.label).join(' + ');

export function NotificationsPage() {
  const { t } = useTranslation();
  const feed = useQuery(feedQuery);
  if (!feed.data) return null;
  const { unclaimed, calls, mine } = feed.data;
  const ready = mine.filter((o) => o.items.some((i) => i.status === 'ready'));
  const toConfirm = mine.filter((o) => o.status === 'claimed');
  const empty = !unclaimed.length && !calls.length && !ready.length && !toConfirm.length;
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold">{t('waiter.notify.title')}</h1>
      {empty && <p className="py-16 text-center text-muted-foreground">{t('waiter.notify.empty')}</p>}
      <ul className="flex flex-col gap-3">
        <AnimatePresence initial={false}>
          {calls.map((c) => <CallCard key={c.id} call={c} />)}
          {unclaimed.map((o) => <ClaimCard key={o.id} order={o} />)}
        </AnimatePresence>
      </ul>
      {ready.length > 0 && (
        <section className="flex flex-col gap-3">
          {ready.map((o) => <ReadyCard key={o.id} order={o} />)}
        </section>
      )}
      {toConfirm.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('waiter.notify.toConfirm')}</h2>
          {toConfirm.map((o) => (
            <Link key={o.id} to="/waiter/bills/$billId" params={{ billId: o.billId }}>
              <Card className="flex min-h-16 items-center gap-3 p-4">
                <span className="font-semibold">{tablesOf(o)}</span>
                <span className="text-sm text-muted-foreground">{t('waiter.bill.order', { n: o.seq })} · {o.customerName}</span>
                {o.flagged && <Chip feedback="danger" icon={<CircleSlashIcon aria-hidden />}>{t('waiter.notify.flagged')}</Chip>}
                <ChevronRightIcon className="ml-auto size-5 text-muted-foreground" aria-hidden />
              </Card>
            </Link>
          ))}
        </section>
      )}
    </div>
  );
}

/** Slides in from the top with one attention pulse (MOTION.md §7). */
function Pulse({ children, escalated }: { children: React.ReactNode; escalated?: boolean }) {
  return (
    <motion.li
      layout
      initial={{ opacity: 0, y: -24 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, height: 0, marginBottom: 0 }}
      transition={{ duration: 0.2, ease: [0.05, 0.7, 0.1, 1] }}
    >
      <Card className={cn('sl-attention flex flex-col gap-3 p-4', escalated && 'border-[var(--sl-color-feedback-danger-border)]')}>{children}</Card>
    </motion.li>
  );
}

function ClaimCard({ order: o }: { order: StaffOrder }) {
  const { t } = useTranslation();
  const ago = useMinutesAgo();
  const qc = useQueryClient();
  const claim = useMutation({
    mutationFn: () => actions.claim(o.id),
    onSuccess: () => {
      toast.success(t('waiter.notify.claimed'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
    },
    onError: (e) => {
      toast.error(errorText(e));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
    },
  });
  const live = o.items.filter((i) => i.status !== 'cancelled');
  const warnings = live.reduce((n, i) => n + i.warnings.length, 0);
  return (
    <Pulse escalated={!!o.escalatedAt}>
      <div className="flex flex-wrap items-center gap-2" data-claim={tablesOf(o)}>
        <span className="text-lg font-bold">{t('waiter.notify.newOrder', { table: tablesOf(o), count: live.length })}</span>
        <span className="text-sm text-muted-foreground">{o.customerName} · {ago(o.createdAt)}</span>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {warnings > 0 && <DietaryBadge icon={<AlertTriangleIcon aria-hidden />}>{t('waiter.notify.warnings', { count: warnings })}</DietaryBadge>}
        {o.flagged && <Chip feedback="danger" icon={<CircleSlashIcon aria-hidden />}>{t('waiter.notify.flagged')}</Chip>}
        {o.escalatedAt ? (
          <Chip feedback="danger" icon={<SirenIcon aria-hidden />}>{t('waiter.notify.escalated')}</Chip>
        ) : (
          o.renotifiedAt && <Chip feedback="warning">{t('waiter.notify.renotified')}</Chip>
        )}
      </div>
      <ItemList items={live} compact />
      <Button size="xl" className="w-full" disabled={claim.isPending} onClick={() => claim.mutate()}>
        {t('waiter.notify.claim')}
      </Button>
    </Pulse>
  );
}

function CallCard({ call: c }: { call: CallView }) {
  const { t } = useTranslation();
  const ago = useMinutesAgo();
  const qc = useQueryClient();
  const answer = useMutation({
    mutationFn: () => actions.answer(c.id),
    onSettled: () => void qc.invalidateQueries({ queryKey: ['waiter'] }),
    onError: (e) => toast.error(errorText(e)),
  });
  const Icon = domainIcons.callWaiter;
  return (
    <Pulse escalated={!!c.escalatedAt}>
      <div className="flex items-center gap-3" data-call={c.table.label}>
        <span className="grid size-11 place-items-center rounded-full bg-[var(--sl-color-feedback-info-bg)] text-[var(--sl-color-feedback-info-fg)]">
          <Icon className="size-5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-lg font-bold">{c.kind === 'not_mine' ? t('waiter.notify.notMine', { table: c.table.label }) : t('waiter.notify.call', { table: c.table.label })}</p>
          <p className="text-sm text-muted-foreground">{c.customerName} · {ago(c.createdAt)}</p>
        </div>
        {c.escalatedAt && <Chip feedback="danger" icon={<SirenIcon aria-hidden />}>{t('waiter.notify.escalated')}</Chip>}
      </div>
      <Button size="lg" variant="secondary" className="w-full" disabled={answer.isPending} onClick={() => answer.mutate()}>
        {t('waiter.notify.answer')}
      </Button>
    </Pulse>
  );
}

function ReadyCard({ order: o }: { order: StaffOrder }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const ready = o.items.filter((i) => i.status === 'ready');
  const serve = useMutation({
    mutationFn: () => actions.serve(ready.map((i) => i.id)),
    onSuccess: () => toast.success(t('waiter.notify.served')),
    onSettled: () => void qc.invalidateQueries({ queryKey: ['waiter'] }),
    onError: (e) => toast.error(errorText(e)),
  });
  const Ready = statusIcons.ready;
  return (
    <Card className="flex flex-col gap-3 border-[var(--sl-color-status-ready-border)] p-4">
      <p className="flex items-center gap-2 text-lg font-bold"><Ready className="size-5" aria-hidden />{t('waiter.notify.ready', { table: tablesOf(o) })}</p>
      <ItemList items={ready} compact />
      <Button size="xl" className="w-full" disabled={serve.isPending} onClick={() => serve.mutate()}>{t('waiter.notify.serve')}</Button>
    </Card>
  );
}

/** Order Items with status, Dietary Warnings and Sold Out / unavailable flags (OrderReviewList). */
export function ItemList({ items, compact, actions: render }: { items: ItemView[]; compact?: boolean; actions?: (i: ItemView) => React.ReactNode }) {
  const { t } = useTranslation();
  return (
    <ul className="flex flex-col divide-y divide-border-subtle">
      {items.map((i) => {
        const gone = i.status === 'cancelled' || i.status === 'voided';
        const Icon = statusIcons[i.status as keyof typeof statusIcons];
        return (
          <li key={i.id} className={cn('flex flex-col gap-1.5', compact ? 'py-1.5' : 'py-3')} data-item={i.productName.en}>
            <div className="flex items-start gap-2">
              <span className="tabular w-8 shrink-0 font-bold">×{i.quantity}</span>
              <span className={cn('min-w-0 flex-1', gone && 'text-muted-foreground line-through')}>
                <span className="font-semibold">{i.productName.id}</span>
                {i.variantName.id && <span className="text-muted-foreground"> · {i.variantName.id}</span>}
                {i.modifiers.length > 0 && <span className="block text-sm text-muted-foreground">{i.modifiers.map((m) => m.name.id).join(', ')}</span>}
                {i.note && <span className="block text-sm italic">“{i.note}”</span>}
                {i.unitPrice === 0 && !gone && <span className="mt-1 inline-block rounded bg-[var(--sl-color-status-preparing-bg)] px-1.5 text-[11px] font-bold">REMAKE</span>}
              </span>
              {!compact && <Chip tone={i.status as Tone} icon={Icon && <Icon aria-hidden />}>{t(`waiter.status.${i.status}`)}</Chip>}
              {render?.(i)}
            </div>
            {!gone && i.warnings.length > 0 && (
              <div className="flex flex-wrap gap-1 pl-10">
                {i.warnings.map((w) => {
                  const WIcon = dietaryIcons[w.code];
                  return <DietaryBadge key={w.id} icon={WIcon && <WIcon aria-hidden />}>{w.nameId}</DietaryBadge>;
                })}
              </div>
            )}
            {!gone && (i.soldOutFlag || i.unavailableFlag) && (
              <p className="flex items-center gap-1.5 pl-10 text-sm font-semibold text-[var(--sl-color-feedback-danger-fg)]">
                <CircleSlashIcon className="size-4" aria-hidden />
                {i.soldOutFlag ? t('waiter.bill.soldOutFlag') : t('waiter.bill.unavailableFlag')}
              </p>
            )}
          </li>
        );
      })}
    </ul>
  );
}
