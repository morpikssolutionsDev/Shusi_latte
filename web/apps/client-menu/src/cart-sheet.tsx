// CartButton / CartSheet (COMPONENTS.md §2) and the "Order sent" celebration (M8).
import { api, idempotencyKey, unwrap } from '@sushi-latte/api-client';
import { Button, cn, formatRupiah } from '@sushi-latte/ui';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { AlertCircleIcon, HourglassIcon, LoaderIcon, ShoppingBagIcon, Trash2Icon } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { problemOf, type CustomerView, type ItemProblem, type Menu } from './api';
import { cart, toItems, useCart } from './cart';
import { CupFill } from './cup';
import { BottomSheet, RollingPrice, Stepper, WarningBadge, ease, spring, useName } from './kit';

/** Floating cart: a full-width bar once the cart has items (LAYOUT.md §3). */
export function CartBar({ onOpen }: { onOpen: () => void }) {
  const { t } = useTranslation();
  const lines = useCart();
  const count = lines.reduce((n, l) => n + l.quantity, 0);
  return (
    <AnimatePresence>
      {count > 0 && (
        <motion.div
          className="fixed inset-x-0 bottom-0 z-30 mx-auto max-w-[480px] px-4 pb-[max(16px,env(safe-area-inset-bottom))]"
          initial={{ y: 96 }}
          animate={{ y: 0 }}
          exit={{ y: 96 }}
          transition={spring.gentle}
        >
          <button
            type="button"
            data-cart-target
            onClick={onOpen}
            className="flex h-14 w-full items-center gap-3 rounded-full bg-primary px-5 text-primary-foreground shadow-lg"
          >
            <span className="relative">
              <ShoppingBagIcon className="size-5" aria-hidden />
              {/* M6: the count pops with overshoot. */}
              <motion.span
                key={count}
                initial={{ scale: 0.3 }}
                animate={{ scale: 1 }}
                transition={spring.bouncy}
                className="tabular absolute -right-2.5 -top-2.5 grid size-5 place-items-center rounded-full bg-card text-[11px] font-bold text-foreground"
              >
                {count}
              </motion.span>
            </span>
            <span className="flex-1 text-left font-semibold">{t('cart.button', { count })}</span>
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export function CartSheet({ open, onClose, qr, menu, me }: { open: boolean; onClose: () => void; qr: string; menu: Menu; me: CustomerView }) {
  const { t } = useTranslation();
  const { of } = useName();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const lines = useCart();
  const items = useMemo(() => toItems(lines), [lines]);
  const [problems, setProblems] = useState<ItemProblem[]>([]);
  const [sent, setSent] = useState(false);
  const settling = me.bill.status === 'settling';

  const quote = useQuery({
    queryKey: ['quote', me.customer.id, items],
    queryFn: async () => unwrap(await api.POST('/public/cart/quote', { body: { items } })),
    enabled: open && items.length > 0,
    placeholderData: keepPreviousData,
  });
  // One key per cart content: a double tap or a retry of the same cart replays (TR-API-01).
  const key = useMemo(() => idempotencyKey(), [items]);
  const send = useMutation({
    mutationFn: async () => unwrap(await api.POST('/public/orders', { body: { items }, headers: { 'Idempotency-Key': key } })),
    onSuccess: (res) => {
      qc.setQueryData(['me', me.customer.id], res);
      cart.clear();
      setProblems([]);
      onClose();
      setSent(true);
    },
    onError: (e) => {
      const p = problemOf(e);
      if (p?.code === 'order_rejected') {
        setProblems((p.extra.items as ItemProblem[]) ?? []);
        void qc.invalidateQueries({ queryKey: ['menu'] });
      } else {
        toast.error(p?.detail ?? p?.message ?? t('errors.generic'));
      }
    },
  });

  const problemAt = (i: number) => problems.find((p) => p.index === i) ?? quote.data?.lines.find((l) => l.index === i)?.problem;
  const blocked = settling || items.length === 0 || send.isPending || lines.some((_, i) => {
    const p = problemAt(i);
    return p && p.code !== 'dietary_ack_required';
  });

  return (
    <>
      <BottomSheet
        open={open}
        onClose={onClose}
        title={t('cart.title')}
        footer={
          lines.length > 0 && (
            <div className="flex flex-col gap-2">
              {quote.data && (
                <dl className="flex flex-col gap-1 text-sm">
                  {quote.data.itemDiscount > 0 && (
                    <>
                      <div className="flex justify-between text-text-secondary"><dt>{t('cart.subtotal')}</dt><dd className="tabular">{formatRupiah(quote.data.subtotal)}</dd></div>
                      <div className="flex justify-between text-[var(--sl-color-feedback-success-fg)]"><dt>{t('cart.discount')}</dt><dd className="tabular">−{formatRupiah(quote.data.itemDiscount)}</dd></div>
                    </>
                  )}
                  <div className="flex justify-between text-base font-bold"><dt>{t('cart.total')}</dt><dd><RollingPrice value={quote.data.total} /></dd></div>
                  <p className="text-xs text-text-secondary">{t('cart.taxNote')}</p>
                </dl>
              )}
              {settling && (
                <p className="flex items-center gap-2 rounded-[16px] bg-[var(--sl-color-feedback-warning-bg)] px-3 py-2 text-sm font-semibold text-[var(--sl-color-feedback-warning-fg)]">
                  <HourglassIcon className="size-4 shrink-0" aria-hidden />
                  {t('menu.settling')}
                </p>
              )}
              <Button size="xl" disabled={blocked} onClick={() => send.mutate()}>
                {send.isPending && <LoaderIcon className="animate-spin" aria-hidden />}
                {send.isPending ? t('cart.sending') : t('cart.send')}
              </Button>
            </div>
          )
        }
      >
        {lines.length === 0 ? (
          <p className="py-10 text-center text-text-secondary">{t('cart.empty')}</p>
        ) : (
          <ul className="flex flex-col divide-y divide-border-subtle">
            {lines.map((l, i) => {
              const p = menu.products.find((x) => x.id === l.productId);
              const v = p?.variants.find((x) => x.id === l.variantId);
              const mods = p?.modifierGroups.flatMap((g) => g.modifiers).filter((m) => l.modifierIds.includes(m.id!)) ?? [];
              const q = quote.data?.lines.find((x) => x.index === i);
              const problem = problemAt(i);
              const warn = menu.dietaryTags.filter((x) => q?.warningTagIds.includes(x.id!));
              return (
                <li key={l.key} className="flex flex-col gap-2 py-3" data-cart-line={p?.nameEn}>
                  <div className="flex items-start gap-3">
                    <div className="min-w-0 flex-1">
                      <p className="font-semibold">{p ? of(p) : '—'}{v && p && p.variants.length > 1 && <span className="font-normal text-text-secondary"> · {of(v)}</span>}</p>
                      {mods.length > 0 && <p className="text-xs text-text-secondary">{mods.map((m) => of(m)).join(', ')}</p>}
                      {l.note && <p className="text-xs italic text-text-secondary">“{l.note}”</p>}
                    </div>
                    <div className="text-right">
                      {q && q.discount > 0 && <p className="tabular text-xs text-text-secondary line-through">{formatRupiah(q.gross)}</p>}
                      <p className="tabular font-semibold">{q ? formatRupiah(q.net) : '…'}</p>
                    </div>
                  </div>
                  {warn.length > 0 && <div className="flex flex-wrap gap-1">{warn.map((w) => <WarningBadge key={w.id} tag={w} />)}</div>}
                  {problem && (
                    <p className={cn('flex items-center gap-2 text-sm font-semibold', problem.code === 'dietary_ack_required' ? 'text-[var(--sl-color-dietary-warning-fg)]' : 'text-[var(--sl-color-feedback-danger-fg)]')}>
                      <AlertCircleIcon className="size-4 shrink-0" aria-hidden />
                      <span className="flex-1">{problem.message}</span>
                      {problem.code === 'dietary_ack_required' && (
                        <button type="button" className="underline" onClick={() => { cart.acknowledge(l.key, problem.tagIds ?? []); setProblems((ps) => ps.filter((x) => x.index !== i)); }}>
                          {t('warning.keep')}
                        </button>
                      )}
                    </p>
                  )}
                  <div className="flex items-center justify-between">
                    <Stepper value={l.quantity} onChange={(n) => cart.setQuantity(l.key, n)} label={t('product.quantity')} />
                    <button
                      type="button"
                      className="inline-flex h-10 items-center gap-1.5 rounded-full px-3 text-sm text-text-secondary"
                      onClick={() => {
                        const removed = cart.remove(l.key);
                        setProblems([]);
                        if (removed) toast(t('cart.removed', { name: p ? of(p) : '' }), { action: { label: t('cart.undo'), onClick: () => cart.restore(removed, i) } });
                      }}
                    >
                      <Trash2Icon className="size-4" aria-hidden />
                      {t('cart.remove')}
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </BottomSheet>
      <SentCelebration open={sent} onDone={() => { setSent(false); void navigate({ to: '/t/$qr/orders', params: { qr } }); }} />
    </>
  );
}

/** M8: the cup fills with latte from the bottom, sparkles burst, "Pesanan terkirim!". */
function SentCelebration({ open, onDone }: { open: boolean; onDone: () => void }) {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-[60] flex flex-col items-center justify-center gap-5 bg-background/95 px-8 text-center"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={ease.fade}
          role="status"
          data-testid="order-sent"
          onAnimationComplete={() => setTimeout(onDone, reduce ? 1200 : 1800)}
        >
          <CupFill level={1} fillFrom={0} celebrate className="w-40 drop-shadow-[0_12px_32px_rgba(242,136,91,0.45)]" />
          <h2 className="font-display text-2xl font-bold">{t('sent.title')}</h2>
          <p className="text-text-secondary">{t('sent.body')}</p>
          <Button variant="secondary" onClick={onDone}>{t('sent.view')}</Button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
