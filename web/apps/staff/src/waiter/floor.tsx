// TableMap (COMPONENTS.md §3) and the Waiter's own Orders.
import { Card, Chip, cn, formatRupiah } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { BellRingIcon, ChevronRightIcon, CircleSlashIcon, HourglassIcon, PlusIcon, UserCheckIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { feedQuery, floorQuery } from './api';

export function FloorPage() {
  const { t } = useTranslation();
  const floor = useQuery(floorQuery);
  if (!floor.data) return null;
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-bold">{t('waiter.floor.title')}</h1>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {floor.data.map((tb) => {
          const b = tb.bill;
          const state = !tb.active ? 'inactive' : !b ? 'free' : b.status === 'settling' ? 'settling' : 'open';
          const body = (
            <Card
              className={cn(
                'flex min-h-32 flex-col gap-1.5 p-4',
                state === 'free' && 'border-dashed',
                state === 'inactive' && 'border-dashed',
                state === 'settling' && 'border-[var(--sl-color-feedback-warning-border)]',
                b?.unclaimed && 'border-[var(--sl-color-status-pending-border)]',
              )}
              data-table={tb.number}
            >
              <div className="flex items-center justify-between">
                <span className="text-lg font-bold">{tb.label}</span>
                {b ? <span className="font-mono text-xs text-muted-foreground">{b.number}</span> : state === 'free' && <PlusIcon className="size-5 text-muted-foreground" aria-hidden />}
              </div>
              <span className="text-sm text-muted-foreground">
                {state === 'free' ? t('waiter.floor.free') : state === 'inactive' ? t('waiter.floor.inactive') : b!.customers.join(', ') || t('waiter.floor.open')}
              </span>
              {b && (
                <div className="mt-auto flex flex-wrap gap-1">
                  {state === 'settling' && <Chip feedback="warning" icon={<HourglassIcon aria-hidden />}>{t('waiter.floor.settling')}</Chip>}
                  {b.unclaimed > 0 && <Chip tone="pending">{t('waiter.floor.unclaimed', { count: b.unclaimed })}</Chip>}
                  {b.readyItems > 0 && <Chip tone="ready" icon={<BellRingIcon aria-hidden />}>{t('waiter.floor.ready', { count: b.readyItems })}</Chip>}
                  {b.flagged && <Chip feedback="danger" icon={<CircleSlashIcon aria-hidden />}>!</Chip>}
                  {b.claimedByMe && <Chip feedback="info" icon={<UserCheckIcon aria-hidden />}>{t('waiter.floor.mine')}</Chip>}
                  <span className="tabular ml-auto self-end text-sm font-semibold">{formatRupiah(b.total)}</span>
                </div>
              )}
            </Card>
          );
          return (
            <li key={tb.id}>
              {b ? (
                <Link to="/waiter/bills/$billId" params={{ billId: b.id }}>{body}</Link>
              ) : tb.active ? (
                <Link to="/waiter/tables/$tableId/new" params={{ tableId: tb.id }}>{body}</Link>
              ) : (
                body
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function MyOrdersPage() {
  const { t } = useTranslation();
  const feed = useQuery(feedQuery);
  if (!feed.data) return null;
  return (
    <div className="flex flex-col gap-3">
      <h1 className="text-xl font-bold">{t('waiter.orders.title')}</h1>
      {feed.data.mine.length === 0 && <p className="py-16 text-center text-muted-foreground">{t('waiter.orders.empty')}</p>}
      {feed.data.mine.map((o) => (
        <Link key={o.id} to="/waiter/bills/$billId" params={{ billId: o.billId }}>
          <Card className="flex min-h-16 items-center gap-3 p-4">
            <div className="min-w-0 flex-1">
              <p className="font-semibold">{o.tables.map((x) => x.label).join(' + ')} · {t('waiter.bill.order', { n: o.seq })}</p>
              <p className="text-sm text-muted-foreground">{o.items.filter((i) => i.status !== 'cancelled').length} item · {o.customerName}</p>
            </div>
            <Chip tone={o.status === 'claimed' ? 'pending' : (o.status as 'confirmed')}>{t(`waiter.status.${o.status}`)}</Chip>
            {o.flagged && <CircleSlashIcon className="size-5 text-[var(--sl-color-feedback-danger-fg)]" aria-label={t('waiter.notify.flagged')} />}
            <ChevronRightIcon className="size-5 text-muted-foreground" aria-hidden />
          </Card>
        </Link>
      ))}
    </div>
  );
}
