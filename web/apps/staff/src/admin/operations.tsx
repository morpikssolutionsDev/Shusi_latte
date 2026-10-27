// Operations report (BR-ADM-11): Shift Closings, Unpaid Bills, Remakes, Refunds and Post-outage
// Entries for one business day — short lists, each row naming who approved it.
import { api, unwrap } from '@sushi-latte/api-client';
import { Card, cn, formatRupiah, formatTime } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

function signed(n: number) {
  return n < 0 ? `−${formatRupiah(-n)}` : n > 0 ? `+${formatRupiah(n)}` : formatRupiah(0);
}

function Section({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <Card className="p-4">
      <h2 className="mb-2 font-semibold">{title} <span className="tabular text-muted-foreground">· {count}</span></h2>
      {count === 0 ? <p className="text-sm text-muted-foreground">{t('cashier.ops.none')}</p> : <ul className="flex flex-col divide-y divide-border-subtle text-sm">{children}</ul>}
    </Card>
  );
}

export function OperationsReport({ date }: { date: string }) {
  const { t } = useTranslation();
  const ops = useQuery({
    queryKey: ['reports', 'operations', date],
    queryFn: async () => unwrap(await api.GET('/reports/operations', { params: { query: { date } } })),
  });
  const o = ops.data;
  if (!o) return null;
  return (
    <section className="flex flex-col gap-3" aria-labelledby="ops-title" data-testid="operations">
      <h2 id="ops-title" className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('cashier.ops.title')}</h2>
      <div className="grid gap-6 lg:grid-cols-2">
        <Section title={t('cashier.ops.shifts')} count={o.shiftClosings.length}>
          {o.shiftClosings.map((s) => (
            <li key={s.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 py-2">
              <span className="flex-1 font-semibold">{s.cashier} · {formatTime(s.openedAt)}–{s.closedAt ? formatTime(s.closedAt) : t('cashier.ops.open')}</span>
              {s.closedAt && (
                <span className="tabular text-muted-foreground">
                  {t('cashier.ops.expected')} {formatRupiah(s.expected ?? 0)} · {t('cashier.ops.counted')} {formatRupiah(s.counted ?? 0)}
                </span>
              )}
              {s.closedAt && (
                <span className={cn('tabular w-24 text-right font-semibold', (s.difference ?? 0) < 0 ? 'text-[var(--sl-color-feedback-danger-fg)]' : (s.difference ?? 0) > 0 && 'text-[var(--sl-color-feedback-warning-fg)]')}>
                  {signed(s.difference ?? 0)}
                </span>
              )}
            </li>
          ))}
        </Section>
        <Section title={t('cashier.ops.unpaid')} count={o.unpaid.length}>
          {o.unpaid.map((b) => (
            <li key={b.id} className="flex items-center gap-3 py-2">
              <span className="flex-1">
                <span className="font-mono">{b.number}</span> · {b.tables.join(' + ')} · {b.reason}
                <span className="block text-xs text-muted-foreground">{t('cashier.ops.approvedBy', { name: b.by })} · {formatTime(b.closedAt)}</span>
              </span>
              <span className="tabular">{formatRupiah(b.total)}</span>
            </li>
          ))}
        </Section>
        <Section title={t('cashier.ops.refunds')} count={o.refunds.length}>
          {o.refunds.map((r) => (
            <li key={r.id} className="flex items-center gap-3 py-2">
              <span className="flex-1">
                <span className="font-mono">{r.billNumber}</span> · {r.reason}
                <span className="block text-xs text-muted-foreground">{t('cashier.ops.approvedBy', { name: r.approvedBy })} · {formatTime(r.createdAt)}</span>
              </span>
              <span className="tabular">−{formatRupiah(r.amount)}</span>
            </li>
          ))}
        </Section>
        <Section title={t('cashier.ops.remakes')} count={o.remakes.length}>
          {o.remakes.map((r, i) => (
            <li key={i} className="py-2">
              <span className="font-mono">{r.billNumber}</span> · {r.item} · {r.reason}
              <span className="block text-xs text-muted-foreground">{t('cashier.ops.approvedBy', { name: r.approvedBy })} · {formatTime(r.at)}</span>
            </li>
          ))}
        </Section>
        <Section title={t('cashier.ops.postOutage')} count={o.postOutage.length}>
          {o.postOutage.map((b) => (
            <li key={b.id} className="flex items-center gap-3 py-2">
              <span className="flex-1"><span className="font-mono">{b.number}</span> · {b.tables.join(' + ')} · {formatTime(b.closedAt)}</span>
              <span className="tabular">{formatRupiah(b.total)}</span>
            </li>
          ))}
        </Section>
      </div>
    </section>
  );
}
