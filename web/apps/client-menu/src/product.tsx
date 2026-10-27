// ProductDetailSheet, VariantPicker, ModifierGroup and DietaryWarningDialog (COMPONENTS.md §2).
import { Button, cn, dietaryIcons, formatRupiah, imageUrl } from '@sushi-latte/ui';
import { AlertCircleIcon, CheckIcon } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { DietaryTag, Menu, Product } from './api';
import { cart } from './cart';
import { fly } from './flight';
import { BottomSheet, ContainsTag, PhotoPlaceholder, RollingPrice, Steam, Stepper, ease, useName } from './kit';
import { warningTags } from './menu';

export function ProductSheet({ product, menu, restriction, onClose }: { product: Product | null; menu: Menu; restriction: string[]; onClose: () => void }) {
  // Keep the last product mounted while the sheet animates out.
  const last = useRef(product);
  if (product) last.current = product;
  return (
    <BottomSheet open={!!product} onClose={onClose} labelledBy="product-title">
      {last.current && <Detail key={last.current.id} product={last.current} menu={menu} restriction={restriction} onDone={onClose} />}
    </BottomSheet>
  );
}

function Hero({ product: p, hot }: { product: Product; hot: boolean }) {
  return (
    <div className="relative -mx-5 aspect-[4/3] shrink-0 overflow-hidden" data-flight-source>
      {p.imageKey ? (
        // M4: the card photo expands into the sheet header.
        <motion.img layoutId={`photo-${p.id}`} src={imageUrl(p.imageKey, 'sq640')} alt="" className="size-full object-cover" />
      ) : (
        <PhotoPlaceholder className="size-full" />
      )}
      {/* M4: hot drinks show the steam loop in the header. */}
      {/* M4: a hot drink steams, rising from the photo's focal point (the cup, when set). */}
      {hot && (
        <Steam
          className="absolute w-1/5 -translate-x-1/2 opacity-60"
          style={{ left: `${p.imageFocus?.x ?? 50}%`, top: `${Math.max(0, (p.imageFocus?.y ?? 50) - 34)}%` }}
        />
      )}
    </div>
  );
}

function Detail({ product: p, menu, restriction, onDone }: { product: Product; menu: Menu; restriction: string[]; onDone: () => void }) {
  const { t } = useTranslation();
  const { of, desc } = useName();
  const reduce = useReducedMotion();
  const firstAvailable = p.variants.find((v) => v.isDefault && !v.soldOut) ?? p.variants.find((v) => !v.soldOut);
  const [variantId, setVariantId] = useState(firstAvailable?.id);
  const [picked, setPicked] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(p.modifierGroups.map((g) => [g.id!, g.minSelect === 1 && g.maxSelect === 1 && g.modifiers[0] ? [g.modifiers[0].id!] : []])),
  );
  const [quantity, setQuantity] = useState(1);
  const [note, setNote] = useState('');
  const [confirming, setConfirming] = useState<DietaryTag[] | null>(null);

  const variant = p.variants.find((v) => v.id === variantId);
  const modifiers = p.modifierGroups.flatMap((g) => g.modifiers.filter((m) => picked[g.id!]?.includes(m.id!)));
  const modTagIds = modifiers.flatMap((m) => m.dietaryTagIds);
  const warnings = useMemo(() => warningTags(p, restriction, menu.dietaryTags, modTagIds), [p, restriction, menu.dietaryTags, modTagIds]);
  const contains = menu.dietaryTags.filter((tag) => p.dietaryTagIds.includes(tag.id!));
  const unit = (variant ? (menu.promos[variant.id!]?.price ?? variant.price) : 0) + modifiers.reduce((n, m) => n + m.price, 0);
  const unmet = p.modifierGroups.find((g) => (picked[g.id!]?.length ?? 0) < g.minSelect);
  const soldOut = !variant || variant.soldOut;
  const tagNames = (tags: DietaryTag[]) => tags.map((x) => of(x).toLowerCase()).join(', ');

  const toggle = (groupId: string, modifierId: string, single: boolean, max: number) => {
    setPicked((cur) => {
      const now = cur[groupId] ?? [];
      if (single) return { ...cur, [groupId]: [modifierId] };
      if (now.includes(modifierId)) return { ...cur, [groupId]: now.filter((x) => x !== modifierId) };
      if (now.length >= max) return cur;
      return { ...cur, [groupId]: [...now, modifierId] };
    });
  };

  const add = (acknowledged: DietaryTag[]) => {
    if (!variant) return;
    cart.add({ productId: p.id!, variantId: variant.id!, modifierIds: modifiers.map((m) => m.id!), quantity, note: note.trim(), acknowledgedTagIds: acknowledged.map((x) => x.id!) });
    // M6: thumbnail flies into the cart; haptic tick where supported.
    const source = document.querySelector('[data-flight-source]')?.getBoundingClientRect();
    if (source && p.imageKey && !reduce) fly(imageUrl(p.imageKey, 'sq320'), source);
    navigator.vibrate?.(10);
    setConfirming(null);
    onDone();
  };

  useEffect(() => {
    if (variant?.soldOut) setVariantId(p.variants.find((v) => !v.soldOut)?.id);
  }, [variant?.soldOut, p.variants]);

  return (
    <div className="flex flex-col gap-5">
      <Hero product={p} hot={!!variant && /\bhot\b/i.test(variant.nameEn)} />
      <div>
        <h2 id="product-title" className="font-display text-2xl font-bold leading-tight">{of(p)}</h2>
        <p className="mt-1.5 text-sm text-text-secondary">{desc(p)}</p>
        {contains.length > 0 && (
          <div className="mt-3">
            <p className="mb-1.5 text-xs font-semibold text-text-secondary">{t('product.contains')}</p>
            <div className="flex flex-wrap gap-1.5">{contains.map((x) => <ContainsTag key={x.id} tag={x} />)}</div>
          </div>
        )}
      </div>

      {p.variants.length > 1 && (
        <fieldset>
          <legend className="mb-2 font-semibold">{t('product.variant')}</legend>
          <div className="flex flex-wrap gap-2" role="radiogroup">
            {p.variants.map((v) => {
              const on = v.id === variantId;
              const diff = v.price - (firstAvailable?.price ?? v.price);
              return (
                <button
                  key={v.id}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  disabled={v.soldOut}
                  onClick={() => setVariantId(v.id)}
                  className={cn(
                    'inline-flex h-11 items-center gap-2 rounded-full border px-4 text-sm font-semibold transition-colors',
                    on ? 'border-[var(--sl-color-brand-espresso)] bg-[var(--sl-color-brand-espresso)] text-[var(--sl-color-brand-rice)]' : 'border-border bg-card',
                    v.soldOut && 'border-dashed text-text-secondary line-through opacity-60',
                  )}
                >
                  {on && <CheckIcon className="size-4" aria-hidden />}
                  {of(v)}
                  {v.soldOut ? <span className="no-underline">· {t('menu.soldOut')}</span> : diff > 0 && <span className="tabular opacity-80">+{formatRupiah(diff)}</span>}
                </button>
              );
            })}
          </div>
        </fieldset>
      )}

      {p.modifierGroups.map((g) => {
        const single = g.maxSelect === 1;
        const rule = g.minSelect > 0 ? t('product.required', { n: g.minSelect }) : t('product.optional', { n: g.maxSelect });
        return (
          <fieldset key={g.id}>
            <legend className="mb-2 flex w-full items-baseline justify-between font-semibold">
              {of(g)}
              <span className="text-xs font-medium text-text-secondary">{rule}</span>
            </legend>
            <div className="flex flex-col divide-y divide-border-subtle rounded-[20px] border border-border-subtle">
              {g.modifiers.map((m) => {
                const on = picked[g.id!]?.includes(m.id!) ?? false;
                const tagIcons = menu.dietaryTags.filter((x) => m.dietaryTagIds.includes(x.id!));
                return (
                  <label key={m.id} className="flex min-h-12 cursor-pointer items-center gap-3 px-4 py-2">
                    <input
                      type={single ? 'radio' : 'checkbox'}
                      name={g.id}
                      checked={on}
                      onChange={() => toggle(g.id!, m.id!, single, g.maxSelect)}
                      className="size-5 accent-[var(--sl-color-brand-salmon-deep)]"
                    />
                    <span className="flex-1 text-sm">{of(m)}</span>
                    {tagIcons.map((x) => {
                      const Icon = dietaryIcons[x.code];
                      return Icon ? <Icon key={x.id} className="size-4 text-text-secondary" aria-label={of(x)} /> : null;
                    })}
                    {m.price > 0 && <span className="tabular text-sm text-text-secondary">+{formatRupiah(m.price)}</span>}
                  </label>
                );
              })}
            </div>
          </fieldset>
        );
      })}

      <label className="block text-sm font-semibold">
        {t('product.note')}
        <input
          value={note}
          maxLength={140}
          onChange={(e) => setNote(e.target.value)}
          placeholder={t('product.notePlaceholder')}
          className="mt-1.5 h-12 w-full rounded-full border border-input bg-card px-4 text-base font-normal outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </label>

      <AnimatePresence>
        {warnings.length > 0 && (
          // M7: Plum banner eases in; never red, never a shake.
          <motion.p
            role="status"
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={ease.expressive}
            className="flex items-start gap-2 rounded-[20px] border px-4 py-3 text-sm font-semibold"
            style={{ background: 'var(--sl-color-dietary-warning-bg)', color: 'var(--sl-color-dietary-warning-fg)', borderColor: 'var(--sl-color-dietary-warning-border)' }}
          >
            <AlertCircleIcon className="mt-0.5 size-4 shrink-0" aria-hidden />
            {t('product.warningBanner', { tags: tagNames(warnings) })}
          </motion.p>
        )}
      </AnimatePresence>

      <div className="sticky bottom-0 -mx-5 flex items-center gap-3 border-t border-border-subtle bg-card px-5 pb-[max(12px,env(safe-area-inset-bottom))] pt-3">
        <Stepper value={quantity} onChange={setQuantity} label={t('product.quantity')} />
        <Button
          size="lg"
          className="h-14 min-w-0 flex-1 flex-col gap-0 px-4 leading-tight"
          disabled={soldOut || !!unmet}
          onClick={() => (warnings.length > 0 ? setConfirming(warnings) : add([]))}
        >
          {unmet ? (
            <span className="truncate text-sm">{t('product.chooseFirst', { group: of(unmet) })}</span>
          ) : (
            <>
              <span className="text-sm font-semibold">{t('product.addToOrder')}</span>
              <RollingPrice value={unit * quantity} className="text-base font-bold" />
            </>
          )}
        </Button>
      </div>

      <BottomSheet open={!!confirming} onClose={() => setConfirming(null)} title={t('warning.title')}>
        {confirming && (
          <div className="flex flex-col gap-4 pb-2">
            <p className="flex items-start gap-3 text-base">
              <AlertCircleIcon className="mt-0.5 size-6 shrink-0" style={{ color: 'var(--sl-color-dietary-warning-icon)' }} aria-hidden />
              {t('warning.body', { tags: tagNames(confirming) })}
            </p>
            <Button size="lg" onClick={() => add(confirming)}>{t('warning.keep')}</Button>
            <Button size="lg" variant="secondary" onClick={() => { setConfirming(null); onDone(); }}>{t('warning.other')}</Button>
          </div>
        )}
      </BottomSheet>
    </div>
  );
}
