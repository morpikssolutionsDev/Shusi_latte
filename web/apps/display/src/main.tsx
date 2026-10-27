import '@sushi-latte/ui/styles.css';
import './display.css';

import { api, connectRealtime, unwrap } from '@sushi-latte/api-client';
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { mountApp } from '@sushi-latte/ui';
import { StrictMode, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { BrandIdle, renderers, slideImages, type PlaylistSlide } from './slides';

const STAGE_W = 1920;
const STAGE_H = 1080;
const fallback: PlaylistSlide = {
  id: 'fallback', template: 'brand_idle', durationSeconds: 8, headline: { id: '', en: '' }, subline: { id: '', en: '' },
  imageFocus: { x: 50, y: 50 }, steam: false,
};

const playlistQuery = {
  queryKey: ['playlist'],
  queryFn: async () => unwrap(await api.GET('/public/display/playlist')),
  refetchInterval: 60_000, // also picks up time-window changes (promo start/end) without events
};

/** Renders the 1920×1080 design stage scaled to any screen, letterboxed (LAYOUT.md §3). */
function Stage({ children }: { children: React.ReactNode }) {
  const [scale, setScale] = useState(1);
  useLayoutEffect(() => {
    const fit = () => setScale(Math.min(window.innerWidth / STAGE_W, window.innerHeight / STAGE_H));
    fit();
    window.addEventListener('resize', fit);
    return () => window.removeEventListener('resize', fit);
  }, []);
  return (
    <div className="grid h-dvh w-dvw place-items-center overflow-hidden bg-[#12221B]">
      <div style={{ width: STAGE_W, height: STAGE_H, transform: `scale(${scale})`, transformOrigin: 'center' }} className="relative shrink-0 overflow-hidden">
        {children}
      </div>
    </div>
  );
}

/**
 * Plays the playlist: each slide dwells for its duration, then a 1 s cross-fade + 40 px slide.
 * The playlist is re-read only at a transition, so a change never interrupts a slide
 * mid-way (MOTION.md §6, BR-PD-03).
 */
function Player() {
  const queryClient = useQueryClient();
  const reduce = useReducedMotion();
  const { data } = useQuery(playlistQuery);
  const latest = useRef<PlaylistSlide[]>([]);
  latest.current = data?.slides.length ? data.slides : [fallback];
  const [current, setCurrent] = useState<{ slide: PlaylistSlide; index: number; n: number } | null>(null);

  useEffect(() => {
    const conn = connectRealtime({
      topics: ['display', 'menu'],
      onEvent: () => void queryClient.invalidateQueries({ queryKey: ['playlist'] }),
      onReconnect: () => void queryClient.invalidateQueries({ queryKey: ['playlist'] }),
    });
    return () => conn.close();
  }, [queryClient]);

  // Start once the first playlist arrives.
  useEffect(() => {
    if (!current && data) setCurrent({ slide: latest.current[0]!, index: 0, n: 0 });
  }, [data, current]);

  // Advance after the dwell; preload the next slide's images 1 s before.
  useEffect(() => {
    if (!current) return;
    const dwell = current.slide.durationSeconds * 1000;
    const preload = setTimeout(() => {
      const list = latest.current;
      const next = list[(current.index + 1) % list.length];
      if (next) for (const src of slideImages(next)) new Image().src = src;
    }, Math.max(0, dwell - 1000));
    const advance = setTimeout(() => {
      const list = latest.current;
      // Keep position by id when the list changed; otherwise move on.
      const at = list.findIndex((s) => s.id === current.slide.id);
      const index = ((at >= 0 ? at : current.index) + 1) % list.length;
      setCurrent({ slide: list[index]!, index, n: current.n + 1 });
    }, dwell);
    return () => {
      clearTimeout(preload);
      clearTimeout(advance);
    };
  }, [current]);

  const Slide = current ? renderers[current.slide.template] : BrandIdle;
  return (
    <Stage>
      <AnimatePresence initial={false}>
        {current && (
          <motion.div
            key={`${current.n}-${current.slide.id}`}
            className="absolute inset-0"
            data-template={current.slide.template}
            data-slide-id={current.slide.id}
            initial={reduce ? { opacity: 0 } : { opacity: 0, x: 40 }}
            animate={{ opacity: 1, x: 0 }}
            exit={reduce ? { opacity: 0 } : { opacity: 0, x: -40 }}
            transition={{ duration: 1, ease: [0.2, 0, 0, 1] }}
          >
            <Slide slide={current.slide} />
          </motion.div>
        )}
      </AnimatePresence>
    </Stage>
  );
}

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: true, refetchOnWindowFocus: false } } });

mountApp(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <Player />
    </QueryClientProvider>
  </StrictMode>,
);
