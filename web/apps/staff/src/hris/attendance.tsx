// Attendance board (OP-15): each Crew member's shift and clock times for a day, with late, absent,
// leave and auto clock-out states; the Outlet Admin corrects a record with a reason (BR-HR-06).
// Management exports a range as CSV for external payroll (BR-HR-05).
import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, type Feedback } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { DownloadIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '../admin/layout';
import { useSave } from '../admin/api';
import { addDays, attendanceQuery, hris, wibDate, wibMs, wibTime, type AttendanceRow } from './api';

const TONE: Record<AttendanceRow['status'], Feedback | undefined> = {
  scheduled: undefined,
  late: 'warning',
  absent: 'danger',
  working: 'success',
  done: undefined,
  auto: 'danger',
  leave: 'info',
};

const hm = (ms?: number) => (ms ? wibTime(ms).replace(':', '.') : '—');

export function AttendancePage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const [date, setDate] = useState(wibDate());
  const rows = useQuery(attendanceQuery(date, date));
  const [fix, setFix] = useState<AttendanceRow | null>(null);
  return (
    <>
      <PageHeader title={t('hris.attendance.title')} actions={
        <Field label={t('hris.attendance.date')} htmlFor="att-date" className="w-44">
          <Input id="att-date" type="date" value={date} onChange={(e) => e.target.value && setDate(e.target.value)} />
        </Field>
      }>
        {t('hris.attendance.hint')}
      </PageHeader>
      <Card className="overflow-x-auto p-0">
        <table className="w-full text-sm" data-testid="attendance">
          <thead className="sticky top-0 bg-card text-left text-muted-foreground">
            <tr className="border-b border-border-subtle">
              {['crew', 'shift', 'in', 'out', 'worked', 'status'].map((k) => (
                <th key={k} className="px-4 py-3 font-medium">{t(`hris.attendance.${k}`)}</th>
              ))}
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody>
            {rows.data?.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">{t('hris.attendance.none')}</td></tr>
            )}
            {rows.data?.map((r, i) => (
              <tr key={`${r.crewId}-${r.attendanceId ?? i}`} className="h-12 border-b border-border-subtle" data-crew={r.code}>
                <td className="px-4"><span className="font-mono text-xs text-muted-foreground">{r.code}</span> <span className="font-semibold">{r.name}</span></td>
                <td className="tabular px-4">{r.shift ? `${hm(r.shift.startsAt)}–${hm(r.shift.endsAt)}` : '—'}</td>
                <td className="tabular px-4">
                  {hm(r.clockInAt)}
                  {r.lateMinutes > 0 && r.clockInAt && <span className="ml-1 text-[var(--sl-color-feedback-warning-fg)]">({t('hris.attendance.lateBy', { count: r.lateMinutes })})</span>}
                </td>
                <td className="tabular px-4">{hm(r.clockOutAt)}</td>
                <td className="tabular px-4">{r.clockInAt ? `${Math.floor(r.workedMinutes / 60)}j ${r.workedMinutes % 60}m` : '—'}</td>
                <td className="px-4">
                  <Chip feedback={TONE[r.status]}>{t(`hris.attendance.statuses.${r.status}`)}</Chip>
                  {r.correctedBy && <span className="mt-1 block text-xs text-muted-foreground">{t('hris.attendance.corrected', { name: r.correctedBy, reason: r.correctionReason })}</span>}
                </td>
                <td className="px-4 text-right">
                  {r.attendanceId && r.clockOutAt && (
                    <Button size="sm" variant={r.status === 'auto' ? 'primary' : 'ghost'} onClick={() => setFix(r)}>{t('hris.attendance.correct')}</Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {me.role === 'management' && <ExportCard />}
      {fix && <CorrectDialog row={fix} onClose={() => setFix(null)} />}
    </>
  );
}

function CorrectDialog({ row, onClose }: { row: AttendanceRow; onClose: () => void }) {
  const { t } = useTranslation();
  const day = wibDate(row.clockInAt);
  const [inAt, setIn] = useState(wibTime(row.clockInAt!));
  const [outAt, setOut] = useState(row.shift && row.auto ? wibTime(row.shift.endsAt) : wibTime(row.clockOutAt!));
  const [reason, setReason] = useState('');
  const outMs = () => {
    const ms = wibMs(day, outAt);
    return ms <= wibMs(day, inAt) ? wibMs(addDays(day, 1), outAt) : ms; // past midnight
  };
  const save = useSave(() => hris.correct(row.attendanceId!, { clockInAt: wibMs(day, inAt), clockOutAt: outMs(), reason }), [['hris']], t('hris.attendance.saved'));
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('hris.attendance.correctTitle', { name: row.name })} description={t('hris.attendance.correctHint')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={!reason.trim() || save.isPending} onClick={() => save.mutate(undefined, { onSuccess: onClose })}>{t('admin.common.save')}</Button></>}>
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('hris.attendance.in')} htmlFor="fix-in"><Input id="fix-in" type="time" value={inAt} onChange={(e) => setIn(e.target.value)} /></Field>
        <Field label={t('hris.attendance.out')} htmlFor="fix-out"><Input id="fix-out" type="time" value={outAt} onChange={(e) => setOut(e.target.value)} /></Field>
        <Field label={t('hris.attendance.correctReason')} htmlFor="fix-reason" className="col-span-2">
          <Input id="fix-reason" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Lupa absen pulang" />
        </Field>
      </div>
    </Dialog>
  );
}

function ExportCard() {
  const { t } = useTranslation();
  const today = wibDate();
  const [from, setFrom] = useState(addDays(today, -13));
  const [to, setTo] = useState(today);
  return (
    <Card className="mt-6 flex flex-wrap items-end gap-3 p-5">
      <div className="min-w-60 flex-1">
        <h2 className="font-semibold">{t('hris.attendance.exportTitle')}</h2>
        <p className="text-sm text-muted-foreground">{t('hris.attendance.exportHint')}</p>
      </div>
      <Field label={t('hris.leave.from')} htmlFor="exp-from" className="w-44"><Input id="exp-from" type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></Field>
      <Field label={t('hris.leave.to')} htmlFor="exp-to" className="w-44"><Input id="exp-to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} /></Field>
      <Button asChild>
        <a href={`/api/v1/attendance/export?from=${from}&to=${to}`} download><DownloadIcon aria-hidden />{t('hris.attendance.download')}</a>
      </Button>
    </Card>
  );
}
