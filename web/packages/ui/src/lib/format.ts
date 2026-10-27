// Formats from docs/design/CONTENT.md §5.

const rupiah = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 0 });

/** "Rp 38.000"; negatives as "−Rp 5.000". */
export function formatRupiah(amount: number): string {
  const abs = rupiah.format(Math.abs(amount));
  return amount < 0 ? `−Rp ${abs}` : `Rp ${abs}`;
}

/** Basis points as a percent: 1500 → "15%". */
export function formatPercentBp(bp: number): string {
  const pct = bp / 100;
  return `${Number.isInteger(pct) ? pct : pct.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')}%`.replace('.', ',');
}

/** WIB clock time "19.45". */
export function formatTime(ms: number): string {
  return new Date(ms).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' });
}

/** "26 Okt 2026, 19.45". */
export function formatDateTime(ms: number): string {
  return new Date(ms).toLocaleString('id-ID', {
    day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta',
  });
}

/** Minutes after midnight → "14.00". */
export function formatMinuteOfDay(minute: number): string {
  const h = Math.floor(minute / 60) % 24;
  return `${String(h).padStart(2, '0')}.${String(minute % 60).padStart(2, '0')}`;
}

export type ImageVariant = 'sq320' | 'sq640' | 'sq1024';

/** URL of a processed image (TR-DM-09). */
export function imageUrl(key: string, variant: ImageVariant = 'sq640'): string {
  return `/media/images/${key}/${variant}.webp`;
}

/** URL of the 16:9 Product Display crop for a focal point. */
export function wideImageUrl(key: string, focus: { x: number; y: number }): string {
  return `/media/images/${key}/wide-${focus.x}-${focus.y}.webp`;
}

/** URL of an uploaded Product Display video; its key already carries the file name. */
export function videoUrl(key: string): string {
  return `/media/images/${key}`;
}

/** srcset for 1:1 images. */
export function imageSrcSet(key: string): string {
  return `${imageUrl(key, 'sq320')} 320w, ${imageUrl(key, 'sq640')} 640w, ${imageUrl(key, 'sq1024')} 1024w`;
}
