import { RotateCcw } from 'lucide-react';
import { useEffect, useState } from 'react';

/**
 * Full-screen notice while the demo resets (DEMO-AND-SEED §9). The app passes `watch` (from the
 * API client) and what "start over" means for it; the notice blocks input until the new demo
 * is live, then the app restarts.
 */
export function useDemoReset(
  watch: (h: { onResetting: () => void; onReset: () => void; onFailed?: () => void }) => () => void,
  restart: () => void,
) {
  const [resetting, setResetting] = useState(false);
  useEffect(
    () =>
      watch({
        onResetting: () => setResetting(true),
        onReset: () => restart(),
        onFailed: () => setResetting(false),
      }),
    // restart is a stable module-level function in every app
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [watch],
  );
  return resetting;
}

export function DemoResetOverlay({ active, label = 'Menyiapkan ulang demo…' }: { active: boolean; label?: string }) {
  if (!active) return null;
  return (
    <div role="alertdialog" aria-modal="true" aria-label={label}
      className="fixed inset-0 z-[100] grid place-items-center bg-[color-mix(in_oklab,var(--sl-color-nori-900)_88%,transparent)] text-rice">
      <div className="flex flex-col items-center gap-4 text-center">
        <RotateCcw className="size-12 animate-spin [animation-direction:reverse] [animation-duration:1.6s]" aria-hidden />
        <p className="font-display text-2xl font-semibold">{label}</p>
      </div>
    </div>
  );
}
