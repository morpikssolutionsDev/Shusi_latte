// Cash Drawer (OP-12..OP-14): Opening Float (BR-CS-04), CashHandoverForm (BR-CS-05), pending QRIS
// (BR-CS-07) and the ShiftClosingSummary (BR-CS-06, TR-PAY-01..03). Every figure is the API's.
import type { Schemas } from '@sushi-latte/api-client';
import { Button, Card, Dialog, Field, Input, cn, formatRupiah, formatTime } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckIcon, HandCoinsIcon, LockIcon, TriangleAlertIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { errorText } from '../admin/api';
import { cashier, drawerQuery, type HeldCash, type PendingQRIS } from './api';

type ShiftClosing = Schemas['ShiftClosing'];

/** Rupiah input: digits only, shown formatted. */
function MoneyInput({ id, value, onChange, autoFocus }: { id: string; value: string; onChange: (v: string) => void; autoFocus?: boolean }) {
  return (
    <Input id={id} inputMode="numeric" autoFocus={autoFocus} className="tabular h-14 text-right text-2xl font-bold" value={value ? formatRupiah(Number(value)) : ''}
      placeholder={formatRupiah(0)} onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, 10))} />
  );
}

function signed(n: number) {
  return n < 0 ? `−${formatRupiah(-n)}` : n > 0 ? `+${formatRupiah(n)}` : formatRupiah(0);
}

function differenceTone(n: number) {
  return n === 0 ? 'text-[var(--sl-color-feedback-success-fg)]' : n < 0 ? 'text-[var(--sl-color-feedback-danger-fg)]' : 'text-[var(--sl-color-feedback-warning-fg)]';
}

export function DrawerPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const drawer = useQuery(drawerQuery);
  const [closing, setClosing] = useState<ShiftClosing | null>(null);
  const refresh = () => void qc.invalidateQueries({ queryKey: ['waiter'] });
  if (!drawer.data) return null;
  const d = drawer.data;
  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto grid max-w-6xl grid-cols-1 gap-6 p-6 lg:grid-cols-2">
        <div className="flex flex-col gap-6">
          {closing && <ClosingResult closing={closing} onDismiss={() => setClosing(null)} />}
          {d.session ? <ShiftClosingSummary drawer={d} onClosed={(c) => { setClosing(c); refresh(); }} /> : <OpenDrawer onOpened={refresh} />}
        </div>
        <div className="flex flex-col gap-6">
          <HeldList held={d.held} disabled={!d.session} />
          <PendingList pending={d.pending} />
          <Card className="flex flex-col gap-2 p-4">
            <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('cashier.drawer.received')}</h2>
            {d.handovers.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('cashier.drawer.noHandovers')}</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border-subtle text-sm">
                {d.handovers.map((h) => (
                  <li key={h.id} className="flex items-center gap-3 py-2">
                    <span className="flex-1"><span className="font-semibold">{h.waiter}</span> · {formatTime(h.receivedAt)}</span>
                    <span className="tabular">{formatRupiah(h.received)}</span>
                    {h.difference !== 0 && <span className={cn('tabular w-24 text-right font-semibold', differenceTone(h.difference))}>{signed(h.difference)}</span>}
                  </li>
                ))}
              </ul>
            )}
          </Card>
          <Card className="flex flex-col gap-2 p-4">
            <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('cashier.drawer.refundList')}</h2>
            {d.refunds.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('cashier.drawer.noRefunds')}</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border-subtle text-sm">
                {d.refunds.map((r) => (
                  <li key={r.id} className="flex items-center gap-3 py-2">
                    <span className="flex-1">
                      <span className="font-mono">{r.billNumber}</span> · {r.reason}
                      <span className="block text-xs text-muted-foreground">{t('cashier.ops.approvedBy', { name: r.approvedBy })} · {formatTime(r.createdAt)}</span>
                    </span>
                    <span className="tabular">−{formatRupiah(r.amount)}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}

function OpenDrawer({ onOpened }: { onOpened: () => void }) {
  const { t } = useTranslation();
  const [float, setFloat] = useState('');
  const open = useMutation({
    mutationFn: () => cashier.openDrawer(Number(float || 0)),
    onSuccess: () => {
      toast.success(t('cashier.drawer.opened'));
      onOpened();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Card className="flex flex-col gap-4 p-6">
      <div className="flex items-center gap-3">
        <LockIcon className="size-6 text-[var(--sl-color-feedback-warning-fg)]" aria-hidden />
        <div>
          <h1 className="text-xl font-bold">{t('cashier.drawer.closedTitle')}</h1>
          <p className="text-sm text-muted-foreground">{t('cashier.drawer.closedHint')}</p>
        </div>
      </div>
      <Field label={t('cashier.drawer.float')} htmlFor="float">
        <MoneyInput id="float" value={float} onChange={setFloat} autoFocus />
      </Field>
      <Button size="xl" disabled={open.isPending} onClick={() => open.mutate()}>{t('cashier.drawer.open')}</Button>
    </Card>
  );
}

/** ShiftClosingSummary (COMPONENTS.md §3): TR-PAY-01 itemised, counted entry, difference. */
function ShiftClosingSummary({ drawer: d, onClosed }: { drawer: Schemas['Drawer']; onClosed: (c: ShiftClosing) => void }) {
  const { t } = useTranslation();
  const [counted, setCounted] = useState('');
  const e = d.expected;
  const close = useMutation({
    mutationFn: () => cashier.closeDrawer(Number(counted)),
    onSuccess: (c) => {
      toast.success(t('cashier.drawer.closed'));
      setCounted('');
      onClosed(c);
    },
    onError: (err) => toast.error(errorText(err)),
  });
  const rows: [string, number][] = [
    [t('cashier.drawer.float'), e.openingFloat],
    [t('cashier.drawer.handovers'), e.handovers],
    [t('cashier.drawer.counter'), e.counterTendered],
    [t('cashier.drawer.change'), -e.change],
    [t('cashier.drawer.refunds'), -e.refunds],
  ];
  const blocked = d.pending.length > 0;
  return (
    <Card className="flex flex-col gap-4 p-6" data-testid="shift-closing">
      <div>
        <h1 className="text-xl font-bold">{t('cashier.drawer.expected')}</h1>
        <p className="text-sm text-muted-foreground">{t('cashier.drawer.session', { name: d.session!.cashier, time: formatTime(d.session!.openedAt) })}</p>
      </div>
      <dl className="flex flex-col gap-1.5">
        {rows.map(([label, value]) => (
          <div key={label} className="flex justify-between text-sm">
            <dt>{label}</dt>
            <dd className="tabular">{value < 0 ? `−${formatRupiah(-value)}` : formatRupiah(value)}</dd>
          </div>
        ))}
        <div className="flex justify-between border-t border-border-subtle pt-2 text-lg font-bold">
          <dt>{t('cashier.drawer.total')}</dt>
          <dd className="tabular" data-testid="expected-total">{formatRupiah(e.total)}</dd>
        </div>
      </dl>
      <div className="flex flex-col gap-3 border-t border-border-subtle pt-4">
        <h2 className="font-bold">{t('cashier.drawer.closing')}</h2>
        <Field label={t('cashier.drawer.counted')} htmlFor="counted">
          <MoneyInput id="counted" value={counted} onChange={setCounted} />
        </Field>
        {counted !== '' && (
          <p className={cn('tabular flex justify-between font-semibold', differenceTone(Number(counted) - e.total))}>
            <span>{t('cashier.drawer.difference')}</span>
            <span>{signed(Number(counted) - e.total)}</span>
          </p>
        )}
        {blocked && (
          <p className="flex items-center gap-2 text-sm text-[var(--sl-color-feedback-warning-fg)]">
            <TriangleAlertIcon className="size-4" aria-hidden />{t('cashier.drawer.closeBlocked', { count: d.pending.length })}
          </p>
        )}
        <Button size="xl" disabled={blocked || counted === '' || close.isPending} onClick={() => close.mutate()}>{t('cashier.drawer.closeSubmit')}</Button>
      </div>
    </Card>
  );
}

function ClosingResult({ closing: c, onDismiss }: { closing: ShiftClosing; onDismiss: () => void }) {
  const { t } = useTranslation();
  const diff = c.difference ?? 0;
  return (
    <Card className="flex flex-col gap-2 border-[var(--sl-color-feedback-success-border)] p-6" role="status">
      <h2 className="flex items-center gap-2 text-lg font-bold"><CheckIcon className="size-5" aria-hidden />{t('cashier.drawer.closed')}</h2>
      <p className="tabular text-sm">
        {t('cashier.drawer.closedSummary', { expected: formatRupiah(c.expected.total), counted: formatRupiah(c.counted ?? 0), difference: signed(diff) })}
      </p>
      <Button variant="secondary" className="self-start" onClick={onDismiss}>OK</Button>
    </Card>
  );
}

function HeldList({ held, disabled }: { held: HeldCash[]; disabled: boolean }) {
  const { t } = useTranslation();
  const [from, setFrom] = useState<HeldCash | null>(null);
  return (
    <Card className="flex flex-col gap-2 p-4">
      <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('cashier.drawer.held')}</h2>
      {held.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('cashier.drawer.heldNone')}</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {held.map((h) => (
            <li key={h.crewId} className="flex min-h-16 items-center gap-3 rounded-control border border-border-subtle px-3" data-held={h.code}>
              <div className="flex-1">
                <p className="font-semibold">{h.name} <span className="font-mono text-xs text-muted-foreground">{h.code}</span></p>
                <p className="text-sm text-muted-foreground">{t('cashier.drawer.heldCount', { count: h.payments })}</p>
              </div>
              <span className="tabular font-bold">{formatRupiah(h.amount)}</span>
              <Button disabled={disabled} onClick={() => setFrom(h)}><HandCoinsIcon aria-hidden />{t('cashier.drawer.receive')}</Button>
            </li>
          ))}
        </ul>
      )}
      {from && <HandoverDialog held={from} onClose={() => setFrom(null)} />}
    </Card>
  );
}

/** CashHandoverForm (COMPONENTS.md §3): the difference is recorded against the Waiter (BR-R-36). */
function HandoverDialog({ held, onClose }: { held: HeldCash; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [received, setReceived] = useState(String(held.amount));
  const diff = Number(received || 0) - held.amount;
  const save = useMutation({
    mutationFn: () => cashier.handover(held.crewId, Number(received || 0)),
    onSuccess: (h) => {
      toast.success(h.difference === 0 ? t('cashier.drawer.handoverDone') : t('cashier.drawer.handoverShort', { amount: signed(h.difference), name: h.waiter }));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('cashier.drawer.handoverTitle', { name: held.name })}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={save.isPending || received === ''} onClick={() => save.mutate()}>{t('cashier.drawer.receive')}</Button></>}>
      <div className="flex flex-col gap-3">
        <p className="flex justify-between"><span className="text-muted-foreground">{t('cashier.drawer.handoverExpected')}</span><span className="tabular font-bold">{formatRupiah(held.amount)}</span></p>
        <Field label={t('cashier.drawer.handoverReceived')} htmlFor="received">
          <MoneyInput id="received" value={received} onChange={setReceived} autoFocus />
        </Field>
        <p className={cn('tabular flex justify-between font-semibold', differenceTone(diff))} role="status">
          <span>{t('cashier.drawer.difference')}</span><span>{signed(diff)}</span>
        </p>
      </div>
    </Dialog>
  );
}

function PendingList({ pending }: { pending: PendingQRIS[] }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [writeOff, setWriteOff] = useState<PendingQRIS | null>(null);
  const verify = useMutation({
    mutationFn: (id: string) => cashier.verify(id),
    onSuccess: () => {
      toast.success(t('cashier.drawer.verified'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Card className={cn('flex flex-col gap-2 p-4', pending.length > 0 && 'border-[var(--sl-color-feedback-warning-border)]')} data-testid="pending-qris">
      <h2 className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('cashier.drawer.pending')}</h2>
      {pending.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('cashier.drawer.pendingNone')}</p>
      ) : (
        <>
          <p className="text-sm text-[var(--sl-color-feedback-warning-fg)]">{t('cashier.drawer.pendingHint')}</p>
          <ul className="flex flex-col gap-2">
            {pending.map((p) => (
              <li key={p.id} className="flex min-h-16 items-center gap-3 rounded-control border border-border-subtle px-3">
                <div className="flex-1">
                  <p className="tabular font-bold">{formatRupiah(p.amount)}</p>
                  <p className="text-sm text-muted-foreground"><span className="font-mono">{p.billNumber}</span> · {p.takenByName} · {formatTime(p.takenAt)}</p>
                </div>
                <Button variant="secondary" disabled={verify.isPending} onClick={() => verify.mutate(p.id)}><CheckIcon aria-hidden />{t('cashier.drawer.verify')}</Button>
                <Button variant="ghost" onClick={() => setWriteOff(p)}>{t('cashier.drawer.writeOff')}</Button>
              </li>
            ))}
          </ul>
        </>
      )}
      {writeOff && <WriteOffDialog payment={writeOff} onClose={() => setWriteOff(null)} />}
    </Card>
  );
}

function WriteOffDialog({ payment, onClose }: { payment: PendingQRIS; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [reason, setReason] = useState('');
  const save = useMutation({
    mutationFn: () => cashier.writeOff(payment.id, reason),
    onSuccess: () => {
      toast.success(t('cashier.drawer.writtenOff'));
      void qc.invalidateQueries({ queryKey: ['waiter'] });
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('cashier.drawer.writeOffTitle', { amount: formatRupiah(payment.amount) })}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button variant="danger" disabled={!reason.trim() || save.isPending} onClick={() => save.mutate()}>{t('cashier.drawer.writeOff')}</Button></>}>
      <Field label={t('cashier.drawer.writeOffReason')} htmlFor="writeoff-reason">
        <Input id="writeoff-reason" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Tidak masuk ke merchant" />
      </Field>
    </Dialog>
  );
}
