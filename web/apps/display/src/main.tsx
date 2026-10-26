import '@sushi-latte/ui/styles.css';

import { Logo } from '@sushi-latte/ui';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

// Phase 0 shell: the Brand Idle slide. The Product Display is built in Phase 2 (todolist.md).
function App() {
  return (
    <main className="grid min-h-dvh place-items-center bg-background p-16">
      <Logo variant="lockup" animated className="w-[min(70vw,1100px)]" />
    </main>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
