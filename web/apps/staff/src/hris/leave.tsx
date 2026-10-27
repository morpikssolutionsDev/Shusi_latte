// Leave decisions (BR-HR-04): pending requests first, approve or reject with a reason; nobody
// decides their own request (the API refuses it).
import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Dialog, Field, Input } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { CheckIcon, XIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '../admin/layout';
import { useSave } from '../admin/api';
import { hris, leaveQuery, type LeaveView } from './api';
import { LeaveChip, dayLabel } from './me';

const days = (l: LeaveView) => Math.round((Date.parse(l.toDate) - Date.parse(l.fromDate)) / 86_400_000) + 1;

export function LeavePage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const leave = useQuery(leaveQuery);
  const [rejecting, setRejecting] = useState<LeaveView | null>(null);
  const approve = useSave((id: string) => hris.decideLeave(id, true), [['hris']], t('hris.leave.approved'));
  const pending = (leave.data ?? []).filter((l) => l.status === 'pending');
  const decided = (leave.data ?? []).filter((l) => l.status !== 'pending');
  const Row = ({ l, actions }: { l: LeaveView; actions?: React.ReactNode }) => (
    <li className="flex flex-wrap items-center gap-3 py-3" data-leave={l.code}>
      <div className="min-w-60 flex-1">
        <p><span className="font-mono text-xs text-muted-foreground">{l.code}</span> <span className="font-semibold">{l.name}</span> · {t(`hris.leave.types.${l.type}`)}</p>
        <p className="text-sm text-muted-foreground">
          {dayLabel(l.fromDate)}{l.toDate !== l.fromDate && ` – ${dayLabel(l.toDate)}`} · {t('hris.leave.days', { count: days(l) })}
          {l.reason && ` · “${l.reason}”`}
        </p>
        {l.decidedBy && (
          <p className="text-xs text-muted-foreground">{t('hris.leave.by', { name: l.decidedBy })}{l.decisionReason && ` · ${l.decisionReason}`}</p>
        )}
      </div>
      {actions ?? <LeaveChip status={l.status} />}
    </li>
  );
  return (
    <>
      <PageHeader title={t('hris.leave.title')}>{t('hris.leave.hint')}</PageHeader>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="p-5">
          <h2 className="mb-1 font-semibold">{t('hris.leave.pending')} · {pending.length}</h2>
          {pending.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('hris.leave.none')}</p>
          ) : (
            <ul className="flex flex-col divide-y divide-border-subtle">
              {pending.map((l) => (
                <Row key={l.id} l={l} actions={l.code === me.crewId ? <LeaveChip status="pending" /> : (
                  <span className="flex gap-2">
                    <Button size="sm" variant="secondary" onClick={() => setRejecting(l)}><XIcon aria-hidden />{t('hris.leave.reject')}</Button>
                    <Button size="sm" disabled={approve.isPending} onClick={() => approve.mutate(l.id)}><CheckIcon aria-hidden />{t('hris.leave.approve')}</Button>
                  </span>
                )} />
              ))}
            </ul>
          )}
        </Card>
        <Card className="p-5">
          <h2 className="mb-1 font-semibold">{t('hris.leave.decided')}</h2>
          {decided.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('hris.leave.none')}</p>
          ) : (
            <ul className="flex flex-col divide-y divide-border-subtle">{decided.map((l) => <Row key={l.id} l={l} />)}</ul>
          )}
        </Card>
      </div>
      {rejecting && <RejectDialog leave={rejecting} onClose={() => setRejecting(null)} />}
    </>
  );
}

function RejectDialog({ leave, onClose }: { leave: LeaveView; onClose: () => void }) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const reject = useSave(() => hris.decideLeave(leave.id, false, reason), [['hris']], t('hris.leave.rejected'));
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('hris.leave.rejectTitle', { name: leave.name })}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button variant="danger" disabled={!reason.trim() || reject.isPending} onClick={() => reject.mutate(undefined, { onSuccess: onClose })}>{t('hris.leave.reject')}</Button></>}>
      <Field label={t('hris.leave.rejectReason')} htmlFor="reject-reason">
        <Input id="reject-reason" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Jadwal penuh" />
      </Field>
    </Dialog>
  );
}
