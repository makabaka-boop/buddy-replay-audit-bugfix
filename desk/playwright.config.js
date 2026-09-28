import { defineConfig } from '@playwright/test';

// Run against the compose stack (or any desk deployment):
//   docker compose up -d --build
//   npx playwright install chromium
//   npm run e2e
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  use: {
    baseURL: process.env.DESK_URL || 'http://localhost:5173',
  },
});
