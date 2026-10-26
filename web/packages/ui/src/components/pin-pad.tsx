import { Delete } from 'lucide-react';
import { useEffect } from 'react';
import { cn } from '../lib/cn';

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', 'back'] as const;

export interface PinPadProps {
  value: string;
  onChange: (value: string) => void;
  /** Called when the sixth digit is entered. */
  onComplete?: (value: string) => void;
  length?: number;
  /** Shake the dots once (wrong PIN). Change the key to replay. */
  error?: boolean;
  disabled?: boolean;
  label?: string;
}

/**
 * COMPONENTS.md §3 PinPad: 6 dots, 3×4 keypad with 72 px keys, fixed layout (muscle memory),
 * hardware number keys accepted on PC. The dot shake is the only shake in the product.
 */
export function PinPad({ value, onChange, onComplete, length = 6, error, disabled, label = 'PIN' }: PinPadProps) {
  const press = (key: string) => {
    if (disabled) return;
    if (key === 'back') return onChange(value.slice(0, -1));
    if (value.length >= length) return;
    const next = value + key;
    onChange(next);
    if (next.length === length) onComplete?.(next);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement) return;
      if (/^[0-9]$/.test(e.key)) press(e.key);
      else if (e.key === 'Backspace') press('back');
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  return (
    <div className="flex flex-col items-center gap-5">
      <div
        role="img"
        aria-label={`${label}: ${value.length} dari ${length} digit`}
        className={cn('flex gap-3', error && 'animate-[sl-shake_300ms_ease-in-out]')}
      >
        {Array.from({ length }, (_, i) => (
          <span
            key={i}
            className={cn('size-3.5 rounded-full border-2 border-foreground', i < value.length && 'bg-foreground')}
          />
        ))}
      </div>
      <div className="grid grid-cols-3 gap-3">
        {KEYS.map((key, i) =>
          key === '' ? (
            <span key={i} />
          ) : (
            <button
              key={i}
              type="button"
              disabled={disabled}
              onClick={() => press(key)}
              aria-label={key === 'back' ? 'Hapus' : key}
              className="grid size-18 place-items-center rounded-[var(--sl-pin-pad-radius)] border border-border bg-card text-2xl font-bold text-foreground transition-transform duration-75 active:scale-95 disabled:opacity-40"
            >
              {key === 'back' ? <Delete aria-hidden className="size-6" /> : key}
            </button>
          ),
        )}
      </div>
      <style>{`@keyframes sl-shake{0%,100%{transform:translateX(0)}25%{transform:translateX(-4px)}75%{transform:translateX(4px)}}`}</style>
    </div>
  );
}
