// OP-02 browsing: Active Products by Category (BR-CM-03), Sold Out greyed out (BR-CM-04),
// Dietary Warning badges for the Customer's own restriction (BR-CM-10).
import { formatPercentBp, formatRupiah, imageSrcSet, imageUrl, cn } from '@sushi-latte/ui';
import { CircleSlashIcon, HourglassIcon, PlusIcon, WifiOffIcon } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { CustomerView, DietaryTag, Menu, Product } from './api';
import { CartBar, CartSheet } from './cart-sheet';
import { useCart } from './cart';
import { PhotoPlaceholder, WarningBadge, ease, spring, useName } from './kit';
import { ProductSheet } from './product';
import { CallWaiterButton, DemoBanner, LanguageToggle, OrdersLink } from './shell';

/** TR-DM-05: the phone computes the same tag match as the server, for badges only. */
export function warningTags(p: Product, restriction: string[], tags: DietaryTag[], extra: string[] = []): DietaryTag[] {
  const contains = new Set([...p.dietaryTagIds, ...extra]);
  return tags.filter((t) => restriction.includes(t.id!) && contains.has(t.id!));
}

export const lowestPrice = (p: Product, promos: Menu['promos']) => {
  const avail = p.variants.filter((v) => !v.soldOut);
  const list = avail.length ? avail : p.variants;
  const prices = list.map((v) => promos[v.id!]?.price ?? v.price);
  const min = Math.min(...prices);
  return { price: min, from: new Set(list.map((v) => v.price)).size > 1, base: Math.min(...list.map((v) => v.price)) };
};

export function MenuScreen({ qr, me, menu, online }: { qr: string; me: CustomerView; menu: Menu; online: boolean }) {
  const { t } = useTranslation();
  const { of } = useName();
  const lines = useCart();
  // Keep the id, not the object: a live catalog refetch (Sold Out, price) must reach an open sheet.
  const [openId, setOpenId] = useState<string | null>(null);
  const openProduct = menu.products.find((p) => p.id === openId) ?? null;
  const [cartOpen, setCartOpen] = useState(false);
  const restriction = me.customer.restrictionTagIds;
  const settling = me.bill.status === 'settling';

  const sections = useMemo(
    () => menu.categories.map((c) => ({ category: c, products: menu.products.filter((p) => p.categoryId === c.id) })).filter((s) => s.products.length > 0),
    [menu],
  );
  const [active, setActive] = useState(sections[0]?.category.id);
  const sectionRefs = useRef(new Map<string, HTMLElement>());
  const barRef = useRef<HTMLDivElement>(null);

  // M2 scroll-spy: the pill follows the section nearest the top.
  useEffect(() => {
    const io = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) setActive(visible[0].target.getAttribute('data-category') ?? undefined);
      },
      { rootMargin: '-120px 0px -60% 0px' },
    );
    sectionRefs.current.forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, [sections]);
  useEffect(() => {
    barRef.current?.querySelector(`[data-pill="${active}"]`)?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'smooth' });
  }, [active]);

  const jump = (id: string) => {
    const el = sectionRefs.current.get(id);
    if (el) window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - 112, behavior: 'smooth' });
  };
  const activeOrders = me.orders.filter((o) => o.status !== 'cancelled' && o.status !== 'served').length;

  return (
    <div className="mx-auto min-h-dvh max-w-[480px] pb-32">
      <DemoBanner />
      <header className="sticky top-0 z-30 bg-background/95 backdrop-blur">
        <div className="flex items-center gap-2 px-4 pb-2 pt-3">
          <div className="min-w-0 flex-1">
            <p className="text-xs font-semibold text-text-secondary">{me.bill.tables.map((x) => x.label).join(' + ')}</p>
            <p className="truncate font-display text-lg font-bold">{t('menu.hello', { name: me.customer.name.split(' ')[0] })}</p>
          </div>
          <LanguageToggle />
          <OrdersLink qr={qr} count={activeOrders} />
        </div>
        <div className="flex items-center gap-2 px-4 pb-2">
          <CallWaiterButton me={me} />
          <p className="min-w-0 flex-1 text-right text-[11px] leading-tight text-text-secondary">
            {menu.priceMode === 'inclusive'
              ? t('menu.priceNoteInclusive')
              : t('menu.priceNote', { tax: formatPercentBp(menu.taxRateBp), service: formatPercentBp(menu.serviceRateBp) })}
          </p>
        </div>
        {/* M2 category bar with a sliding pill. */}
        <nav ref={barRef} className="no-scrollbar flex gap-1 overflow-x-auto px-4 pb-3" aria-label="Kategori">
          {sections.map(({ category }) => (
            <button
              key={category.id}
              type="button"
              data-pill={category.id}
              onClick={() => jump(category.id!)}
              aria-current={active === category.id}
              className="relative h-10 shrink-0 rounded-full px-4 text-sm font-semibold"
            >
              {active === category.id && <motion.span layoutId="category-pill" className="absolute inset-0 rounded-full bg-[var(--sl-color-brand-espresso)]" transition={spring.gentle} />}
              <span className={cn('relative', active === category.id && 'text-[var(--sl-color-brand-rice)]')}>{of(category)}</span>
            </button>
          ))}
        </nav>
        {settling && (
          <p className="flex items-center gap-2 bg-[var(--sl-color-feedback-warning-bg)] px-4 py-2 text-sm font-semibold text-[var(--sl-color-feedback-warning-fg)]" role="status">
            <HourglassIcon className="size-4 shrink-0" aria-hidden />
            {t('menu.settling')}
          </p>
        )}
        {!online && (
          <p className="flex items-center gap-2 bg-sunken px-4 py-1.5 text-xs text-text-secondary" role="status">
            <WifiOffIcon className="size-3.5" aria-hidden />
            {t('errors.offline')}
          </p>
        )}
      </header>

      {sections.map(({ category, products }) => (
        <section
          key={category.id}
          data-category={category.id}
          ref={(el) => {
            if (el) sectionRefs.current.set(category.id!, el);
          }}
          className="px-4 pt-5"
          aria-labelledby={`cat-${category.id}`}
        >
          <h2 id={`cat-${category.id}`} className="mb-3 font-display text-xl font-bold">{of(category)}</h2>
          <div className="grid grid-cols-2 gap-3">
            {products.map((p, i) => (
              <ProductCard
                key={p.id}
                product={p}
                feature={i === 0}
                index={i}
                menu={menu}
                warnings={warningTags(p, restriction, menu.dietaryTags)}
                inCart={lines.filter((l) => l.productId === p.id).reduce((n, l) => n + l.quantity, 0)}
                onOpen={() => setOpenId(p.id!)}
              />
            ))}
          </div>
        </section>
      ))}

      <ProductSheet product={openProduct} menu={menu} restriction={restriction} onClose={() => setOpenId(null)} />
      <CartBar onOpen={() => setCartOpen(true)} />
      <CartSheet open={cartOpen} onClose={() => setCartOpen(false)} qr={qr} menu={menu} me={me} />
    </div>
  );
}

function ProductCard({ product: p, feature, index, menu, warnings, inCart, onOpen }: {
  product: Product;
  feature: boolean;
  index: number;
  menu: Menu;
  warnings: DietaryTag[];
  inCart: number;
  onOpen: () => void;
}) {
  const { t } = useTranslation();
  const { of, desc } = useName();
  const reduce = useReducedMotion();
  const soldOut = p.variants.every((v) => v.soldOut);
  const { price, from, base } = lowestPrice(p, menu.promos);
  const promo = price < base;
  const label = `${soldOut ? `${t('menu.soldOut')}, ` : ''}${of(p)}, ${formatRupiah(price)}`;
  return (
    // M3: rise 16 px + fade, staggered 40 ms, once per section; max 8 staggered.
    <motion.button
      type="button"
      onClick={onOpen}
      aria-label={label}
      data-product={p.nameEn}
      initial={reduce ? { opacity: 0 } : { opacity: 0, y: 16 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: '0px 0px -40px 0px' }}
      transition={{ ...ease.expressive, delay: Math.min(index, 8) * 0.04 }}
      whileTap={{ scale: 0.98 }}
      className={cn(
        'group flex flex-col overflow-hidden rounded-[20px] bg-card text-left shadow-[var(--sl-product-card-shadow)]',
        feature && 'col-span-2',
      )}
    >
      <div className={cn('relative w-full overflow-hidden', feature ? 'aspect-[16/10]' : 'aspect-square')}>
        {p.imageKey ? (
          <motion.img
            layoutId={`photo-${p.id}`}
            src={imageUrl(p.imageKey, feature ? 'sq640' : 'sq320')}
            srcSet={imageSrcSet(p.imageKey)}
            sizes={feature ? '(max-width: 480px) 100vw, 480px' : '(max-width: 480px) 50vw, 240px'}
            alt=""
            loading={index < 4 ? 'eager' : 'lazy'}
            fetchPriority={feature ? 'high' : 'auto'}
            className="size-full object-cover transition-[filter] duration-[450ms]"
            style={{ filter: soldOut ? 'grayscale(0.45) opacity(0.7)' : undefined }}
          />
        ) : (
          <PhotoPlaceholder className="size-full" />
        )}
        {warnings.length > 0 && (
          <div className="absolute left-2 top-2 flex flex-col items-start gap-1">
            {warnings.slice(0, 2).map((w) => <WarningBadge key={w.id} tag={w} className="shadow-sm" />)}
          </div>
        )}
        <AnimatePresence>
          {soldOut && (
            // M11: the Habis label slides in.
            <motion.span
              initial={{ opacity: 0, x: -12 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0 }}
              transition={ease.slow}
              className="absolute bottom-2 left-2 inline-flex h-6 items-center gap-1 rounded-full px-2.5 text-xs font-bold"
              style={{ background: 'var(--sl-color-sold-out-badge-bg)', color: 'var(--sl-color-sold-out-badge-fg)' }}
            >
              <CircleSlashIcon className="size-3.5" aria-hidden />
              {t('menu.soldOut')}
            </motion.span>
          )}
        </AnimatePresence>
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <span className="line-clamp-2 font-semibold leading-snug">{of(p)}</span>
        {feature && <span className="line-clamp-2 text-xs text-text-secondary">{desc(p)}</span>}
        <span className="mt-auto flex items-end justify-between gap-2 pt-1">
          <span className="flex flex-col leading-tight">
            {promo && <span className="tabular text-[11px] text-text-secondary line-through">{formatRupiah(base)}</span>}
            <span className="tabular font-bold text-[var(--sl-product-card-price-color)]">
              {from && <span className="mr-1 text-[11px] font-semibold text-text-secondary">{t('menu.from')}</span>}
              {formatRupiah(price)}
            </span>
          </span>
          {!soldOut &&
            (inCart > 0 ? (
              <span className="tabular grid h-11 min-w-11 place-items-center rounded-full bg-[var(--sl-color-brand-espresso)] px-2 text-sm font-bold text-[var(--sl-color-brand-rice)]">×{inCart}</span>
            ) : (
              <span className="grid size-11 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground" aria-hidden>
                <PlusIcon className="size-5" />
              </span>
            ))}
        </span>
      </div>
    </motion.button>
  );
}
