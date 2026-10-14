import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => ({
  plugins: [react()],
  // Bac à sable : le backend compilé en WASM et wasm_exec.js (npm run sandbox:wasm).
  publicDir: mode === 'sandbox' ? 'public-sandbox' : 'public',
  optimizeDeps: { exclude: ['@electric-sql/pglite'] },
  // npm run dev:api : le front parle au backend local (même origine pour les cookies).
  server: { proxy: { '/api': 'http://localhost:8080' } },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
}));
