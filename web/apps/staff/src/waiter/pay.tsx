// Bill actions shared by the Waiter and Cashier: Settling, PaymentPad, close, Split, Move, Merge,
// manual discount and Unpaid Bill (OP-09, OP-10, COMPONENTS.md §3). Every amount shown comes
// from the API (Billing): the app never adds up money itself.
import type { Schemas } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, Select, cn, formatRupiah } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRightLeftIcon, BanknoteIcon, CreditCardIcon, DeleteIcon, HourglassIcon, LockOpenIcon, MergeIcon, PercentIcon, QrCodeIcon, SplitIcon, UserXIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { discountsQuery, errorText } from '../admin/api';
import { PinOverrideDialog } from '../components/pin-override';
import { ReceiptDialog } from '../components/receipt';
import { actions, floorQuery, type StaffBill } from './api';

type Billing = Schemas['Billing'];
type SubBill = Schemas['SubBill'];
type Method = 'cash' | 'qris' | 'card';
type Panel = 'pay' | 'split' | 'move' | 'merge' | 'unpaid' | 'discount' | null;

export function BillActions({ bill }: { bill: StaffBill }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [panel, setPanel] = useState<Panel>(null);
  const [share, setShare] = useState<SubBill | null>(null);
  const [receipt, setReceipt] = useState<{ subBillId?: string } | null>(null);
  const refresh = () => void qc.invalidateQueries({ queryKey: ['waiter'] });
  const b = bill.billing;
  const live = bill.status === 'open' || bill.status === 'settling';
  const settle = useMutation({
    mutationFn: () => (bill.status === 'settling' ? actions.unlock(bill.id) : actions.settle(bill.id)),
    onSettled: refresh,
    onError: (e) => toast.error(errorText(e)),
  });
  const openPay = (s: SubBill | null) => {
    setShare(s);
    setPanel('pay');
  };

  return (
    <Card className="flex flex-col gap-3 p-4" data-testid="bill-actions">
      {b.subBills.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {b.subBills.map((s) => (
            <li key={s.id} className="flex items-center gap-3 rounded-control border border-border-subtle p-3" data-share={s.index}>
              <span className="font-semibold">{t('waiter.split.share', { n: `${s.index}/${s.count}` })}</span>
              <span className="tabular text-sm text-muted-foreground">{formatRupiah(s.totals.total)}</span>
              {s.status === 'closed' ? (
                <Chip tone="served" className="ml-auto">{t('waiter.pay.lunas')}</Chip>
              ) : (
                <Button size="sm" className="ml-auto" disabled={!live} onClick={() => openPay(s)}>
                  {t('waiter.pay.pay')} · {formatRupiah(s.balance)}
                </Button>
              )}
              {s.status === 'closed' && (
                <Button size="sm" variant="ghost" onClick={() => setReceipt({ subBillId: s.id })}>{t('waiter.receipt.reprint')}</Button>
              )}
            </li>
          ))}
        </ul>
      ) : (
        <div className="flex items-baseline justify-between">
          <span className="text-sm text-muted-foreground">{t('waiter.pay.balance')}</span>
          <span className="tabular text-2xl font-bold" data-testid="balance">{formatRupiah(b.balance)}</span>
        </div>
      )}
      {bill.status === 'settling' && (
        <p className="flex items-center gap-2 text-sm text-[var(--sl-color-feedback-warning-fg)]"><HourglassIcon className="size-4" aria-hidden />{t('waiter.pay.settlingHint')}</p>
      )}
      {live ? (
        <div className="flex flex-wrap gap-2">
          {b.subBills.length === 0 && (
            <Button size="lg" className="flex-1" onClick={() => openPay(null)}><BanknoteIcon aria-hidden />{t('waiter.pay.pay')}</Button>
          )}
          <Button size="lg" variant="secondary" disabled={settle.isPending} onClick={() => settle.mutate()}>
            {bill.status === 'settling' ? <><LockOpenIcon aria-hidden />{t('waiter.pay.unlock')}</> : <><HourglassIcon aria-hidden />{t('waiter.pay.settle')}</>}
          </Button>
          {bill.type === 'table' && (
            <>
              <Button variant="outline" disabled={b.payments.length > 0} onClick={() => setPanel('split')}><SplitIcon aria-hidden />{t('waiter.split.action')}</Button>
              <Button variant="outline" onClick={() => setPanel('move')}><ArrowRightLeftIcon aria-hidden />{t('waiter.move.action')}</Button>
              <Button variant="outline" onClick={() => setPanel('merge')}><MergeIcon aria-hidden />{t('waiter.merge.action')}</Button>
            </>
          )}
          <Button variant="outline" disabled={b.payments.length > 0} onClick={() => setPanel('discount')}><PercentIcon aria-hidden />{t('waiter.pay.discount')}</Button>
          <Button variant="ghost" onClick={() => setPanel('unpaid')}><UserXIcon aria-hidden />{t('waiter.pay.unpaid')}</Button>
        </div>
      ) : (
        <Button variant="secondary" onClick={() => setReceipt({})}>{t('waiter.receipt.reprint')}</Button>
      )}
      {b.payments.length > 0 && (
        <ul className="flex flex-col gap-1 border-t border-border-subtle pt-2 text-sm">
          {b.payments.map((p) => (
            <li key={p.id} className="flex justify-between gap-2">
              <span>
                {t(`waiter.pay.${p.method}`)}
                {p.status === 'pending_verification' && <span className="text-[var(--sl-color-feedback-warning-fg)]"> · {t('waiter.pay.pending')}</span>}
                {p.status === 'held_by_waiter' && <span className="text-muted-foreground"> · {t('waiter.pay.held')}</span>}
              </span>
              <span className="tabular">{formatRupiah(p.amount)}</span>
            </li>
          ))}
        </ul>
      )}

      {panel === 'pay' && (
        <PayDialog bill={bill} share={share} onClose={() => setPanel(null)} onClosed={(subBillId) => { setPanel(null); refresh(); setReceipt({ subBillId }); }} />
      )}
      {panel === 'split' && <SplitDialog bill={bill} onClose={() => { setPanel(null); refresh(); }} />}
      {panel === 'move' && <MoveDialog bill={bill} onClose={() => { setPanel(null); refresh(); }} />}
      {panel === 'merge' && <MergeDialog bill={bill} onClose={() => { setPanel(null); refresh(); }} />}
      {panel === 'discount' && <DiscountDialog bill={bill} onClose={() => { setPanel(null); refresh(); }} />}
      {panel === 'unpaid' && <UnpaidDialog bill={bill} onClose={() => { setPanel(null); refresh(); }} />}
      {receipt && <ReceiptDialog billId={bill.id} subBillId={receipt.subBillId} onClose={() => setReceipt(null)} />}
    </Card>
  );
}

const METHODS: { method: Method; icon: typeof BanknoteIcon }[] = [
  { method: 'cash', icon: BanknoteIcon },
  { method: 'qris', icon: QrCodeIcon },
  { method: 'card', icon: CreditCardIcon },
];

/** PaymentPad (COMPONENTS.md §3): method, amount keypad with 64 px keys, change from the API. */
function PayDialog({ bill, share, onClose, onClosed }: { bill: StaffBill; share: SubBill | null; onClose: () => void; onClosed: (subBillId?: string) => void }) {
  const { t } = useTranslation();
  const [billing, setBilling] = useState<Billing>(bill.billing);
  const current = share ? billing.subBills.find((s) => s.id === share.id)! : null;
  const balance = current ? current.balance : billing.balance;
  const cashDue = current ? current.cashDue : billing.cashDue;
  const [method, setMethod] = useState<Method>('cash');
  const [amount, setAmount] = useState('');
  const [verified, setVerified] = useState(true);
  const [askPin, setAskPin] = useState(false);
  const [change, setChange] = useState<number | null>(null);
  const [needsLater, setNeedsLater] = useState(false);
  const [payLater, setPayLater] = useState(false);
  const value = Number(amount || (method === 'cash' ? 0 : balance));

  const pay = useMutation({
    mutationFn: (token?: string) => actions.pay(bill.id, { subBillId: share?.id, method, amount: value, verified: method !== 'qris' || verified }, token),
    onSuccess: (res) => {
      setBilling(res.billing);
      setAmount('');
      setAskPin(false);
      setChange(res.payment.method === 'cash' ? res.payment.change : null);
      toast.success(res.payment.status === 'held_by_waiter' ? t('waiter.pay.heldByWaiter') : t('waiter.pay.recorded'));
    },
    onError: (e) => {
      setAskPin(false);
      toast.error(errorText(e));
    },
  });
  const close = useMutation({
    mutationFn: () => actions.close(bill.id, share?.id, payLater),
    onSuccess: () => {
      toast.success(t('waiter.pay.closed'));
      onClosed(share?.id);
    },
    onError: (e) => {
      if ((e as { code?: string }).code === 'items_unfinished') setNeedsLater(true);
      else toast.error(errorText(e));
    },
  });
  const press = (k: string) => setAmount((a) => (k === 'back' ? a.slice(0, -1) : (a + k).replace(/^0+/, '').slice(0, 9)));
  const paidUp = balance <= 0;

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={share ? t('waiter.pay.shareTitle', { share: `${share.index}/${share.count}` }) : t('waiter.pay.title', { bill: bill.number })}>
      <div className="flex flex-col gap-4">
        <div className="flex items-baseline justify-between">
          <span className="text-muted-foreground">{paidUp ? t('waiter.pay.lunas') : t('waiter.pay.balance')}</span>
          <span className="tabular text-3xl font-bold" data-testid="pay-balance">{formatRupiah(Math.max(balance, 0))}</span>
        </div>
        {change !== null && (
          <div className="flex items-baseline justify-between rounded-control bg-[var(--sl-color-feedback-success-bg)] px-4 py-3 text-[var(--sl-color-feedback-success-fg)]" role="status">
            <span className="font-semibold">{t('waiter.pay.change')}</span>
            <span className="tabular text-2xl font-bold" data-testid="change">{formatRupiah(change)}</span>
          </div>
        )}
        {!paidUp ? (
          <>
            <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label={t('waiter.pay.method')}>
              {METHODS.map(({ method: m, icon: Icon }) => (
                <button key={m} type="button" role="radio" aria-checked={method === m} onClick={() => { setMethod(m); setAmount(''); }}
                  className={cn('flex h-14 items-center justify-center gap-2 rounded-control border font-semibold', method === m ? 'border-foreground bg-foreground text-background' : 'border-border')}>
                  <Icon className="size-5" aria-hidden />{t(`waiter.pay.${m}`)}
                </button>
              ))}
            </div>
            <Field label={method === 'cash' ? t('waiter.pay.tendered') : t('waiter.pay.amount')} htmlFor="pay-amount"
              hint={method === 'cash' && cashDue !== balance ? t('waiter.pay.rounded', { amount: formatRupiah(cashDue) }) : undefined}>
              <Input id="pay-amount" inputMode="numeric" className="tabular h-14 text-right text-2xl font-bold" value={amount ? formatRupiah(Number(amount)) : ''}
                placeholder={formatRupiah(method === 'cash' ? cashDue : balance)} onChange={(e) => setAmount(e.target.value.replace(/\D/g, '').slice(0, 9))} />
            </Field>
            {method === 'cash' && (
              <div className="flex gap-2">
                {[cashDue, 50000, 100000].filter((v, i, a) => a.indexOf(v) === i).map((v, i) => (
                  <Button key={v} variant="secondary" className="flex-1" onClick={() => setAmount(String(v))}>{i === 0 ? t('waiter.pay.exact') : formatRupiah(v)}</Button>
                ))}
              </div>
            )}
            <div className="grid grid-cols-3 gap-2">
              {['1', '2', '3', '4', '5', '6', '7', '8', '9', '000', '0', 'back'].map((k) => (
                <button key={k} type="button" onClick={() => press(k)} aria-label={k === 'back' ? 'Hapus' : k}
                  className="grid h-16 place-items-center rounded-control bg-sunken text-xl font-semibold active:scale-[0.98]">
                  {k === 'back' ? <DeleteIcon className="size-6" aria-hidden /> : k}
                </button>
              ))}
            </div>
            {method === 'qris' && (
              <label className="flex items-start gap-3 text-sm">
                <input type="checkbox" className="mt-0.5 size-5" checked={verified} onChange={(e) => setVerified(e.target.checked)} />
                <span>{t('waiter.pay.qrisVerified')}{!verified && <span className="block text-muted-foreground">{t('waiter.pay.qrisPendingHint')}</span>}</span>
              </label>
            )}
            <Button size="xl" disabled={pay.isPending || value <= 0} onClick={() => (method === 'qris' && !verified ? setAskPin(true) : pay.mutate(undefined))}>
              {t('waiter.pay.record')} · {formatRupiah(value)}
            </Button>
          </>
        ) : bill.type === 'takeaway' ? (
          // Paid in full: the API has already sent the Order to the Kitchen and closed the Bill (TR-PAY-04).
          <>
            <p className="rounded-control bg-[var(--sl-color-feedback-success-bg)] p-3 text-sm text-[var(--sl-color-feedback-success-fg)]">{t('cashier.takeaway.released')}</p>
            <Button size="xl" onClick={() => onClosed()}>{t('cashier.takeaway.receipt')}</Button>
          </>
        ) : (
          <>
            {needsLater && (
              <label className="flex items-start gap-3 rounded-control bg-[var(--sl-color-feedback-warning-bg)] p-3 text-sm">
                <input type="checkbox" className="mt-0.5 size-5" checked={payLater} onChange={(e) => setPayLater(e.target.checked)} />
                <span><span className="font-semibold">{t('waiter.pay.payLater')}</span><span className="block">{t('waiter.pay.payLaterHint')}</span></span>
              </label>
            )}
            <Button size="xl" disabled={close.isPending || (needsLater && !payLater)} onClick={() => close.mutate()}>
              {share ? t('waiter.pay.closeShare', { share: `${share.index}/${share.count}` }) : t('waiter.pay.close')}
            </Button>
          </>
        )}
      </div>
      <PinOverrideDialog action="qris_unverified" open={askPin} onClose={() => setAskPin(false)} onApproved={(tok) => pay.mutate(tok)} />
    </Dialog>
  );
}

function SplitDialog({ bill, onClose }: { bill: StaffBill; onClose: () => void }) {
  const { t } = useTranslation();
  const [method, setMethod] = useState<'by_item' | 'equal'>('by_item');
  const [count, setCount] = useState(2);
  const items = bill.orders.flatMap((o) => o.items).filter((i) => i.status !== 'cancelled' && i.status !== 'voided');
  const [assign, setAssign] = useState<Record<string, number>>(() => Object.fromEntries(items.map((i) => [i.id, 1])));
  const split = useMutation({
    mutationFn: () => actions.split(bill.id, { method, count, assign: method === 'by_item' ? assign : undefined }),
    onSuccess: () => {
      toast.success(t('waiter.split.done'));
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('waiter.split.title')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={split.isPending} onClick={() => split.mutate()}>{t('waiter.split.submit')}</Button></>}>
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-2" role="radiogroup">
          {(['by_item', 'equal'] as const).map((m) => (
            <button key={m} type="button" role="radio" aria-checked={method === m} onClick={() => setMethod(m)}
              className={cn('h-12 rounded-control border font-semibold', method === m ? 'border-foreground bg-foreground text-background' : 'border-border')}>
              {t(m === 'by_item' ? 'waiter.split.byItem' : 'waiter.split.equal')}
            </button>
          ))}
        </div>
        <Field label={t('waiter.split.shares')} htmlFor="split-n">
          <Input id="split-n" type="number" min={2} max={20} value={count} onChange={(e) => setCount(Math.max(2, Math.min(20, Number(e.target.value))))} />
        </Field>
        {method === 'by_item' && (
          <>
            <p className="text-sm text-muted-foreground">{t('waiter.split.assignHint')}</p>
            <ul className="flex flex-col gap-1.5">
              {items.map((i) => (
                <li key={i.id}>
                  <button type="button" data-assign={i.productName.en} onClick={() => setAssign({ ...assign, [i.id]: (assign[i.id]! % count) + 1 })}
                    className="flex min-h-12 w-full items-center gap-3 rounded-control border border-border px-3 text-left">
                    <span className="tabular grid size-8 place-items-center rounded-full bg-foreground text-sm font-bold text-background">{Math.min(assign[i.id]!, count)}</span>
                    <span className="flex-1">{i.quantity}× {i.productName.id}</span>
                    <span className="tabular text-sm text-muted-foreground">{formatRupiah(i.lineTotal)}</span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </Dialog>
  );
}

function MoveDialog({ bill, onClose }: { bill: StaffBill; onClose: () => void }) {
  const { t } = useTranslation();
  const floor = useQuery(floorQuery);
  const [to, setTo] = useState('');
  const free = (floor.data ?? []).filter((x) => x.active && !x.bill);
  const move = useMutation({
    mutationFn: () => actions.move(bill.id, to, floor.data?.find((x) => x.bill?.id === bill.id && x.label === bill.tables[0]?.label)?.id),
    onSuccess: () => {
      toast.success(t('waiter.move.done'));
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('waiter.move.title')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={!to || move.isPending} onClick={() => move.mutate()}>{t('waiter.move.action')}</Button></>}>
      <Select aria-label={t('waiter.move.title')} value={to} onChange={(e) => setTo(e.target.value)}>
        <option value="">—</option>
        {free.map((x) => <option key={x.id} value={x.id}>{x.label}</option>)}
      </Select>
    </Dialog>
  );
}

function MergeDialog({ bill, onClose }: { bill: StaffBill; onClose: () => void }) {
  const { t } = useTranslation();
  const floor = useQuery(floorQuery);
  const others = (floor.data ?? []).filter((x) => x.bill && x.bill.id !== bill.id && x.bill.status === 'open');
  const [picked, setPicked] = useState<string[]>([]);
  const merge = useMutation({
    mutationFn: () => actions.merge(bill.id, [...new Set(picked)]),
    onSuccess: () => {
      toast.success(t('waiter.merge.done'));
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={t('waiter.merge.title')} description={t('waiter.merge.hint')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={!picked.length || merge.isPending} onClick={() => merge.mutate()}>{t('waiter.merge.submit')}</Button></>}>
      <ul className="flex flex-col gap-1.5">
        {others.map((x) => {
          const on = picked.includes(x.bill!.id);
          return (
            <li key={x.id}>
              <label className="flex min-h-12 items-center gap-3 rounded-control border border-border px-3">
                <input type="checkbox" className="size-5" checked={on} onChange={() => setPicked(on ? picked.filter((p) => p !== x.bill!.id) : [...picked, x.bill!.id])} />
                <span className="font-semibold">{x.label}</span>
                <span className="text-sm text-muted-foreground">{x.bill!.customers.join(', ')}</span>
                <span className="tabular ml-auto text-sm">{formatRupiah(x.bill!.total)}</span>
              </label>
            </li>
          );
        })}
      </ul>
    </Dialog>
  );
}

function DiscountDialog({ bill, onClose }: { bill: StaffBill; onClose: () => void }) {
  const { t } = useTranslation();
  const discounts = useQuery(discountsQuery);
  const manual = (discounts.data ?? []).filter((d) => d.trigger === 'manual' && d.scope === 'bill' && d.active);
  const [id, setId] = useState('');
  const [askPin, setAskPin] = useState(false);
  const apply = useMutation({
    mutationFn: (token: string) => actions.discount(bill.id, id, token),
    onSuccess: () => {
      toast.success(t('waiter.pay.applied'));
      onClose();
    },
    onError: (e) => {
      setAskPin(false);
      toast.error(errorText(e));
    },
  });
  return (
    <>
      <Dialog open={!askPin} onOpenChange={(o) => !o && onClose()} title={t('waiter.pay.discount')} description={t('waiter.pay.discountHint')}
        footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button disabled={!id} onClick={() => setAskPin(true)}>{t('waiter.pay.discountApply')}</Button></>}>
        <Select aria-label={t('waiter.pay.discount')} value={id} onChange={(e) => setId(e.target.value)}>
          <option value="">—</option>
          {manual.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
        </Select>
      </Dialog>
      <PinOverrideDialog action="manual_discount" open={askPin} onClose={() => setAskPin(false)} onApproved={(tok) => apply.mutate(tok)} />
    </>
  );
}

function UnpaidDialog({ bill, onClose }: { bill: StaffBill; onClose: () => void }) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');
  const [askPin, setAskPin] = useState(false);
  const unpaid = useMutation({
    mutationFn: (token: string) => actions.unpaid(bill.id, reason, token),
    onSuccess: () => {
      toast.success(t('waiter.pay.unpaidDone'));
      onClose();
    },
    onError: (e) => {
      setAskPin(false);
      toast.error(errorText(e));
    },
  });
  return (
    <>
      <Dialog open={!askPin} onOpenChange={(o) => !o && onClose()} title={t('waiter.pay.unpaid')} description={t('waiter.pay.unpaidHint')}
        footer={<><Button variant="secondary" onClick={onClose}>{t('admin.common.cancel')}</Button><Button variant="danger" disabled={!reason.trim()} onClick={() => setAskPin(true)}>{t('waiter.void.submit')}</Button></>}>
        <Field label={t('waiter.pay.reason')} htmlFor="unpaid-reason">
          <Input id="unpaid-reason" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Pelanggan pergi tanpa bayar" />
        </Field>
      </Dialog>
      <PinOverrideDialog action="unpaid_bill" open={askPin} onClose={() => setAskPin(false)} onApproved={(tok) => unpaid.mutate(tok)} />
    </>
  );
}
