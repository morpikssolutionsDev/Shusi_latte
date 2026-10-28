import { createContext, useContext, useLayoutEffect, useState, type ReactNode } from 'react';

/**
 * The design stage: slides are laid out in stage pixels with the short side fixed at 1080
 * (a 1920×1080 TV is 1:1), then scaled to the screen (LAYOUT.md §3). The long side stretches
 * between 1440 and 2560, so 4:3, 16:10, 16:9 and 21:9 screens fill edge to edge; only screens
 * beyond that are letterboxed. A screen taller than it is wide gets the portrait layouts.
 */
export interface StageSize {
  width: number;
  height: number;
  portrait: boolean;
}

const SHORT = 1080;
const LONG_MIN = 1440;
const LONG_MAX = 2560;

const StageContext = createContext<StageSize>({ width: 1920, height: 1080, portrait: false });

export const useStage = () => useContext(StageContext);

function measure(): StageSize & { scale: number } {
  const w = window.innerWidth;
  const h = window.innerHeight;
  const portrait = h > w;
  const scale = portrait ? Math.min(w / SHORT, h / LONG_MIN) : Math.min(h / SHORT, w / LONG_MIN);
  return {
    scale,
    portrait,
    width: Math.round(Math.min(w / scale, portrait ? LONG_MIN : LONG_MAX)),
    height: Math.round(Math.min(h / scale, portrait ? LONG_MAX : LONG_MIN)),
  };
}

export function Stage({ children }: { children: ReactNode }) {
  const [size, setSize] = useState(measure);
  useLayoutEffect(() => {
    const fit = () => setSize(measure());
    window.addEventListener('resize', fit);
    return () => window.removeEventListener('resize', fit);
  }, []);
  const { scale, ...stage } = size;
  return (
    <div className="fixed inset-0 overflow-hidden bg-[var(--sl-color-bg-canvas)]">
      <div
        className="absolute left-1/2 top-1/2 overflow-hidden"
        style={{ width: stage.width, height: stage.height, transform: `translate(-50%, -50%) scale(${scale})` }}
        data-orientation={stage.portrait ? 'portrait' : 'landscape'}
      >
        <StageContext.Provider value={stage}>{children}</StageContext.Provider>
      </div>
    </div>
  );
}
