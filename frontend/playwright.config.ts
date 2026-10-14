import { defineConfig } from '@playwright/test';

// Parcours de bout en bout du mode API contre le serveur e2e du backend
// (vrai service, vraie base, faux GitHub). Base jetable : E2E_DATABASE_URL.
const port = 8090;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    launchOptions: process.env.PW_CHROMIUM ? { executablePath: process.env.PW_CHROMIUM } : {},
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm run build:api && cd ../backend && go run -tags e2e ./cmd/e2e',
    url: `http://127.0.0.1:${port}/api/v1/health`,
    timeout: 180_000,
    reuseExistingServer: false,
    env: {
      DATABASE_URL: process.env.E2E_DATABASE_URL ?? '',
      E2E_RESET: '1',
      FRONT_DIR: '../frontend/dist',
      HTTP_ADDR: `127.0.0.1:${port}`,
    },
  },
});
