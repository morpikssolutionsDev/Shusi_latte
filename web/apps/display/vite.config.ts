import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { apiProxy } from '../vite.shared.ts';

// Product Display: in-outlet TV (TRD §1).
export default defineConfig({
  base: '/display/',
  plugins: [react(), tailwindcss()],
  server: { port: 5175, strictPort: true, proxy: apiProxy },
});
