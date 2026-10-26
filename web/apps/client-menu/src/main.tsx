import '@sushi-latte/ui/styles.css';

import { Logo } from '@sushi-latte/ui';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

// Phase 0 shell. The Client Menu is built in Phase 3 (todolist.md).
function App() {
  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col items-center justify-center gap-6 px-4 text-center">
      <Logo variant="lockup" animated className="w-full max-w-sm" />
      <p className="text-text-secondary">Sushi &amp; kopi, satu meja.</p>
      <p className="rounded-chip bg-sunken px-3 py-1 text-sm font-semibold text-muted-foreground">Menu segera hadir</p>
    </main>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
