// The latte cup used for M8 (order sent), M9 (cup-fill wait progress) and M10 (Ready: a nigiri
// drops onto the cup). Inline SVG + Motion, transform/opacity only (MOTION.md §3).
import { motion, useReducedMotion } from 'motion/react';
import { spring } from './kit';

const CUP_TOP = 78;
const CUP_BOTTOM = 160;
const CUP_H = CUP_BOTTOM - CUP_TOP;

export function CupFill({ level, fillFrom, ready = false, celebrate = false, className }: {
  level: number; // 0..1
  fillFrom?: number;
  ready?: boolean;
  celebrate?: boolean;
  className?: string;
}) {
  const reduce = useReducedMotion();
  const clamped = Math.max(0, Math.min(1, level));
  const y = CUP_BOTTOM - clamped * CUP_H;
  const fromY = CUP_BOTTOM - Math.max(0, Math.min(1, fillFrom ?? clamped)) * CUP_H;
  const steam = reduce ? 0 : 0.25 + clamped * 0.75; // steam grows as Ready nears (M9)
  return (
    <svg viewBox="0 0 200 200" className={className} aria-hidden>
      <defs>
        <clipPath id="cup-inside">
          <path d="M44 78 H156 L146 150 Q144 162 132 162 H68 Q56 162 54 150 Z" />
        </clipPath>
        <linearGradient id="latte" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#F3E4CF" />
          <stop offset="0.18" stopColor="#D9B48C" />
          <stop offset="1" stopColor="#8A5A3B" />
        </linearGradient>
      </defs>

      {/* steam */}
      <g stroke="#C9B8A6" strokeWidth="5" strokeLinecap="round" fill="none" opacity={steam} className={reduce ? undefined : 'cm-steam'}>
        <path d="M80 66C72 54 88 46 80 34C74 24 84 16 80 8" />
        <path d="M100 66C92 54 108 46 100 34C94 24 104 16 100 8" />
        <path d="M120 66C112 54 128 46 120 34C114 24 124 16 120 8" />
      </g>

      {/* saucer */}
      <ellipse cx="100" cy="172" rx="78" ry="10" fill="#1F2A24" />
      {/* handle */}
      <path d="M150 96 C178 92 182 132 146 136" stroke="#1F2A24" strokeWidth="10" fill="none" strokeLinecap="round" />
      {/* cup interior + liquid */}
      <path d="M44 78 H156 L146 150 Q144 162 132 162 H68 Q56 162 54 150 Z" fill="#F7F0E4" />
      <g clipPath="url(#cup-inside)">
        <motion.rect
          x="40"
          width="120"
          height={CUP_H + 20}
          fill="url(#latte)"
          initial={{ y: fromY }}
          animate={{ y }}
          transition={reduce ? { duration: 0 } : { duration: 1.2, ease: [0.22, 1, 0.36, 1] }}
        />
      </g>
      {/* cup outline and salmon band */}
      <path d="M44 78 H156 L146 150 Q144 162 132 162 H68 Q56 162 54 150 Z" fill="none" stroke="#1F2A24" strokeWidth="6" strokeLinejoin="round" />
      <path d="M50 118 H150" stroke="#F2885B" strokeWidth="7" />

      {/* M10: a nigiri drops onto the cup when Ready */}
      {ready && (
        <motion.g initial={reduce ? { opacity: 0 } : { y: -90, opacity: 0 }} animate={{ y: 0, opacity: 1 }} transition={reduce ? { duration: 0.15 } : spring.bouncy}>
          <rect x="66" y="56" width="68" height="20" rx="10" fill="#FAF6F1" stroke="#1F2A24" strokeWidth="4" />
          <path d="M62 58 Q100 34 138 58 Q100 66 62 58 Z" fill="#F2885B" stroke="#1F2A24" strokeWidth="4" />
          <path d="M84 50 L90 60 M100 46 L104 58 M116 50 L118 60" stroke="#FAF6F1" strokeWidth="3" strokeLinecap="round" />
        </motion.g>
      )}

      {/* M8: sparkles burst */}
      {celebrate && !reduce &&
        [
          [36, 52, 0],
          [168, 48, 0.08],
          [24, 120, 0.16],
          [178, 112, 0.1],
          [100, 22, 0.2],
        ].map(([cx, cy, delay]) => (
          <motion.path
            key={`${cx}-${cy}`}
            d={`M${cx} ${cy! - 9} L${cx! + 3} ${cy! - 3} L${cx! + 9} ${cy} L${cx! + 3} ${cy! + 3} L${cx} ${cy! + 9} L${cx! - 3} ${cy! + 3} L${cx! - 9} ${cy} L${cx! - 3} ${cy! - 3} Z`}
            fill="#F2885B"
            initial={{ scale: 0, opacity: 0 }}
            animate={{ scale: [0, 1.3, 1], opacity: [0, 1, 0] }}
            transition={{ duration: 1.2, delay: 0.6 + delay!, ease: 'easeOut' }}
            style={{ transformOrigin: `${cx}px ${cy}px` }}
          />
        ))}
    </svg>
  );
}
