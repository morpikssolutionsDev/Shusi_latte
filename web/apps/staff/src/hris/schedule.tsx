// Shift schedule (BR-HR-02): one week, a row per active Crew member, a column per day. Outlet
// Admin and Management add, move and delete shifts; approved leave shows as an absence (BR-HR-04).
import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Dialog, Field, Input, Select, cn } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { ChevronLeftIcon, ChevronRightIcon, PlusIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '../admin/layout';
import { useSave } from '../admin/api';
import { addDays, hris, scheduleQuery, wibDate, wibMs, wibTime, type ShiftView } from './api';
import { isHrAdmin } from './layout';
import { dayLabel } from './me';

/** Monday of the week holding a WIB date. */
function monday(date: string) {
  const day = new Date(`${date}T00:00:00Z`).getUTCDay(); // 0 = Sunday
  return addDays(date, day === 0 ? -6 : 1 - day);
}

type Editing = { id?: string; crewId: string; date: string; start: string; end: string };

export function SchedulePage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const [week, setWeek] = useState(monday(wibDate()));
  const days = Array.from({ length: 7 }, (_, i) => addDays(week, i));
  const sch = useQuery(scheduleQuery(days[0]!, days[6]!));
  const [edit, setEdit] = useState<Editing | null>(null);
  const canEdit = isHrAdmin(me);
  const today = wibDate();
  const shiftsOf = (crewId: string, date: string) => (sch.data?.shifts ?? []).filter((s) => s.crewId === crewId && wibDate(s.startsAt) === date);
  const onLeave = (crewId: string, date: string) => (sch.data?.leave ?? []).some((l) => l.crewId === crewId && l.fromDate <= date && date <= l.toDate);
  const open = (s: ShiftView) => canEdit && setEdit({ id: s.id, crewId: s.crewId, date: wibDate(s.startsAt), start: wibTime(s.startsAt), end: wibTime(s.endsAt) });

  return (
    <>
      <PageHeader title={t('hris.schedule.title')} actions={
        <>
          <Button variant="outline" size="icon" aria-label={t('hris.schedule.prev')} onClick={() => setWeek(addDays(week, -7))}><ChevronLeftIcon aria-hidden /></Button>
          <Button variant="outline" onClick={() => setWeek(monday(today))}>{t('hris.schedule.today')}</Button>
          <Button variant="outline" size="icon" aria-label={t('hris.schedule.next')} onClick={() => setWeek(addDays(week, 7))}><ChevronRightIcon aria-hidden /></Button>
          {canEdit && <Button onClick={() => setEdit({ crewId: '', date: today, start: '08:00', end: '16:00' })}><PlusIcon aria-hidden />{t('hris.schedule.add')}</Button>}
        </>
      }>
        {t('hris.schedule.hint')}
      </PageHeader>
      <Card className="overflow-x-auto p-0">
        <table className="w-full min-w-[960px] table-fixed text-sm" data-testid="schedule">
          <thead className="text-left text-muted-foreground">
            <tr className="border-b border-border-subtle">
              <th className="w-48 px-4 py-3 font-medium">{t('hris.schedule.crew')}</th>
              {days.map((d) => (
                <th key={d} className={cn('px-2 py-3 font-medium', d === today && 'text-foreground')}>{dayLabel(d)}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sch.data?.crew.map((c) => (
              <tr key={c.id} className="border-b border-border-subtle align-top" data-crew={c.code}>
                <td className="px-4 py-2"><span className="font-mono text-xs text-muted-foreground">{c.code}</span> <span className="font-semibold">{c.name}</span></td>
                {days.map((d) => (
                  <td key={d} className={cn('px-1.5 py-1.5', d === today && 'bg-sunken')}>
                    <div className="flex min-h-10 flex-col gap-1">
                      {onLeave(c.id, d) && (
                        <span className="rounded-control bg-[var(--sl-color-feedback-info-bg)] px-2 py-1 text-xs font-semibold text-[var(--sl-color-feedback-info-fg)]">{t('hris.schedule.onLeave')}</span>
                      )}
                      {shiftsOf(c.id, d).map((s) => (
                        <button key={s.id} type="button" disabled={!canEdit} onClick={() => open(s)} data-shift={`${c.code}-${d}`}
                          className="tabular rounded-control border border-border bg-card px-2 py-1 text-left text-xs font-semibold enabled:hover:bg-accent">
                          {wibTime(s.startsAt).replace(':', '.')}–{wibTime(s.endsAt).replace(':', '.')}
                        </button>
                      ))}
                      {canEdit && shiftsOf(c.id, d).length === 0 && !onLeave(c.id, d) && (
                        <button type="button" aria-label={`${t('hris.schedule.add')} · ${c.name} · ${dayLabel(d)}`} onClick={() => setEdit({ crewId: c.id, date: d, start: '08:00', end: '16:00' })}
                          className="grid h-8 place-items-center rounded-control text-muted-foreground opacity-0 hover:bg-accent hover:opacity-100 focus-visible:opacity-100">
                          <PlusIcon className="size-4" aria-hidden />
                        </button>
                      )}
                    </div>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {edit && sch.data && <ShiftDialog edit={edit} crew={sch.data.crew} onClose={() => setEdit(null)} />}
    </>
  );
}

function ShiftDialog({ edit, crew, onClose }: { edit: Editing; crew: { id: string; code: string; name: string }[]; onClose: () => void }) {
  const { t } = useTranslation();
  const [v, setV] = useState(edit);
  const body = () => {
    const startsAt = wibMs(v.date, v.start);
    let endsAt = wibMs(v.date, v.end);
    if (endsAt <= startsAt) endsAt = wibMs(addDays(v.date, 1), v.end); // a night shift past midnight
    return { crewId: v.crewId, startsAt, endsAt };
  };
  const save = useSave(() => (v.id ? hris.updateShift(v.id, body()) : hris.createShift(body())), [['hris']], t('hris.schedule.saved'));
  const remove = useSave(() => hris.deleteShift(v.id!), [['hris']], t('hris.schedule.deleted'));
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={v.id ? t('hris.schedule.edit') : t('hris.schedule.add')}
      footer={
        <>
          {v.id && <Button variant="ghost" className="mr-auto" disabled={remove.isPending} onClick={() => remove.mutate(undefined, { onSuccess: onClose })}>{t('hris.schedule.delete')}</Button>}
          <Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button>
          <Button disabled={!v.crewId || save.isPending} onClick={() => save.mutate(undefined, { onSuccess: onClose })}>{t('admin.common.save')}</Button>
        </>
      }>
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('hris.schedule.crew')} htmlFor="shift-crew" className="col-span-2">
          <Select id="shift-crew" value={v.crewId} onChange={(e) => setV({ ...v, crewId: e.target.value })}>
            <option value="">—</option>
            {crew.map((c) => <option key={c.id} value={c.id}>{c.code} · {c.name}</option>)}
          </Select>
        </Field>
        <Field label={t('hris.schedule.date')} htmlFor="shift-date" className="col-span-2">
          <Input id="shift-date" type="date" value={v.date} onChange={(e) => setV({ ...v, date: e.target.value })} />
        </Field>
        <Field label={t('hris.schedule.start')} htmlFor="shift-start"><Input id="shift-start" type="time" value={v.start} onChange={(e) => setV({ ...v, start: e.target.value })} /></Field>
        <Field label={t('hris.schedule.end')} htmlFor="shift-end"><Input id="shift-end" type="time" value={v.end} onChange={(e) => setV({ ...v, end: e.target.value })} /></Field>
        <div className="col-span-2 flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" onClick={() => setV({ ...v, start: '08:00', end: '16:00' })}>{t('hris.schedule.morning')}</Button>
          <Button size="sm" variant="secondary" onClick={() => setV({ ...v, start: '15:00', end: '23:00' })}>{t('hris.schedule.evening')}</Button>
        </div>
      </div>
    </Dialog>
  );
}
