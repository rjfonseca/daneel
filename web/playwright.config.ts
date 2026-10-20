import { defineConfig, devices } from '@playwright/test';

const embedded = process.env.DANEEL_E2E_EMBEDDED === '1';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: 0,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: embedded ? 'http://localhost:8088' : 'http://localhost:5173',
    trace: 'retain-on-failure',
    launchOptions: {
      args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader']
    }
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: [
    {
      command: 'node tests/e2e/start-backend.mjs',
      port: 8088,
      reuseExistingServer: false,
      timeout: 300 * 1000,
    },
    ...(embedded ? [] : [{
      command: process.env.DANEEL_E2E_PREVIEW === '1'
        ? 'npm run preview -- --host 127.0.0.1 --port 5173 --strictPort'
        : 'npm run dev -- --strictPort',
      env: { VITE_API_URL: 'http://127.0.0.1:8088' },
      port: 5173,
      reuseExistingServer: false,
    }]),
  ],
});
