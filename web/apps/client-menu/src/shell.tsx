// Header pieces shared by the Client Menu screens (LAYOUT.md §3: Table label + language toggle
// + call-Waiter button, sticky).
import { api, idempotencyKey, unwrap } from '@sushi-latte/api-client';
import { cn, domainIcons } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ClipboardListIcon, FlaskConicalIcon } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { healthQuery, type CustomerView } from './api';
import { setLang, type Lang } from './i18n';
import { spring } from './kit';

export function LanguageToggle() {
  const { i18n, t } = useTranslation();
  const langs: Lang[] = ['id', 'en'];
  return (
    <div className="inline-flex rounded-full bg-sunken p-1" role="group" aria-label={t('common.language')}>
      {langs.map((l) => (
        <button
          key={l}
          type="button"
          aria-pressed={i18n.language === l}
          onClick={() => setLang(l)}
          className={cn('h-8 rounded-full px-3 text-xs font-bold uppercase', i18n.language === l ? 'bg-card shadow-sm' : 'text-text-secondary')}
        >
          {l}
        </button>
      ))}
    </div>
  );
}

/** Call Waiter (BR-CM-07): one open call at a time; shows "Waiter called" until answered. */
export function CallWaiterButton({ me }: { me: CustomerView }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const call = useMutation({
    mutationFn: async () => unwrap(await api.POST('/public/waiter-calls', { headers: { 'Idempotency-Key': idempotencyKey() } })),
    onSuccess: () => {
      toast.success(t('menu.waiterCalledToast'));
      void qc.invalidateQueries({ queryKey: ['me'] });
    },
    onError: () => toast.error(t('errors.generic')),
  });
  const Icon = domainIcons.callWaiter;
  const called = !!me.waiterCall;
  return (
    <button
      type="button"
      onClick={() => !called && call.mutate()}
      disabled={call.isPending}
      aria-pressed={called}
      className={cn(
        'inline-flex h-10 items-center gap-1.5 rounded-full px-3 text-sm font-semibold',
        called ? 'bg-[var(--sl-color-feedback-success-bg)] text-[var(--sl-color-feedback-success-fg)]' : 'bg-sunken',
      )}
    >
      <Icon className="size-4" aria-hidden />
      {called ? t('menu.waiterCalled') : t('menu.callWaiter')}
    </button>
  );
}

export function OrdersLink({ qr, count }: { qr: string; count: number }) {
  const { t } = useTranslation();
  return (
    <Link to="/t/$qr/orders" params={{ qr }} className="relative inline-flex size-10 items-center justify-center rounded-full bg-sunken" aria-label={t('menu.orders')}>
      <ClipboardListIcon className="size-5" aria-hidden />
      {count > 0 && (
        <motion.span
          key={count}
          initial={{ scale: 0.4 }}
          animate={{ scale: 1 }}
          transition={spring.bouncy}
          className="tabular absolute -right-1 -top-1 grid size-5 place-items-center rounded-full bg-primary text-[11px] font-bold text-primary-foreground"
        >
          {count}
        </motion.span>
      )}
    </Link>
  );
}

/** Demo banner (DEMO-AND-SEED §2) with a link to the launcher. */
export function DemoBanner() {
  const { t } = useTranslation();
  const { data } = useQuery(healthQuery);
  if (!data?.demo) return null;
  return (
    <a href="/demo" className="flex h-7 items-center justify-center gap-1.5 bg-[var(--sl-color-brand-nori)] text-xs font-semibold text-[var(--sl-color-brand-rice)]">
      <FlaskConicalIcon className="size-3.5" aria-hidden />
      {t('demo.banner')} · {t('demo.launcher')}
    </a>
  );
}
