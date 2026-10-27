// M6 "Add to order": a thumbnail flies along a curve into the cart bar, then the bar's count
// pops. Pure WAAPI on a detached element, so it survives the sheet closing underneath it.
const DURATION = 600;

export function fly(src: string, from: DOMRect) {
  const target = document.querySelector('[data-cart-target]')?.getBoundingClientRect();
  const size = 72;
  const img = document.createElement('img');
  img.src = src;
  img.alt = '';
  Object.assign(img.style, {
    position: 'fixed',
    left: '0px',
    top: '0px',
    width: `${size}px`,
    height: `${size}px`,
    borderRadius: '9999px',
    objectFit: 'cover',
    zIndex: '70',
    pointerEvents: 'none',
    boxShadow: '0 8px 24px rgb(0 0 0 / 0.25)',
  });
  document.body.appendChild(img);
  const x0 = from.left + from.width / 2 - size / 2;
  const y0 = from.top + from.height / 2 - size / 2;
  const x1 = target ? target.left + 28 - size / 2 : window.innerWidth / 2 - size / 2;
  const y1 = target ? target.top + target.height / 2 - size / 2 : window.innerHeight - 80;
  // Quadratic curve through a point above the straight line.
  const cx = (x0 + x1) / 2;
  const cy = Math.min(y0, y1) - 120;
  const at = (t: number) => {
    const x = (1 - t) ** 2 * x0 + 2 * (1 - t) * t * cx + t ** 2 * x1;
    const y = (1 - t) ** 2 * y0 + 2 * (1 - t) * t * cy + t ** 2 * y1;
    const s = 1 - 0.65 * t;
    return { transform: `translate(${x}px, ${y}px) scale(${s})`, opacity: t > 0.9 ? 1 - (t - 0.9) * 10 : 1 };
  };
  const frames = Array.from({ length: 13 }, (_, i) => at(i / 12));
  img.animate(frames, { duration: DURATION, easing: 'cubic-bezier(0.22, 1, 0.36, 1)', fill: 'forwards' }).finished.finally(() => img.remove());
}
