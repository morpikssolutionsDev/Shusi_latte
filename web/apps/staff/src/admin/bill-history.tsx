// Closed Bills of a business day (BR-ADM-11): Customer names while they are kept, "Customer" once
// Anonymised after 48 hours (TR-RET-01, G-24) — pick the day before yesterday to see both.
import { api, unwrap } from '@sushi-latte/api-client';
import { Card, Chip, formatRupiah, formatTime } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { ShieldCheckIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export function BillHistory({ date }: { date: string }) {
  const { t } = useTranslation();
  const bills = useQuery({
    queryKey: ['reports', 'bills', date],
    queryFn: async () => unwrap(await api.GET('/reports/bills', { params: { query: { date } } })),
  });
  if (!bills.data) return null;
  return (
    <section className="flex flex-col gap-3" aria-labelledby="history-title">
      <div>
        <h2 id="history-title" className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('admin.reports.history')} · {bills.data.length}</h2>
        <p className="text-sm text-muted-foreground">{t('admin.reports.historyHint')}</p>
      </div>
      {/* Scrolls on its own, so it is a named, focusable region for keyboard users. */}
      <Card className="max-h-[480px] overflow-y-auto p-0" tabIndex={0} role="region" aria-labelledby="history-title">
        <table className="w-full text-sm" data-testid="bill-history">
          <thead className="sticky top-0 bg-card text-left text-muted-foreground">
            <tr className="border-b border-border-subtle">
              <th className="px-4 py-2.5 font-medium">{t('admin.reports.bill')}</th>
              <th className="px-4 py-2.5 font-medium">{t('admin.reports.customers')}</th>
              <th className="px-4 py-2.5 font-medium">{t('admin.reports.closedAt')}</th>
              <th className="px-4 py-2.5 text-right font-medium">{t('admin.reports.amount')}</th>
            </tr>
          </thead>
          <tbody>
            {bills.data.length === 0 && (
              <tr><td colSpan={4} className="px-4 py-6 text-center text-muted-foreground">{t('admin.common.empty')}</td></tr>
            )}
            {bills.data.map((b) => (
              <tr key={b.id} className="h-11 border-b border-border-subtle" data-bill={b.number}>
                <td className="px-4">
                  <span className="font-mono">{b.number}</span> · {b.tables.join(' + ')}
                  {b.status === 'unpaid' && <Chip feedback="danger" className="ml-2">{t('cashier.bills.unpaid')}</Chip>}
                  {b.source === 'post_outage' && <Chip feedback="info" className="ml-2">{t('cashier.outage.title')}</Chip>}
                </td>
                <td className="px-4">
                  {b.customers.join(', ') || '—'}
                  {b.anonymised && (
                    <Chip className="ml-2" icon={<ShieldCheckIcon aria-hidden />}>{t('admin.reports.anonymised')}</Chip>
                  )}
                </td>
                <td className="tabular px-4">{formatTime(b.closedAt)}</td>
                <td className="tabular px-4 text-right">{formatRupiah(b.total)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </section>
  );
}
