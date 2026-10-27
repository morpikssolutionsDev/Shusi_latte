// Reports (BR-ADM-11): daily sales from the frozen totals of closed Bills. Headline figures are
// stat tiles; payment methods and best sellers are short tables — no chart needed. Prep Time
// performance and the Operations lists (Shift Closings, Unpaid, Remakes, Refunds) follow.
import { api, unwrap } from '@sushi-latte/api-client';
import { Card, Field, Input, formatRupiah } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from './layout';
import { OperationsReport } from './operations';

/** The business day turns over at 04:00 WIB (TRD §4.3). */
function businessDate(now = new Date()) {
  const wib = new Date(now.getTime() + 7 * 3600_000 - 4 * 3600_000);
  return wib.toISOString().slice(0, 10);
}

function Tile({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card className="flex flex-col gap-1 p-4">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="tabular text-2xl font-bold">{value}</span>
      {sub && <span className="text-xs text-muted-foreground">{sub}</span>}
    </Card>
  );
}

export function ReportsPage() {
  const { t } = useTranslation();
  const [date, setDate] = useState(businessDate());
  const report = useQuery({
    queryKey: ['reports', 'daily', date],
    queryFn: async () => unwrap(await api.GET('/reports/daily-sales', { params: { query: { date } } })),
  });
  const prep = useQuery({
    queryKey: ['reports', 'prep', date],
    queryFn: async () => unwrap(await api.GET('/reports/prep-time', { params: { query: { date } } })),
  });
  const r = report.data;
  const pct = (x: number) => `${Math.round(x * 100)}%`;
  return (
    <>
      <PageHeader title={t('admin.reports.title')} actions={
        <Field label={t('admin.reports.date')} htmlFor="report-date" className="w-44">
          <Input id="report-date" type="date" value={date} onChange={(e) => e.target.value && setDate(e.target.value)} />
        </Field>
      }>
        {t('admin.reports.dailyHint')}
      </PageHeader>
      {r && (
        <div className="flex flex-col gap-6">
          <section className="grid grid-cols-2 gap-3 lg:grid-cols-4" aria-label={t('admin.reports.daily')}>
            <Tile label={t('admin.reports.sales')} value={formatRupiah(r.totals.total)} sub={t('admin.reports.bills', { count: r.bills })} />
            <Tile label={t('admin.reports.discounts')} value={formatRupiah(r.totals.itemDiscount + r.totals.billDiscount)} />
            <Tile label={t('admin.reports.serviceTax')} value={formatRupiah(r.totals.service + r.totals.tax)}
              sub={`${formatRupiah(r.totals.service)} + ${formatRupiah(r.totals.tax)}`} />
            <Tile label={t('admin.reports.unpaid')} value={formatRupiah(r.unpaidTotal)} sub={t('admin.reports.bills', { count: r.unpaidBills })} />
          </section>
          <div className="grid gap-6 lg:grid-cols-2">
            <Card className="p-4">
              <h2 className="mb-3 font-semibold">{t('admin.reports.methods')}</h2>
              <table className="w-full text-sm">
                <thead className="text-left text-muted-foreground">
                  <tr><th className="py-2 font-medium">{t('admin.reports.method')}</th><th className="py-2 text-right font-medium">{t('admin.reports.count')}</th><th className="py-2 text-right font-medium">{t('admin.reports.amount')}</th><th className="py-2 text-right font-medium">{t('admin.reports.pending')}</th></tr>
                </thead>
                <tbody>
                  {r.methods.map((m) => (
                    <tr key={m.method} className="border-t border-border-subtle">
                      <td className="py-2">{t(`waiter.pay.${m.method}`)}</td>
                      <td className="tabular py-2 text-right">{m.count}</td>
                      <td className="tabular py-2 text-right">{formatRupiah(m.amount)}</td>
                      <td className="tabular py-2 text-right">{m.pending ? formatRupiah(m.pending) : '—'}</td>
                    </tr>
                  ))}
                  <tr className="border-t border-border-subtle text-muted-foreground">
                    <td className="py-2" colSpan={2}>{t('admin.reports.rounding')}</td>
                    <td className="tabular py-2 text-right">{formatRupiah(r.rounding)}</td>
                    <td />
                  </tr>
                </tbody>
              </table>
            </Card>
            <Card className="p-4">
              <h2 className="mb-3 font-semibold">{t('admin.reports.bestSellers')}</h2>
              {r.products.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t('admin.common.empty')}</p>
              ) : (
                <table className="w-full text-sm">
                  <thead className="text-left text-muted-foreground">
                    <tr><th className="py-2 font-medium">{t('admin.nav.products')}</th><th className="py-2 text-right font-medium">{t('admin.reports.qty')}</th><th className="py-2 text-right font-medium">{t('admin.reports.gross')}</th></tr>
                  </thead>
                  <tbody>
                    {r.products.map((p) => (
                      <tr key={p.productId} className="border-t border-border-subtle">
                        <td className="py-2">{p.name}</td>
                        <td className="tabular py-2 text-right">{p.quantity}</td>
                        <td className="tabular py-2 text-right">{formatRupiah(p.gross)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </Card>
          </div>
          {prep.data && (
            <Card className="p-4">
              <h2 className="mb-1 font-semibold">{t('admin.reports.prepTime')}</h2>
              <p className="mb-3 text-sm text-muted-foreground">
                {t('admin.reports.prepSummary', { share: pct(prep.data.share), onTime: prep.data.onTime, items: prep.data.items })}
              </p>
              <table className="w-full text-sm">
                <thead className="text-left text-muted-foreground">
                  <tr><th className="py-2 font-medium">{t('admin.reports.prepWhat')}</th><th className="py-2 text-right font-medium">{t('admin.reports.items')}</th><th className="py-2 text-right font-medium">{t('admin.reports.onTime')}</th></tr>
                </thead>
                <tbody>
                  {[...prep.data.prepAreas, ...prep.data.products.slice(0, 8)].map((row, i) => (
                    <tr key={row.key} className={i === prep.data!.prepAreas.length ? 'border-t-2 border-border' : 'border-t border-border-subtle'}>
                      <td className={i < prep.data!.prepAreas.length ? 'py-2 font-semibold' : 'py-2'}>{row.name}</td>
                      <td className="tabular py-2 text-right">{row.items}</td>
                      <td className="tabular py-2 text-right">{pct(row.share)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          )}
          <OperationsReport date={date} />
        </div>
      )}
    </>
  );
}
