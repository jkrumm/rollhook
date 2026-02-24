import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    globalSetup: './setup/global.ts',
    testTimeout: 120_000,
    hookTimeout: 30_000,
    sequence: { concurrent: false },
    include: ['tests/**/*.test.ts'],
  },
})
