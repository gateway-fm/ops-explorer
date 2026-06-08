/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: process.env.API_PROXY_TARGET || 'http://api:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: process.env.API_PROXY_TARGET || 'http://api:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    coverage: {
      provider: 'v8',
      // Count untested source files in the denominator so coverage reflects the
      // whole app, not just the files that happen to have a test. New 0%
      // files cannot inflate the number by being invisible.
      all: true,
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/*.test.{ts,tsx}',
        'src/**/*.d.ts',
        'src/test/**',
        'src/main.tsx',
        'src/vite-env.d.ts',
      ],
      reporter: ['text', 'text-summary', 'html', 'lcov'],
      // Low-but-real floor; ratchet upward as more units land. The point is to
      // fail CI if coverage REGRESSES, not to gate at a high bar yet. Set just
      // below the current whole-app numbers (vitest --coverage with all:true).
      thresholds: {
        lines: 5,
        statements: 5,
        functions: 4,
        branches: 3,
      },
    },
  },
})
