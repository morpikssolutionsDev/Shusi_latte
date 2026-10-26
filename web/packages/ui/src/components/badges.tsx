import type { ReactNode } from 'react';
import { CircleSlash } from 'lucide-react';
import { cn } from '../lib/cn';

export type Tone = 'pending' | 'confirmed' | 'preparing' | 'ready' | 'served' | 'cancelled' | 'voided';
export type Feedback = 'success' | 'warning' | 'danger' | 'info';

/** Pill in a status or feedback tone (TOKENS.md §4). Always pair with a text label. */
export function Chip({
  tone,
  feedback,
  icon,
  children,
  className,
}: {
  tone?: Tone;
  feedback?: Feedback;
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const key = tone ? `status-${tone}` : feedback ? `feedback-${feedback}` : 'status-served';
  return (
    <span
      className={cn('inline-flex h-7 items-center gap-1.5 whitespace-nowrap rounded-chip border px-3 text-xs font-semibold [&_svg]:size-3.5', className)}
      style={{
        background: `var(--sl-color-${key}-bg)`,
        color: `var(--sl-color-${key}-fg)`,
        borderColor: `var(--sl-color-${key}-border)`,
      }}
    >
      {icon}
      <span className={cn(tone === 'cancelled' && 'line-through')}>{children}</span>
    </span>
  );
}

/** Plum Dietary badge: Plum is reserved for Dietary Warnings (COLOR.md §6). */
export function DietaryBadge({ children, icon, className }: { children: ReactNode; icon?: ReactNode; className?: string }) {
  return (
    <span
      className={cn('inline-flex h-7 items-center gap-1.5 rounded-chip border px-3 text-xs font-semibold [&_svg]:size-3.5', className)}
      style={{
        background: 'var(--sl-color-dietary-warning-bg)',
        color: 'var(--sl-color-dietary-warning-fg)',
        borderColor: 'var(--sl-color-dietary-warning-border)',
      }}
    >
      {icon}
      {children}
    </span>
  );
}

/** Neutral tag chip for "Contains: …" lists (not a warning). */
export function TagChip({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cn('inline-flex h-6 items-center rounded-chip bg-sunken px-2.5 text-xs font-medium text-text-secondary', className)}>
      {children}
    </span>
  );
}

/** "Habis · Sold out" label (COLOR.md §4). */
export function SoldOutBadge({ label = 'Habis', className }: { label?: string; className?: string }) {
  return (
    <span
      className={cn('inline-flex h-6 items-center gap-1 rounded-chip px-2.5 text-xs font-bold [&_svg]:size-3.5', className)}
      style={{ background: 'var(--sl-color-sold-out-badge-bg)', color: 'var(--sl-color-sold-out-badge-fg)' }}
    >
      <CircleSlash aria-hidden />
      {label}
    </span>
  );
}
