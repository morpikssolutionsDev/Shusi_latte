// Customer-surface building blocks (COMPONENTS.md §2, MOTION.md expressive profile).
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cn, dietaryIcons, formatRupiah, motion as tokens } from '@sushi-latte/ui';
import { MinusIcon, PlusIcon, XIcon } from 'lucide-react';
import { AnimatePresence, animate, motion, useMotionValue, useReducedMotion, useTransform } from 'motion/react';
import { useEffect, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { DietaryTag } from './api';

const ms = (d: string) => parseInt(d, 10) / 1000;

export const spring = {
  gentle: { type: 'spring', stiffness: tokens.SpringGentleStiffness, damping: tokens.SpringGentleDamping, mass: tokens.SpringGentleMass },
  bouncy: { type: 'spring', stiffness: tokens.SpringBouncyStiffness, damping: tokens.SpringBouncyDamping, mass: tokens.SpringBouncyMass },
} as const;

export const ease = {
  expressive: { duration: ms(tokens.DurationModerate), ease: tokens.EasingExpressive },
  slow: { duration: ms(tokens.DurationSlow), ease: tokens.EasingExpressive },
  fade: { duration: ms(tokens.DurationFast), ease: tokens.EasingStandard },
};

/** Picks the Indonesian or English field for the current language (TR-I18N-01). */
export function useName() {
  const { i18n } = useTranslation();
  const en = i18n.language === 'en';
  return {
    en,
    of: (x: { nameId: string; nameEn: string }) => (en ? x.nameEn || x.nameId : x.nameId),
    desc: (x: { descriptionId?: string; descriptionEn?: string }) => (en ? x.descriptionEn || x.descriptionId : x.descriptionId) ?? '',
    bi: (x: { id: string; en: string }) => (en ? x.en || x.id : x.id),
  };
}

/**
 * Bottom sheet with a 28 px top radius sliding up on spring `gentle` (LAYOUT.md §3, M12). Under
 * reduced motion it fades (MOTION.md §4).
 */
export function BottomSheet({ open, onClose, title, children, footer, hero, labelledBy }: {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  footer?: ReactNode;
  hero?: ReactNode;
  labelledBy?: string;
}) {
  const reduce = useReducedMotion();
  const { t } = useTranslation();
  return (
    <DialogPrimitive.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <AnimatePresence>
        {open && (
          <DialogPrimitive.Portal forceMount>
            <DialogPrimitive.Overlay asChild forceMount>
              <motion.div className="fixed inset-0 z-50 bg-[var(--sl-color-bg-scrim)]/50" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={ease.fade} />
            </DialogPrimitive.Overlay>
            <DialogPrimitive.Content asChild forceMount aria-describedby={undefined} aria-labelledby={labelledBy}>
              <motion.div
                className="fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[92dvh] w-full max-w-[480px] flex-col overflow-hidden rounded-t-[28px] bg-card text-card-foreground shadow-xl focus:outline-none"
                initial={reduce ? { opacity: 0 } : { y: '100%' }}
                animate={reduce ? { opacity: 1 } : { y: 0 }}
                exit={reduce ? { opacity: 0 } : { y: '100%' }}
                transition={reduce ? ease.fade : spring.gentle}
              >
                {title ? (
                  <header className="flex items-center gap-3 px-5 pb-2 pt-5">
                    <DialogPrimitive.Title className="min-w-0 flex-1 font-display text-xl font-bold">{title}</DialogPrimitive.Title>
                    <DialogPrimitive.Close className="grid size-10 place-items-center rounded-full bg-sunken text-text-secondary" aria-label={t('common.close')}>
                      <XIcon className="size-5" aria-hidden />
                    </DialogPrimitive.Close>
                  </header>
                ) : (
                  <DialogPrimitive.Title className="sr-only">{labelledBy ? '' : 'Sushi Latte'}</DialogPrimitive.Title>
                )}
                {hero}
                <div className="flex-1 overflow-y-auto overflow-x-hidden overscroll-contain px-5 pb-4">{children}</div>
                {footer && <footer className="border-t border-border-subtle bg-card px-5 pb-[max(16px,env(safe-area-inset-bottom))] pt-3">{footer}</footer>}
              </motion.div>
            </DialogPrimitive.Content>
          </DialogPrimitive.Portal>
        )}
      </AnimatePresence>
    </DialogPrimitive.Root>
  );
}

/** Plum Dietary Warning badge with its tag icon (DietaryBadge compact, M7). Never icon-only. */
export function WarningBadge({ tag, className }: { tag: DietaryTag | { code: string; nameId: string; nameEn: string }; className?: string }) {
  const { t } = useTranslation();
  const { of } = useName();
  const Icon = dietaryIcons[tag.code];
  return (
    <motion.span
      role="status"
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={ease.expressive}
      className={cn('inline-flex h-7 items-center gap-1.5 rounded-chip border px-2.5 text-xs font-semibold', className)}
      style={{ background: 'var(--sl-color-dietary-warning-bg)', color: 'var(--sl-color-dietary-warning-fg)', borderColor: 'var(--sl-color-dietary-warning-border)' }}
    >
      {Icon && <Icon className="size-3.5" aria-hidden />}
      {t('warning.badge', { tag: of(tag).toLowerCase() })}
    </motion.span>
  );
}

/** Neutral "Contains" tag with icon (Product detail). */
export function ContainsTag({ tag }: { tag: DietaryTag }) {
  const { of } = useName();
  const Icon = dietaryIcons[tag.code];
  return (
    <span className="inline-flex h-7 items-center gap-1.5 rounded-chip bg-sunken px-2.5 text-xs font-medium text-text-secondary">
      {Icon && <Icon className="size-3.5" aria-hidden />}
      {of(tag)}
    </span>
  );
}

export function Stepper({ value, onChange, min = 1, max = 20, label }: { value: number; onChange: (n: number) => void; min?: number; max?: number; label: string }) {
  return (
    <div className="inline-flex items-center gap-1 rounded-full bg-sunken p-1" role="group" aria-label={label}>
      <button type="button" className="grid size-9 place-items-center rounded-full bg-card shadow-sm disabled:opacity-40" disabled={value <= min} onClick={() => onChange(value - 1)} aria-label="−1">
        <MinusIcon className="size-4" aria-hidden />
      </button>
      <span className="tabular w-8 text-center font-bold" aria-live="polite">{value}</span>
      <button type="button" className="grid size-9 place-items-center rounded-full bg-card shadow-sm disabled:opacity-40" disabled={value >= max} onClick={() => onChange(value + 1)} aria-label="+1">
        <PlusIcon className="size-4" aria-hidden />
      </button>
    </div>
  );
}

/** Price that rolls to its new value (M5). */
export function RollingPrice({ value, className }: { value: number; className?: string }) {
  const reduce = useReducedMotion();
  const mv = useMotionValue(value);
  const text = useTransform(mv, (v) => formatRupiah(Math.round(v)));
  useEffect(() => {
    if (reduce) {
      mv.set(value);
      return;
    }
    const controls = animate(mv, value, { duration: ms(tokens.DurationBase), ease: tokens.EasingStandard });
    return () => controls.stop();
  }, [value, reduce, mv]);
  return <motion.span className={cn('tabular', className)}>{text}</motion.span>;
}

/** Rising steam curls, the logo's ambient loop (MOTION.md §1); hidden under reduced motion. */
export function Steam({ className, style }: { className?: string; style?: React.CSSProperties }) {
  return (
    <svg viewBox="0 0 120 120" className={cn('cm-steam pointer-events-none', className)} style={style} aria-hidden fill="none" stroke="#FAF6F1" strokeWidth="7" strokeLinecap="round">
      <path d="M30 110C18 90 42 76 30 56C20 40 36 28 30 14" />
      <path d="M60 110C48 90 72 76 60 56C50 40 66 28 60 14" />
      <path d="M90 110C78 90 102 76 90 56C80 40 96 28 90 14" />
    </svg>
  );
}

/** Branded placeholder for Products without a photo (TR-DM-09). */
export function PhotoPlaceholder({ className }: { className?: string }) {
  return (
    <div className={cn('grid place-items-center bg-[var(--sl-color-rice-100)]', className)}>
      <img src="/favicon.svg" alt="" className="w-1/3 opacity-70" />
    </div>
  );
}
