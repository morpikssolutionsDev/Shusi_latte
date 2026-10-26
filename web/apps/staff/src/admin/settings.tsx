import { api, unwrap, type Principal } from '@sushi-latte/api-client';
import { Button, Card, Field, Input, Select, formatRupiah } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { settingsQuery, useSave, type Settings } from './api';
import { PageHeader } from './layout';

const OPS_FIELDS = [
  ['claimRenotifySeconds', 'claimRenotify'],
  ['claimEscalateSeconds', 'claimEscalate'],
  ['pickupReminderSeconds', 'pickupReminder'],
  ['emptyBillCloseSeconds', 'emptyBillClose'],
  ['autoClockOutGraceSeconds', 'autoClockOut'],
] as const;

/** Tax, service charge, price mode, Refund cap (BR-ADM-04, BR-ADM-06) and timers (BR-ADM-13). */
export function SettingsPage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const { data } = useQuery(settingsQuery);
  const [draft, setDraft] = useState<Settings | null>(null);
  useEffect(() => {
    if (data) setDraft(structuredClone(data));
  }, [data]);
  const canEdit = me.role === 'management';
  const savePricing = useSave(async (p: Settings['pricing']) => unwrap(await api.PUT('/settings/pricing', { body: p })), [['settings']], t('admin.common.saved'));
  const saveOps = useSave(async (o: Settings['ops']) => unwrap(await api.PUT('/settings/ops', { body: o })), [['settings']], t('admin.common.saved'));
  if (!draft) return null;
  const p = draft.pricing;
  const setPricing = (patch: Partial<Settings['pricing']>) => setDraft({ ...draft, pricing: { ...p, ...patch } });

  return (
    <>
      <PageHeader title={t('admin.settings.title')}>{!canEdit && t('admin.common.readOnly')}</PageHeader>
      <fieldset disabled={!canEdit} className="grid max-w-4xl gap-6 lg:grid-cols-2">
        <Card className="flex flex-col gap-4 p-5">
          <h2 className="text-lg font-bold">{t('admin.settings.pricing')}</h2>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t('admin.settings.tax')} htmlFor="tax">
              <Input id="tax" type="number" min={0} max={25} step={0.5} className="tabular" value={p.taxRateBp / 100} onChange={(e) => setPricing({ taxRateBp: Math.round(Number(e.target.value) * 100) })} />
            </Field>
            <Field label={t('admin.settings.service')} htmlFor="service">
              <Input id="service" type="number" min={0} max={25} step={0.5} className="tabular" value={p.serviceRateBp / 100} onChange={(e) => setPricing({ serviceRateBp: Math.round(Number(e.target.value) * 100) })} />
            </Field>
          </div>
          <Field label={t('admin.settings.priceMode')} htmlFor="mode">
            <Select id="mode" value={p.priceMode} onChange={(e) => setPricing({ priceMode: e.target.value as Settings['pricing']['priceMode'] })}>
              <option value="exclusive">{t('admin.settings.exclusive')}</option>
              <option value="inclusive">{t('admin.settings.inclusive')}</option>
            </Select>
          </Field>
          <Field label={t('admin.settings.refundCap')} htmlFor="cap" hint={formatRupiah(p.refundCap)}>
            <Input id="cap" type="number" min={0} step={50000} className="tabular" value={p.refundCap} onChange={(e) => setPricing({ refundCap: Number(e.target.value) })} />
          </Field>
          {canEdit && <Button className="self-start" disabled={savePricing.isPending} onClick={() => savePricing.mutate(p)}>{t('admin.common.save')}</Button>}
        </Card>

        <Card className="flex flex-col gap-4 p-5">
          <h2 className="text-lg font-bold">{t('admin.settings.ops')}</h2>
          {OPS_FIELDS.map(([key, label]) => (
            <Field key={key} label={t(`admin.settings.${label}`)} htmlFor={key}>
              <Input id={key} type="number" min={0.5} step={0.5} className="tabular" value={draft.ops[key] / 60}
                onChange={(e) => setDraft({ ...draft, ops: { ...draft.ops, [key]: Math.round(Number(e.target.value) * 60) } })} />
            </Field>
          ))}
          {canEdit && <Button className="self-start" disabled={saveOps.isPending} onClick={() => saveOps.mutate(draft.ops)}>{t('admin.common.save')}</Button>}
        </Card>
      </fieldset>
    </>
  );
}
