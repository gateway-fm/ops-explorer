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
      // `include` makes coverage span ALL matching source files, not just the
      // ones a test happens to import — so untested files count toward the
      // denominator at 0% and new 0% files cannot inflate the number by being
      // invisible. (vitest 4 dropped the explicit `all` flag; `include` is the
      // supported way to get whole-app coverage.)
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
      // below the current whole-app numbers (vitest --coverage with all:true:
      // ~8.4% stmts / 8.5% lines / 5.9% funcs / 5.4% branches after A11).
      thresholds: {
        lines: 8,
        statements: 8,
        functions: 5,
        branches: 5,
      },
    },
  },
})
