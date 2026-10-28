import type { Schemas } from '@sushi-latte/api-client';
import { Logo, cn, formatRupiah, imageUrl, videoUrl, wideImageUrl } from '@sushi-latte/ui';
import { motion, useReducedMotion } from 'motion/react';
import type { ReactNode } from 'react';
import { Countdown, Sparkles, Steam, useNow } from './decor';
import { useStage } from './stage';

export type PlaylistSlide = Schemas['PlaylistSlide'];
type DisplayProduct = Schemas['DisplayProduct'];

const ease = [0.22, 1, 0.36, 1] as const;
const overshoot = [0.34, 1.56, 0.64, 1] as const;
/** 64 px safe area on every edge (LAYOUT.md §3, TV overscan). */
const SAFE = 64;

/** Entrance that collapses to a fade under reduced motion (MOTION.md §4). */
function Enter({ children, delay = 0, y = 32, x = 0, className }: { children: ReactNode; delay?: number; y?: number; x?: number; className?: string }) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      className={className}
      initial={reduce ? { opacity: 0 } : { opacity: 0, y, x }}
      animate={{ opacity: 1, y: 0, x: 0 }}
      transition={{ duration: reduce ? 0.15 : 0.6, delay: reduce ? 0 : delay, ease }}
    >
      {children}
    </motion.div>
  );
}

/** Slow Ken Burns zoom across the slide's dwell time; `focus` keeps the subject in frame when cropped. */
function KenBurns({ src, duration, className, from = 1, to = 1.08, focus }: { src: string; duration: number; className?: string; from?: number; to?: number; focus?: { x: number; y: number } }) {
  const reduce = useReducedMotion();
  return (
    <motion.img
      src={src}
      alt=""
      className={cn('size-full object-cover', className)}
      style={focus && { objectPosition: `${focus.x}% ${focus.y}%`, transformOrigin: `${focus.x}% ${focus.y}%` }}
      initial={{ scale: reduce ? 1 : from }}
      animate={{ scale: reduce ? 1 : to }}
      transition={{ duration, ease: 'linear' }}
    />
  );
}

/** Indonesian first; the English line follows smaller and lighter (TR-I18N-01). */
function Bilingual({ id, en, enClassName }: { id: string; en: string; enClassName?: string }) {
  return (
    <>
      <span className="block">{id}</span>
      {en && en !== id && (
        <span className={cn('mt-3 block text-[0.46em] font-semibold leading-tight tracking-[-0.005em] text-[var(--sl-color-text-secondary)]', enClassName)} lang="en">
          {en}
        </span>
      )}
    </>
  );
}

function Kicker({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn('text-[28px] font-bold uppercase leading-tight tracking-[0.12em] text-[var(--sl-color-brand-salmon)]', className)}>{children}</p>;
}

function QrGlyph({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="6" height="6" rx="1" />
      <rect x="15" y="3" width="6" height="6" rx="1" />
      <rect x="3" y="15" width="6" height="6" rx="1" />
      <path d="M15 15h2v2h-2zM19 15h2M15 21h2M19 19v2h2M19 15v2" />
    </svg>
  );
}

/** The Display's one call to action: order from the Table's QR (BR-PD). */
function OrderHint({ delay = 1.5 }: { delay?: number }) {
  return (
    <Enter delay={delay} y={16}>
      <p className="inline-flex items-center gap-5 rounded-full border border-white/15 bg-white/[0.06] py-4 pl-5 pr-8 text-[26px] font-semibold leading-tight text-[var(--sl-color-text-primary)]">
        <span className="grid size-14 shrink-0 place-items-center rounded-full bg-[var(--sl-color-action-brand-bg)] text-[var(--sl-color-action-brand-fg)]">
          <QrGlyph className="size-8" />
        </span>
        <span>
          Pindai QR di mejamu untuk memesan
          <span className="block text-[20px] font-medium text-[var(--sl-color-text-secondary)]" lang="en">Scan the QR on your table to order</span>
        </span>
      </p>
    </Enter>
  );
}

function PriceTag({ product, delay = 0.9 }: { product: DisplayProduct; delay?: number }) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      className="inline-flex shrink-0 items-baseline gap-4 whitespace-nowrap rounded-full bg-[var(--sl-color-action-brand-bg)] px-10 py-5 text-[var(--sl-color-action-brand-fg)] shadow-[var(--sl-shadow-glow)]"
      initial={reduce ? { opacity: 0 } : { opacity: 0, scale: 0.4, rotate: -6 }}
      animate={{ opacity: 1, scale: 1, rotate: 0 }}
      transition={{ duration: reduce ? 0.15 : 0.7, delay: reduce ? 0 : delay, ease: overshoot }}
    >
      {product.priceFrom && <span className="text-[32px] font-bold">mulai</span>}
      <span className="tabular text-[72px] font-extrabold leading-none">{formatRupiah(product.price)}</span>
    </motion.div>
  );
}

// ---------------------------------------------------------------- templates

export function HeroPromo({ slide }: { slide: PlaylistSlide }) {
  const { portrait } = useStage();
  return (
    <div className="relative size-full overflow-hidden bg-[var(--sl-color-bg-canvas)]">
      {slide.imageKey && (
        <KenBurns src={wideImageUrl(slide.imageKey, slide.imageFocus)} focus={slide.imageFocus} duration={slide.durationSeconds + 1} from={1.08} to={1} />
      )}
      <div
        className={cn(
          'absolute inset-0',
          portrait
            ? 'bg-gradient-to-t from-[var(--sl-color-bg-canvas)] via-[var(--sl-color-bg-canvas)]/70 to-transparent'
            : 'bg-gradient-to-r from-[var(--sl-color-bg-canvas)]/95 via-[var(--sl-color-bg-canvas)]/60 to-transparent',
        )}
      />
      <Sparkles points={[{ x: 48, y: 22, size: 2.2, delay: 0.8 }, { x: 86, y: 12, size: 1.4, delay: 1.6 }, { x: 6, y: 90, size: 1.6, delay: 2.2 }]} />
      <div
        className={cn('absolute flex flex-col gap-8 text-[var(--sl-color-text-primary)]', portrait ? 'inset-x-0 bottom-0 justify-end' : 'inset-y-0 left-0 max-w-[1040px] justify-center')}
        style={{ padding: portrait ? `${SAFE}px ${SAFE}px ${SAFE * 1.5}px` : `${SAFE}px ${SAFE * 2}px` }}
      >
        <Enter delay={0.2} x={portrait ? 0 : -48} y={portrait ? 24 : 0}>
          <Kicker>Sushi Latte</Kicker>
        </Enter>
        <Enter delay={0.35} x={portrait ? 0 : -48} y={portrait ? 24 : 0}>
          {/* Size before leading: tailwind-merge drops a leading-* that precedes a text-* size. */}
          <h1 className={cn('font-display font-bold tracking-[-0.02em]', portrait ? 'text-[112px]' : 'text-[104px]', 'leading-[1.02]')}>
            <Bilingual id={slide.headline.id} en={slide.headline.en} />
          </h1>
        </Enter>
        {slide.subline.id && (
          <Enter delay={0.8}>
            <p className="max-w-[880px] text-[40px] font-semibold leading-snug">
              {slide.subline.id}
              {slide.subline.en && slide.subline.en !== slide.subline.id && (
                <span className="mt-1 block text-[28px] font-medium text-[var(--sl-color-text-secondary)]" lang="en">{slide.subline.en}</span>
              )}
            </p>
          </Enter>
        )}
      </div>
    </div>
  );
}

export function FeaturedProduct({ slide }: { slide: PlaylistSlide }) {
  const { width, height, portrait } = useStage();
  const p = slide.product!;
  // The photo is the hero: as large as the stage allows next to (or above) the text column,
  // leaving room above it for the steam.
  const photo = portrait ? Math.min(width - SAFE * 3, height - SAFE * 5 - 720) : Math.min(height - SAFE * 4, (width - SAFE * 4) * 0.48);
  const compact = !portrait && width < 1800;
  return (
    <div
      className={cn('relative flex size-full items-center bg-[var(--sl-color-bg-canvas)] text-[var(--sl-color-text-primary)]', portrait ? 'flex-col justify-center' : 'justify-center')}
      style={{ padding: SAFE * 1.5, gap: portrait ? SAFE : SAFE * 1.5 }}
    >
      <Sparkles points={[{ x: 48, y: 8, size: 1.6, delay: 1.1 }, { x: 96, y: 94, size: 1.2, delay: 2 }]} />
      <div className="relative shrink-0" style={{ width: photo, height: photo }}>
        <div className="size-full overflow-hidden rounded-[48px] bg-[var(--sl-color-bg-surface)] shadow-[var(--sl-shadow-xl)]">
          {p.imageKey && <KenBurns src={imageUrl(p.imageKey, 'sq1024')} focus={p.imageFocus} duration={slide.durationSeconds + 1} />}
        </div>
        {slide.steam && <Steam className="absolute bottom-[calc(100%-40px)] left-1/2 w-[160px] -translate-x-1/2" />}
      </div>
      <div className={cn('flex min-w-0 flex-col', compact ? 'gap-5' : 'gap-7', portrait ? 'w-full items-center text-center' : 'max-w-[880px] flex-1')}>
        {slide.headline.id && (
          <Enter delay={0.2}>
            <Kicker>{slide.headline.id}</Kicker>
          </Enter>
        )}
        <Enter delay={0.35}>
          <h1 className={cn('font-display font-bold tracking-[-0.02em]', compact ? 'text-[76px]' : 'text-[96px]', 'leading-[1.02]')}>
            <Bilingual id={p.nameId} en={p.nameEn} />
          </h1>
        </Enter>
        {p.descriptionId && (
          <Enter delay={0.55}>
            <p className="line-clamp-3 max-w-[760px] text-[32px] leading-snug text-[var(--sl-color-text-secondary)]">{p.descriptionId}</p>
          </Enter>
        )}
        <div className={cn('flex flex-wrap items-center gap-x-8 gap-y-5', portrait && 'justify-center')}>
          <PriceTag product={p} />
          {(p.dietaryTags.length > 0 || p.soldOutVariants.length > 0) && (
            <Enter delay={1.3} className="flex flex-wrap gap-3">
              {p.dietaryTags.map((t) => (
                <span key={t.code} className="rounded-full border border-white/25 px-6 py-2 text-[24px] font-semibold text-[var(--sl-color-text-primary)]">
                  {t.nameId}
                </span>
              ))}
              {p.soldOutVariants.map((v) => (
                <span key={v.id} className="rounded-full bg-white/10 px-6 py-2 text-[24px] font-semibold text-[var(--sl-color-text-secondary)]">
                  {v.id} habis
                </span>
              ))}
            </Enter>
          )}
        </div>
        <OrderHint />
      </div>
    </div>
  );
}

export function HappyHour({ slide }: { slide: PlaylistSlide }) {
  const { width, portrait } = useStage();
  const now = useNow();
  const d = slide.discount!;
  const p = slide.product;
  const active = d.activeNow && now < d.endsAt;
  const target = active ? d.endsAt : d.startsAt;
  const pct = d.kind === 'percent' ? `${d.value / 100}%` : formatRupiah(d.value);
  // 4:3 and 16:10 screens leave the text column narrower, so its type steps down a size.
  const compact = !portrait && width < 1800;
  const photo = portrait ? 640 : compact ? 460 : Math.min(620, (width - SAFE * 4) * 0.42);
  return (
    <div
      className={cn(
        'relative flex size-full overflow-hidden bg-[radial-gradient(ellipse_at_80%_40%,var(--sl-color-espresso-800),var(--sl-color-espresso-950)_70%)] text-[var(--sl-color-text-primary)]',
        portrait ? 'flex-col justify-center' : 'items-center justify-between',
      )}
      style={{ padding: SAFE * 1.5, gap: SAFE }}
    >
      <Sparkles points={[{ x: 44, y: 12, size: 2, delay: 0.5 }, { x: 92, y: 88, size: 1.6, delay: 1.4 }, { x: 4, y: 60, size: 1.2, delay: 2.1 }]} />
      <div className={cn('flex min-w-0 flex-1 flex-col', compact ? 'gap-6' : 'gap-8', portrait && 'items-center text-center')}>
        <Enter delay={0.2}>
          <Kicker>{active ? 'Sedang berlangsung · Now on' : 'Segera · Coming up'}</Kicker>
        </Enter>
        <Enter delay={0.35}>
          <h1 className={cn('font-display font-bold tracking-[-0.02em]', compact ? 'text-[80px]' : 'text-[100px]', 'leading-[1.02]')}>
            <Bilingual id={slide.headline.id || d.name} en={slide.headline.en} />
          </h1>
        </Enter>
        <Enter delay={0.55} className={cn('flex flex-wrap items-baseline gap-x-8 gap-y-2', portrait && 'justify-center')}>
          <span className={cn('font-display font-bold text-[var(--sl-color-brand-salmon)]', compact ? 'text-[120px]' : 'text-[148px]', 'leading-none')}>−{pct}</span>
          {slide.subline.id && <span className="text-[34px] font-semibold text-[var(--sl-color-rice-100)]">{slide.subline.id}</span>}
        </Enter>
        <Enter delay={0.8} className={cn('flex flex-col gap-4', portrait && 'items-center')}>
          <span className="text-[26px] font-semibold uppercase tracking-[0.12em] text-[var(--sl-color-text-secondary)]">
            {active ? 'Berakhir dalam · Ends in' : 'Mulai dalam · Starts in'}
          </span>
          <span className={cn(compact ? 'text-[72px]' : 'text-[88px]', 'leading-none')}>
            <Countdown ms={target - now} />
          </span>
        </Enter>
      </div>
      {p && (
        <div className="flex shrink-0 flex-col items-center gap-8">
          <div className="sl-float aspect-square overflow-hidden rounded-full border-[12px] border-white/10 bg-[var(--sl-color-bg-surface)] shadow-[var(--sl-shadow-xl)]" style={{ width: photo }}>
            {p.imageKey && <img src={imageUrl(p.imageKey, 'sq1024')} alt="" className="size-full object-cover" style={{ objectPosition: `${p.imageFocus.x}% ${p.imageFocus.y}%` }} />}
          </div>
          <Enter delay={0.9} className="text-center">
            <p className="font-display text-[48px] font-semibold leading-tight">{p.nameId}</p>
            <p className="tabular mt-3 flex items-baseline justify-center gap-6">
              <motion.span
                className="text-[44px] font-bold text-[var(--sl-color-text-secondary)]"
                initial={{ textDecorationColor: 'rgba(242,136,91,0)' }}
                animate={{ textDecorationColor: 'rgba(242,136,91,1)' }}
                transition={{ delay: 1.2, duration: 0.4 }}
                style={{ textDecorationLine: 'line-through', textDecorationThickness: 4 }}
              >
                {formatRupiah(p.price)}
              </motion.span>
              <motion.span
                className="text-[76px] font-extrabold text-[var(--sl-color-brand-salmon)]"
                initial={{ opacity: 0, scale: 0.6 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: 1.4, duration: 0.6, ease: overshoot }}
              >
                {formatRupiah(d.discountedPrice)}
              </motion.span>
            </p>
          </Enter>
        </div>
      )}
    </div>
  );
}

export function CategorySpotlight({ slide }: { slide: PlaylistSlide }) {
  const { portrait } = useStage();
  const reduce = useReducedMotion();
  const products = slide.products ?? [];
  // Landscape: one row of up to 4, or 3 × 2 for 5–6. Portrait: 2 columns.
  const cols = portrait ? Math.min(2, products.length) : products.length > 4 ? 3 : Math.max(products.length, 1);
  return (
    <div className="relative size-full overflow-hidden text-[var(--sl-color-text-primary)]">
      {slide.imageKey && <img src={wideImageUrl(slide.imageKey, slide.imageFocus)} alt="" className="absolute inset-0 size-full scale-110 object-cover blur-[6px]" />}
      <div className="absolute inset-0 bg-[var(--sl-color-bg-canvas)]/85" />
      <div className="relative flex size-full flex-col" style={{ padding: `${SAFE * 1.25}px ${SAFE * 1.5}px`, gap: SAFE * 0.75 }}>
        <div className={cn('flex gap-8', portrait ? 'flex-col' : 'items-end justify-between')}>
          <Enter delay={0.2}>
            <h1 className="font-display text-[88px] font-bold leading-none tracking-[-0.02em]">
              <Bilingual id={slide.headline.id} en={slide.headline.en} />
            </h1>
          </Enter>
          {!portrait && <OrderHint delay={1.2} />}
        </div>
        <div className="grid min-h-0 flex-1 gap-8" style={{ gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`, gridAutoRows: 'minmax(0, 1fr)' }}>
          {products.map((p, i) => (
            <motion.div
              key={p.id}
              initial={reduce ? { opacity: 0 } : { opacity: 0, y: 48 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: reduce ? 0.15 : 0.6, delay: reduce ? 0 : 0.5 + i * 0.08, ease }}
              className="min-h-0"
            >
              <div className="sl-float flex h-full flex-col overflow-hidden rounded-[32px] bg-[var(--sl-color-bg-surface)] shadow-[var(--sl-shadow-lg)]" style={{ animationDelay: `${i * 0.4}s` }}>
                <div className="relative min-h-0 flex-1">
                  {p.imageKey && (
                    <img
                      src={imageUrl(p.imageKey, 'sq640')}
                      alt=""
                      className={cn('absolute inset-0 size-full object-cover', p.soldOut && 'opacity-45 grayscale')}
                      style={{ objectPosition: `${p.imageFocus.x}% ${p.imageFocus.y}%` }}
                    />
                  )}
                  {(p.soldOut || p.soldOutVariants.length > 0) && (
                    <span className="absolute bottom-4 right-4 rounded-full bg-[var(--sl-color-bg-surface)] px-4 py-1.5 text-[22px] font-bold text-[var(--sl-color-text-primary)]">
                      {p.soldOut ? 'Habis' : `${p.soldOutVariants.map((v) => v.id).join(', ')} habis`}
                    </span>
                  )}
                </div>
                <div className="flex flex-col gap-2 px-7 py-6">
                  <span className="line-clamp-2 text-[32px] font-bold leading-tight">{p.nameId}</span>
                  <span className="tabular flex items-baseline gap-2 text-[32px] font-bold text-[var(--sl-color-brand-salmon)]">
                    {p.priceFrom && <span className="text-[22px] font-semibold opacity-80">mulai</span>}
                    {formatRupiah(p.price)}
                  </span>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
        {portrait && <OrderHint delay={1.2} />}
      </div>
    </div>
  );
}

export function BrandIdle({ slide }: { slide: PlaylistSlide }) {
  const { width, portrait } = useStage();
  return (
    <div className="relative flex size-full flex-col items-center justify-center gap-16 overflow-hidden bg-[var(--sl-color-bg-canvas)]">
      {slide.imageKey && (
        <img src={wideImageUrl(slide.imageKey, slide.imageFocus)} alt="" className="absolute inset-0 size-full object-cover opacity-30" style={{ objectPosition: `${slide.imageFocus.x}% ${slide.imageFocus.y}%` }} />
      )}
      {/* Opaque card: a translucent one lets the photo's details ghost through the wordmark. */}
      <div className="relative rounded-[64px] bg-[var(--sl-color-rice-50)] shadow-[var(--sl-shadow-xl)]" style={{ padding: portrait ? '64px 56px' : '64px 96px' }}>
        <div style={{ width: Math.min(1100, width - SAFE * 2 - (portrait ? 112 : 192)) }}>
          <Logo variant="lockup" animated className="block h-auto w-full" />
        </div>
      </div>
      <div className="relative">
        <OrderHint delay={0.6} />
      </div>
    </div>
  );
}

export function Uploaded({ slide }: { slide: PlaylistSlide }) {
  const { portrait } = useStage();
  const reduce = useReducedMotion();
  const poster = slide.imageKey && wideImageUrl(slide.imageKey, slide.imageFocus);
  // Landscape media fills the screen; on a portrait screen it is shown whole over a blurred copy.
  const fit = portrait ? 'object-contain' : 'object-cover';
  return (
    <div className="relative size-full overflow-hidden bg-black">
      {portrait && poster && <img src={poster} alt="" className="absolute inset-0 size-full scale-110 object-cover opacity-60 blur-[24px]" />}
      {slide.videoKey ? (
        <video src={videoUrl(slide.videoKey)} poster={poster} autoPlay={!reduce} muted loop playsInline className={cn('relative size-full', fit)} />
      ) : (
        poster && <KenBurns src={poster} duration={slide.durationSeconds + 1} className={cn('relative', fit)} />
      )}
    </div>
  );
}

export const renderers: Record<PlaylistSlide['template'], (props: { slide: PlaylistSlide }) => ReactNode> = {
  hero_promo: HeroPromo,
  featured_product: FeaturedProduct,
  happy_hour: HappyHour,
  category_spotlight: CategorySpotlight,
  brand_idle: BrandIdle,
  uploaded: Uploaded,
};

/** Image URLs a slide shows, so the next slide can be preloaded before its transition. */
export function slideImages(s: PlaylistSlide): string[] {
  const out: string[] = [];
  if (s.imageKey) out.push(wideImageUrl(s.imageKey, s.imageFocus));
  if (s.product?.imageKey) out.push(imageUrl(s.product.imageKey, 'sq1024'));
  for (const p of s.products ?? []) if (p.imageKey) out.push(imageUrl(p.imageKey, 'sq640'));
  return out;
}
