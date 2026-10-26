import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({
  plugins: [react()],
  base: '/staff/',
  server: { port: 5174, proxy: { '/api': 'http://localhost:8080', '/public': 'http://localhost:8080', '/auth': 'http://localhost:8080' } },
})
