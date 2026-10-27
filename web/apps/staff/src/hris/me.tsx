// A Crew member's own HRIS page: clock status, held cash to hand over (BR-HR-07), the next 14 days
// of shifts, and their leave with a request form (BR-HR-04).
import { Button, Card, Chip, Dialog, Field, Input, Select, Textarea, formatRupiah } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { HandCoinsIcon, PlusIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '../admin/layout';
import { useSave } from '../admin/api';
import { addDays, hris, meQuery, wibDate, wibTime, type LeaveView } from './api';

const TYPES = ['annual', 'sick', 'unpaid'] as const;

export function dayLabel(date: string) {
  return new Date(`${date}T00:00:00+07:00`).toLocaleDateString('id-ID', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'Asia/Jakarta' });
}

export function LeaveChip({ status }: { status: LeaveView['status'] }) {
  const { t } = useTranslation();
  return <Chip feedback={status === 'approved' ? 'success' : status === 'rejected' ? 'danger' : 'warning'}>{t(`hris.leave.status.${status}`)}</Chip>;
}

export function MePage() {
  const { t } = useTranslation();
  const me = useQuery(meQuery);
  const [asking, setAsking] = useState(false);
  if (!me.data) return null;
  const m = me.data;
  return (
    <>
      <PageHeader title={t('hris.me.title')} actions={<Button onClick={() => setAsking(true)}><PlusIcon aria-hidden />{t('hris.me.request')}</Button>} />
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="flex flex-col gap-6">
          <Card className="flex flex-col gap-2 p-5">
            <p className="text-lg font-semibold">
              {m.clockedInAt ? t('hris.me.clockedIn', { time: wibTime(m.clockedInAt).replace(':', '.') }) : t('hris.me.notClockedIn')}
            </p>
            {m.heldCash > 0 && (
              <p className="flex items-center gap-2 font-semibold text-[var(--sl-color-feedback-warning-fg)]">
                <HandCoinsIcon className="size-5" aria-hidden />{t('hris.me.heldCash', { amount: formatRupiah(m.heldCash) })}
              </p>
            )}
          </Card>
          <Card className="p-5">
            <h2 className="mb-3 font-semibold">{t('hris.me.shifts')}</h2>
            {m.shifts.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('hris.me.noShifts')}</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border-subtle text-sm">
                {m.shifts.map((s) => (
                  <li key={s.id} className="flex justify-between py-2">
                    <span className="font-semibold">{dayLabel(wibDate(s.startsAt))}</span>
                    <span className="tabular">{wibTime(s.startsAt).replace(':', '.')}–{wibTime(s.endsAt).replace(':', '.')}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
        <Card className="p-5">
          <h2 className="mb-3 font-semibold">{t('hris.me.myLeave')}</h2>
          {m.leave.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('hris.me.noLeave')}</p>
          ) : (
            <ul className="flex flex-col divide-y divide-border-subtle text-sm">
              {m.leave.map((l) => (
                <li key={l.id} className="flex flex-wrap items-center gap-2 py-2.5">
                  <span className="flex-1">
                    <span className="font-semibold">{dayLabel(l.fromDate)}{l.toDate !== l.fromDate && ` – ${dayLabel(l.toDate)}`}</span> · {t(`hris.leave.types.${l.type}`)}
                    {l.decisionReason && <span className="block text-muted-foreground">{l.decisionReason}</span>}
                  </span>
                  <LeaveChip status={l.status} />
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
      {asking && <LeaveDialog onClose={() => setAsking(false)} />}
    </>
  );
}

function LeaveDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const tomorrow = addDays(wibDate(), 1);
  const [from, setFrom] = useState(tomorrow);
  const [to, setTo] = useState(tomorrow);
  const [type, setType] = useState<(typeof TYPES)[number]>('annual');
  const [reason, setReason] = useState('');
  const save = useSave(() => hris.requestLeave({ fromDate: from, toDate: to < from ? from : to, type, reason }), [['hris']], t('hris.leave.requested'));
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('hris.me.request')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={save.isPending} onClick={() => save.mutate(undefined, { onSuccess: onClose })}>{t('hris.leave.submit')}</Button></>}>
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('hris.leave.from')} htmlFor="leave-from"><Input id="leave-from" type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></Field>
        <Field label={t('hris.leave.to')} htmlFor="leave-to"><Input id="leave-to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} /></Field>
        <Field label={t('hris.leave.type')} htmlFor="leave-type" className="col-span-2">
          <Select id="leave-type" value={type} onChange={(e) => setType(e.target.value as (typeof TYPES)[number])}>
            {TYPES.map((x) => <option key={x} value={x}>{t(`hris.leave.types.${x}`)}</option>)}
          </Select>
        </Field>
        <Field label={t('hris.leave.reason')} htmlFor="leave-reason" className="col-span-2">
          <Textarea id="leave-reason" rows={2} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
      </div>
    </Dialog>
  );
}
