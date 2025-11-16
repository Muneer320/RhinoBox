import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.js'],
    globals: true,
    maxWorkers: 2,
    restoreMocks: true,
    clearMocks: true,
  },
})
