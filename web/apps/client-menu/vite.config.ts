import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { apiProxy } from '../vite.shared.ts';

// Client Menu: customers on phones, served at the site root (TRD §1).
export default defineConfig({
  base: '/',
  plugins: [react(), tailwindcss()],
  server: { port: 5173, strictPort: true, proxy: apiProxy, host: true },
});
