import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'node:path';
export default defineConfig({ plugins: [react()], publicDir: false, resolve: { alias: { '@': resolve(import.meta.dirname, 'src') } }, define: { 'import.meta.env.VITE_AUTH_PROTOCOL': JSON.stringify('ops-v2') }, build: { outDir: 'dist-v2', emptyOutDir: true, rollupOptions: { input: resolve(import.meta.dirname, 'index-v2.html') } } });
