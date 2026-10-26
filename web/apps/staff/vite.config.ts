import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';
import { apiProxy } from '../vite.shared.ts';

// Staff PWA: Admin Panel, Waiter, Cashier, Kitchen, HRIS behind role-gated routes (TRD §3).
export default defineConfig({
  base: '/staff/',
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: 'Sushi Latte Staff',
        short_name: 'SL Staff',
        lang: 'id',
        start_url: '/staff/',
        scope: '/staff/',
        display: 'standalone',
        background_color: '#F5F7F6',
        theme_color: '#1E2B25',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: { navigateFallbackDenylist: [/^\/api\//, /^\/ws/] },
    }),
  ],
  server: { port: 5174, strictPort: true, proxy: apiProxy },
});
