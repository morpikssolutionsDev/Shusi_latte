import { motion, useReducedMotion } from 'motion/react';
import { useEffect, useState } from 'react';

/** Rising steam curls (same keyframes as the logo). */
export function Steam({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 120 120" className={`sl-steam ${className ?? ''}`} aria-hidden fill="none" stroke="#F7F0E4" strokeWidth="7" strokeLinecap="round" opacity="0.9">
      <path d="M30 110C18 90 42 76 30 56C20 40 36 28 30 14" />
      <path d="M60 104C48 84 72 70 60 50C50 34 66 22 60 8" />
      <path d="M90 110C78 90 102 76 90 56C80 40 96 28 90 14" />
    </svg>
  );
}

const SPARK = 'M0 -12C2 -3 3 -2 12 0C3 2 2 3 0 12C-2 3 -3 2 -12 0C-3 -2 -2 -3 0 -12Z';

/** Salmon sparkles twinkling at the given positions (% of the parent). */
export function Sparkles({ points }: { points: Array<{ x: number; y: number; size: number; delay: number }> }) {
  return (
    <svg className="pointer-events-none absolute inset-0 size-full" aria-hidden>
      {points.map((p, i) => (
        <g key={i} transform={`translate(${p.x * 19.2} ${p.y * 10.8}) scale(${p.size})`}>
          <path d={SPARK} fill="#F2885B" className="sl-sparkle" style={{ animationDelay: `${p.delay}s` }} />
        </g>
      ))}
    </svg>
  );
}

/** Ticks every second; used by the Happy Hour countdown. */
export function useNow(intervalMs = 1000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}

/** hh:mm:ss with each digit flipping as it changes (tabular, MOTION.md §6). */
export function Countdown({ ms }: { ms: number }) {
  const reduce = useReducedMotion();
  const total = Math.max(0, Math.floor(ms / 1000));
  const parts = [Math.floor(total / 3600), Math.floor((total % 3600) / 60), total % 60].map((n) => String(n).padStart(2, '0'));
  return (
    <span className="tabular inline-flex items-center gap-[0.12em] font-extrabold" aria-label={parts.join(':')}>
      {parts.map((part, i) => (
        <span key={i} className="inline-flex items-center gap-[0.12em]">
          {i > 0 && <span className="-mt-[0.08em] opacity-50">:</span>}
          <span className="inline-flex gap-[0.06em]">
            {part.split('').map((digit, j) => (
              <span key={j} className="relative inline-block w-[0.78em] overflow-hidden rounded-[0.14em] bg-white/10 py-[0.06em] text-center leading-none">
                <motion.span
                  key={digit}
                  className="inline-block"
                  initial={reduce ? false : { y: '-100%', opacity: 0 }}
                  animate={{ y: 0, opacity: 1 }}
                  transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
                >
                  {digit}
                </motion.span>
              </span>
            ))}
          </span>
        </span>
      ))}
    </span>
  );
}
