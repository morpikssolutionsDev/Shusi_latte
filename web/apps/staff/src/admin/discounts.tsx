import { api, unwrap, type Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, Select, Switch, cn, formatMinuteOfDay, formatPercentBp, formatRupiah } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { PlusIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { catalogQuery, discountsQuery, useSave, type Discount } from './api';
import { PageHeader } from './layout';

const blank: Discount = {
  name: '', kind: 'percent', value: 1000, scope: 'item', trigger: 'auto', daysMask: 127, active: true,
  categoryIds: [], productIds: [], variantIds: [],
};

const toTime = (m?: number) => (m == null ? '' : `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`);
const fromTime = (s: string) => {
  const [h, m] = s.split(':').map(Number);
  return h == null || m == null || Number.isNaN(h) ? undefined : h * 60 + m;
};

export function DiscountsPage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const { data: discounts } = useQuery(discountsQuery);
  const { data: catalog } = useQuery(catalogQuery);
  const [edit, setEdit] = useState<Discount | null>(null);
  const canEdit = me.role === 'management';
  const save = useSave(
    async (d: Discount) =>
      d.id ? unwrap(await api.PUT('/discounts/{id}', { params: { path: { id: d.id } }, body: d })) : unwrap(await api.POST('/discounts', { body: d })),
    [['discounts']],
    t('admin.common.saved'),
  );
  if (!discounts || !catalog) return null;
  const days = t('admin.days', { returnObjects: true }) as string[];

  const describe = (d: Discount) => {
    const value = d.kind === 'percent' ? formatPercentBp(d.value) : formatRupiah(d.value);
    const when = [
      d.daysMask === 127 ? null : days.filter((_, i) => d.daysMask & (1 << i)).join(', '),
      d.startMinute != null && d.endMinute != null ? `${formatMinuteOfDay(d.startMinute)}–${formatMinuteOfDay(d.endMinute)}` : null,
      d.minSubtotal ? `≥ ${formatRupiah(d.minSubtotal)}` : null,
    ].filter(Boolean);
    const targets = [
      ...d.categoryIds.map((id) => catalog.categories.find((c) => c.id === id)?.nameId),
      ...d.productIds.map((id) => catalog.products.find((p) => p.id === id)?.nameId),
    ].filter(Boolean);
    return { value, when: when.join(' · '), targets: targets.join(', ') };
  };

  return (
    <>
      <PageHeader
        title={t('admin.discounts.title')}
        actions={canEdit && <Button onClick={() => setEdit(structuredClone(blank))}><PlusIcon aria-hidden />{t('admin.discounts.add')}</Button>}
      >
        {t('admin.discounts.rules')}
      </PageHeader>
      <div className="grid gap-3 md:grid-cols-2">
        {discounts.map((d) => {
          const info = describe(d);
          return (
            <Card key={d.id} className={cn('flex flex-col gap-2 p-4', !d.active && 'opacity-60')}>
              <div className="flex items-start justify-between gap-2">
                <h2 className="font-bold">{d.name}</h2>
                <span className="tabular text-lg font-extrabold">{info.value}</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                <Chip feedback="info">{t(`admin.discounts.${d.scope}`)}</Chip>
                <Chip feedback={d.trigger === 'auto' ? 'success' : 'warning'}>{t(`admin.discounts.${d.trigger}`)}</Chip>
                {!d.active && <Chip tone="cancelled">{t('admin.common.inactive')}</Chip>}
              </div>
              {info.when && <p className="text-sm text-text-secondary">{info.when}</p>}
              {info.targets && <p className="text-sm text-muted-foreground">{t('admin.discounts.targets')}: {info.targets}</p>}
              {canEdit && <Button variant="secondary" size="sm" className="self-start" onClick={() => setEdit(structuredClone(d))}>{t('admin.common.edit')}</Button>}
            </Card>
          );
        })}
      </div>

      {edit && (
        <Dialog
          open
          variant="sheet"
          onOpenChange={(o) => !o && setEdit(null)}
          title={edit.id ? edit.name : t('admin.discounts.add')}
          footer={
            <>
              <Button variant="secondary" onClick={() => setEdit(null)}>{t('admin.common.cancel')}</Button>
              <Button disabled={save.isPending} onClick={() => save.mutate(edit, { onSuccess: () => setEdit(null) })}>{t('admin.common.save')}</Button>
            </>
          }
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t('admin.discounts.name')} htmlFor="d-name" className="sm:col-span-2">
              <Input id="d-name" value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })} />
            </Field>
            <Field label={t('admin.discounts.kind')} htmlFor="d-kind">
              <Select id="d-kind" value={edit.kind} onChange={(e) => setEdit({ ...edit, kind: e.target.value as Discount['kind'], value: e.target.value === 'percent' ? 1000 : 10000 })}>
                <option value="percent">{t('admin.discounts.percent')}</option>
                <option value="nominal">{t('admin.discounts.nominal')}</option>
              </Select>
            </Field>
            <Field label={`${t('admin.discounts.value')} (${edit.kind === 'percent' ? '%' : 'Rp'})`} htmlFor="d-value">
              <Input id="d-value" type="number" min={0} step={edit.kind === 'percent' ? 0.5 : 1000} className="tabular"
                value={edit.kind === 'percent' ? edit.value / 100 : edit.value}
                onChange={(e) => setEdit({ ...edit, value: Math.round(Number(e.target.value) * (edit.kind === 'percent' ? 100 : 1)) })} />
            </Field>
            <Field label={t('admin.discounts.scope')} htmlFor="d-scope">
              <Select id="d-scope" value={edit.scope} onChange={(e) => setEdit({ ...edit, scope: e.target.value as Discount['scope'], categoryIds: [], productIds: [], variantIds: [], minSubtotal: undefined })}>
                <option value="item">{t('admin.discounts.item')}</option>
                <option value="bill">{t('admin.discounts.bill')}</option>
              </Select>
            </Field>
            <Field label={t('admin.discounts.trigger')} htmlFor="d-trigger">
              <Select id="d-trigger" value={edit.trigger} onChange={(e) => setEdit({ ...edit, trigger: e.target.value as Discount['trigger'] })}>
                <option value="auto">{t('admin.discounts.auto')}</option>
                <option value="manual">{t('admin.discounts.manual')}</option>
              </Select>
            </Field>
            {edit.trigger === 'auto' && (
              <>
                <fieldset className="sm:col-span-2">
                  <legend className="mb-1.5 text-sm font-semibold">{t('admin.discounts.days')}</legend>
                  <div className="flex flex-wrap gap-1.5">
                    {days.map((day, i) => {
                      const on = (edit.daysMask & (1 << i)) !== 0;
                      return (
                        <button key={day} type="button" aria-pressed={on}
                          onClick={() => setEdit({ ...edit, daysMask: (edit.daysMask ^ (1 << i)) || edit.daysMask })}
                          className={cn('h-9 min-w-12 rounded-chip border px-3 text-sm font-semibold', on ? 'border-foreground bg-foreground text-background' : 'border-border bg-card')}>
                          {day}
                        </button>
                      );
                    })}
                  </div>
                </fieldset>
                <Field label={`${t('admin.discounts.from')} (${t('admin.discounts.allDay')} = kosong)`} htmlFor="d-from">
                  <Input id="d-from" type="time" value={toTime(edit.startMinute)} onChange={(e) => setEdit({ ...edit, startMinute: fromTime(e.target.value) })} />
                </Field>
                <Field label={t('admin.discounts.to')} htmlFor="d-to">
                  <Input id="d-to" type="time" value={toTime(edit.endMinute)} onChange={(e) => setEdit({ ...edit, endMinute: fromTime(e.target.value) })} />
                </Field>
              </>
            )}
            {edit.scope === 'bill' && (
              <Field label={`${t('admin.discounts.minSubtotal')} (Rp)`} htmlFor="d-min" className="sm:col-span-2">
                <Input id="d-min" type="number" min={0} step={10000} className="tabular" value={edit.minSubtotal ?? ''}
                  onChange={(e) => setEdit({ ...edit, minSubtotal: e.target.value === '' ? undefined : Number(e.target.value) })} />
              </Field>
            )}
            {edit.scope === 'item' && (
              <fieldset className="sm:col-span-2">
                <legend className="text-sm font-semibold">{t('admin.discounts.targets')}</legend>
                <p className="mb-2 text-xs text-muted-foreground">{t('admin.discounts.targetsHint')}</p>
                <div className="flex flex-wrap gap-1.5">
                  {catalog.categories.map((c) => {
                    const on = edit.categoryIds.includes(c.id!);
                    return (
                      <button key={c.id} type="button" aria-pressed={on}
                        onClick={() => setEdit({ ...edit, categoryIds: on ? edit.categoryIds.filter((x) => x !== c.id) : [...edit.categoryIds, c.id!] })}
                        className={cn('h-9 rounded-chip border px-3 text-sm font-semibold', on ? 'border-foreground bg-foreground text-background' : 'border-border bg-card')}>
                        {c.nameId}
                      </button>
                    );
                  })}
                </div>
                <select multiple aria-label={t('admin.nav.products')} className="mt-2 h-40 w-full rounded-control border border-input bg-card p-2 text-sm"
                  value={edit.productIds}
                  onChange={(e) => setEdit({ ...edit, productIds: Array.from(e.target.selectedOptions, (o) => o.value) })}>
                  {catalog.products.filter((p) => p.status !== 'archived').map((p) => <option key={p.id} value={p.id}>{p.nameId}</option>)}
                </select>
              </fieldset>
            )}
            <label className="flex items-center gap-3 text-sm font-semibold sm:col-span-2">
              <Switch checked={edit.active} onCheckedChange={(active) => setEdit({ ...edit, active })} />
              {t('admin.common.active')}
            </label>
          </div>
        </Dialog>
      )}
    </>
  );
}
