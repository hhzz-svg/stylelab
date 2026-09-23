import { defineConfig } from '@playwright/test'
import { APP_PORT, STUB_PORT } from './e2e/env'

// Browser smoke tests against the real Go server (embedding the built
// web/dist) and a stub model provider. Run `npm run build` first.
export default defineConfig({
  testDir: 'e2e',
  timeout: 60_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${APP_PORT}`,
    viewport: { width: 1440, height: 1000 },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // A preinstalled Chromium can be used instead of `playwright install`.
    launchOptions: { executablePath: process.env.E2E_CHROMIUM || undefined },
  },
  webServer: [
    {
      command: 'node e2e/stub-llm.mjs',
      url: `http://127.0.0.1:${STUB_PORT}/health`,
      env: { E2E_STUB_PORT: String(STUB_PORT) },
    },
    {
      command: 'sh e2e/start-server.sh',
      url: `http://127.0.0.1:${APP_PORT}/api/health`,
      env: { E2E_APP_PORT: String(APP_PORT) },
      timeout: 180_000,
    },
  ],
})
