import type { ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';

type Host = HTMLElement & { __slRoot?: Root };

/**
 * Renders an app into #root, reusing the React root if the entry module runs again (a Vite hot
 * update without an accept boundary). A second createRoot on the same node would mount a rival
 * tree — on the TV that restarted the current slide mid-way.
 */
export function mountApp(app: ReactNode) {
  const host = document.getElementById('root') as Host;
  host.__slRoot ??= createRoot(host);
  host.__slRoot.render(app);
}
