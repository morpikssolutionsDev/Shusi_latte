// Dev-server proxy shared by all apps: API and WebSocket go to the Go server.
const target = process.env.SL_API_URL ?? 'http://127.0.0.1:8080';

export const apiProxy = {
  '/api': { target, changeOrigin: false },
  '/healthz': { target, changeOrigin: false },
  '/media': { target, changeOrigin: false },
  '/ws': { target, ws: true, changeOrigin: false },
};
