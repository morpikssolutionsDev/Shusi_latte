import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({
  plugins: [react()],
  base: '/display/',
  server: { port: 5175, proxy: { '/api': 'http://localhost:8080', '/public': 'http://localhost:8080', '/auth': 'http://localhost:8080' } },
})
