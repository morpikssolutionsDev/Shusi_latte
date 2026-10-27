import type { Schemas } from '@sushi-latte/api-client';
import { Logo, cn, formatRupiah, imageUrl, videoUrl, wideImageUrl } from '@sushi-latte/ui';
import { motion, useReducedMotion } from 'motion/react';
import type { ReactNode } from 'react';
import { Countdown, Sparkles, Steam, useNow } from './decor';

export type PlaylistSlide = Schemas['PlaylistSlide'];
type DisplayProduct = Schemas['DisplayProduct'];

const ease = [0.22, 1, 0.36, 1] as const;
const overshoot = [0.34, 1.56, 0.64, 1] as const;

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

/** Slow Ken Burns zoom across the slide's dwell time. */
function KenBurns({ src, duration, className, from = 1, to = 1.08 }: { src: string; duration: number; className?: string; from?: number; to?: number }) {
  const reduce = useReducedMotion();
  return (
    <motion.img
      src={src}
      alt=""
      className={cn('size-full object-cover', className)}
      initial={{ scale: reduce ? 1 : from }}
      animate={{ scale: reduce ? 1 : to }}
      transition={{ duration, ease: 'linear' }}
    />
  );
}

function Bilingual({ id, en, className, enClassName }: { id: string; en: string; className?: string; enClassName?: string }) {
  return (
    <>
      <span className={cn('block', className)}>{id}</span>
      {en && en !== id && <span className={cn('mt-2 block text-[0.5em] font-semibold opacity-70', enClassName)} lang="en">{en}</span>}
    </>
  );
}

function PriceTag({ product, delay = 0.9 }: { product: DisplayProduct; delay?: number }) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      className="inline-flex items-baseline gap-4 rounded-full bg-[var(--sl-color-salmon-400)] px-10 py-5 text-[var(--sl-color-nori-950)] shadow-[var(--sl-shadow-glow)]"
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
  return (
    <div className="relative size-full overflow-hidden">
      {slide.imageKey && <KenBurns src={wideImageUrl(slide.imageKey, slide.imageFocus)} duration={slide.durationSeconds + 1} from={1.08} to={1} />}
      <div className="absolute inset-0 bg-gradient-to-r from-[#12221B]/90 via-[#12221B]/55 to-transparent" />
      <Sparkles points={[{ x: 48, y: 22, size: 2.2, delay: 0.8 }, { x: 56, y: 70, size: 1.4, delay: 1.6 }, { x: 8, y: 86, size: 1.6, delay: 2.2 }]} />
      <div className="absolute inset-y-0 left-16 flex max-w-[900px] flex-col justify-center gap-8 text-[var(--sl-color-rice-50)]">
        <Enter delay={0.2} x={-48} y={0}>
          <p className="text-[24px] font-bold uppercase tracking-[0.12em] text-[var(--sl-color-salmon-400)]">Sushi Latte</p>
        </Enter>
        <Enter delay={0.35} x={-48} y={0}>
          <h1 className="font-display text-[104px] font-bold leading-[1.02] tracking-[-0.02em]">
            <Bilingual id={slide.headline.id} en={slide.headline.en} />
          </h1>
        </Enter>
        {slide.subline.id && (
          <Enter delay={0.8}>
            <p className="text-[36px] font-semibold text-[var(--sl-color-rice-100)]">
              <Bilingual id={slide.subline.id} en={slide.subline.en} />
            </p>
          </Enter>
        )}
      </div>
    </div>
  );
}

export function FeaturedProduct({ slide }: { slide: PlaylistSlide }) {
  const p = slide.product!;
  return (
    <div className="grid size-full grid-cols-[1fr_1fr] items-center gap-16 bg-[var(--sl-color-nori-950)] p-16 text-[var(--sl-color-rice-50)]">
      <div className="relative aspect-square overflow-hidden rounded-[48px] shadow-[var(--sl-shadow-xl)]">
        {p.imageKey && <KenBurns src={imageUrl(p.imageKey, 'sq1024')} duration={slide.durationSeconds + 1} />}
        {slide.steam && <Steam className="absolute left-1/2 top-[6%] w-[30%] -translate-x-1/2" />}
      </div>
      <div className="flex flex-col gap-7">
        {slide.headline.id && (
          <Enter delay={0.2}>
            <p className="text-[28px] font-bold uppercase tracking-[0.12em] text-[var(--sl-color-salmon-400)]">{slide.headline.id}</p>
          </Enter>
        )}
        <Enter delay={0.35}>
          <h1 className="font-display text-[96px] font-bold leading-[1.02] tracking-[-0.02em]">
            <Bilingual id={p.nameId} en={p.nameEn} />
          </h1>
        </Enter>
        <Enter delay={0.55}>
          <p className="max-w-[760px] text-[32px] leading-snug text-[var(--sl-color-neutral-300)]">{p.descriptionId}</p>
        </Enter>
        <div>
          <PriceTag product={p} />
        </div>
        <Enter delay={1.3} className="flex flex-wrap gap-3">
          {p.dietaryTags.map((t) => (
            <span key={t.code} className="rounded-full border border-white/25 px-5 py-2 text-[22px] font-semibold text-[var(--sl-color-rice-100)]">
              {t.nameId}
            </span>
          ))}
          {p.soldOutVariants.map((v) => (
            <span key={v.id} className="rounded-full bg-white/10 px-5 py-2 text-[22px] font-semibold text-[var(--sl-color-neutral-300)]">
              {v.id} habis
            </span>
          ))}
        </Enter>
      </div>
    </div>
  );
}

export function HappyHour({ slide }: { slide: PlaylistSlide }) {
  const now = useNow();
  const d = slide.discount!;
  const p = slide.product;
  const active = d.activeNow && now < d.endsAt;
  const target = active ? d.endsAt : d.startsAt;
  const pct = d.kind === 'percent' ? `${d.value / 100}%` : formatRupiah(d.value);
  return (
    <div className="relative grid size-full grid-cols-[1.1fr_0.9fr] items-center gap-16 overflow-hidden bg-[var(--sl-color-espresso-900)] p-16 text-[var(--sl-color-rice-50)]">
      <Sparkles points={[{ x: 44, y: 12, size: 2, delay: 0.5 }, { x: 92, y: 88, size: 1.6, delay: 1.4 }, { x: 4, y: 60, size: 1.2, delay: 2.1 }]} />
      <div className="flex flex-col gap-8">
        <Enter delay={0.2}>
          <p className="text-[28px] font-bold uppercase tracking-[0.12em] text-[var(--sl-color-salmon-400)]">{active ? 'Sedang berlangsung · Now on' : 'Segera · Coming up'}</p>
        </Enter>
        <Enter delay={0.35}>
          <h1 className="font-display text-[100px] font-bold leading-[1.02] tracking-[-0.02em]">
            <Bilingual id={slide.headline.id || d.name} en={slide.headline.en} />
          </h1>
        </Enter>
        <Enter delay={0.55} className="flex items-baseline gap-6">
          <span className="font-display text-[140px] font-bold leading-none text-[var(--sl-color-salmon-400)]">−{pct}</span>
          {slide.subline.id && <span className="text-[32px] font-semibold text-[var(--sl-color-rice-100)]">{slide.subline.id}</span>}
        </Enter>
        <Enter delay={0.8} className="flex flex-col gap-3">
          <span className="text-[26px] font-semibold uppercase tracking-[0.12em] text-[var(--sl-color-neutral-300)]">
            {active ? 'Berakhir dalam · Ends in' : 'Mulai dalam · Starts in'}
          </span>
          <span className="text-[96px] leading-none">
            <Countdown ms={target - now} />
          </span>
        </Enter>
      </div>
      {p && (
        <div className="flex flex-col items-center gap-8">
          <div className="sl-float aspect-square w-[620px] overflow-hidden rounded-full border-[12px] border-white/10 shadow-[var(--sl-shadow-xl)]">
            {p.imageKey && <img src={imageUrl(p.imageKey, 'sq1024')} alt="" className="size-full object-cover" />}
          </div>
          <Enter delay={0.9} className="text-center">
            <p className="font-display text-[48px] font-semibold">{p.nameId}</p>
            <p className="tabular mt-2 flex items-baseline justify-center gap-6">
              <motion.span
                className="text-[44px] font-bold text-[var(--sl-color-neutral-300)]"
                initial={{ textDecorationColor: 'rgba(242,136,91,0)' }}
                animate={{ textDecorationColor: 'rgba(242,136,91,1)' }}
                transition={{ delay: 1.2, duration: 0.4 }}
                style={{ textDecorationLine: 'line-through', textDecorationThickness: 4 }}
              >
                {formatRupiah(p.price)}
              </motion.span>
              <motion.span
                className="text-[72px] font-extrabold text-[var(--sl-color-salmon-400)]"
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
  const reduce = useReducedMotion();
  const products = slide.products ?? [];
  return (
    <div className="relative size-full overflow-hidden text-[var(--sl-color-rice-50)]">
      {slide.imageKey && <img src={wideImageUrl(slide.imageKey, slide.imageFocus)} alt="" className="absolute inset-0 size-full scale-110 object-cover blur-[6px]" />}
      <div className="absolute inset-0 bg-[#12221B]/80" />
      <div className="relative flex size-full flex-col gap-10 p-16">
        <Enter delay={0.2}>
          <h1 className="font-display text-[88px] font-bold leading-none tracking-[-0.02em]">
            <Bilingual id={slide.headline.id} en={slide.headline.en} enClassName="text-[0.45em]" />
          </h1>
        </Enter>
        <div className={cn('grid flex-1 gap-8', products.length > 4 ? 'grid-cols-3' : 'grid-cols-4')}>
          {products.map((p, i) => (
            <motion.div
              key={p.id}
              initial={reduce ? { opacity: 0 } : { opacity: 0, y: 48 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: reduce ? 0.15 : 0.6, delay: reduce ? 0 : 0.5 + i * 0.08, ease }}
              className="min-h-0"
            >
              <div className="sl-float flex h-full flex-col overflow-hidden rounded-[32px] bg-[var(--sl-color-nori-900)] shadow-[var(--sl-shadow-lg)]" style={{ animationDelay: `${i * 0.4}s` }}>
                <div className="relative min-h-0 flex-1">
                  {p.imageKey && <img src={imageUrl(p.imageKey, 'sq640')} alt="" className={cn('absolute inset-0 size-full object-cover', p.soldOut && 'opacity-45 grayscale')} />}
                  {(p.soldOut || p.soldOutVariants.length > 0) && (
                    <span className="absolute bottom-4 right-4 rounded-full bg-[var(--sl-color-nori-900)] px-4 py-1.5 text-[20px] font-bold text-[var(--sl-color-rice-50)]">
                      {p.soldOut ? 'Habis' : `${p.soldOutVariants.map((v) => v.id).join(', ')} habis`}
                    </span>
                  )}
                </div>
                <div className="flex flex-col gap-1 px-6 py-5">
                  <span className="line-clamp-2 min-h-[2.5em] text-[30px] font-bold leading-tight">{p.nameId}</span>
                  <span className="tabular text-[30px] font-bold text-[var(--sl-color-salmon-400)]">
                    {p.priceFrom && <span className="mr-2 text-[22px] font-semibold opacity-80">mulai</span>}
                    {formatRupiah(p.price)}
                  </span>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function BrandIdle({ slide }: { slide: PlaylistSlide }) {
  return (
    <div className="relative grid size-full place-items-center overflow-hidden bg-[var(--sl-color-nori-950)]">
      {slide.imageKey && <img src={wideImageUrl(slide.imageKey, slide.imageFocus)} alt="" className="absolute inset-0 size-full object-cover opacity-30" />}
      <div className="relative rounded-[64px] bg-[var(--sl-color-rice-50)]/95 px-24 py-16 shadow-[var(--sl-shadow-xl)]">
        <Logo variant="lockup" animated className="w-[1100px]" />
      </div>
    </div>
  );
}

export function Uploaded({ slide }: { slide: PlaylistSlide }) {
  const reduce = useReducedMotion();
  return (
    <div className="size-full overflow-hidden bg-black">
      {slide.videoKey ? (
        <video src={videoUrl(slide.videoKey)} poster={slide.imageKey && wideImageUrl(slide.imageKey, slide.imageFocus)}
          autoPlay={!reduce} muted loop playsInline className="size-full object-cover" />
      ) : slide.imageKey && <KenBurns src={wideImageUrl(slide.imageKey, slide.imageFocus)} duration={slide.durationSeconds + 1} />}
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
