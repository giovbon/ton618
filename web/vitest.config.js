import { defineConfig } from 'vitest/config';
import preact from '@preact/preset-vite';

export default defineConfig({
  plugins: [preact()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/__tests__/setup.jsx'],
    exclude: ['**/node_modules/**', '**/e2e/**', '**/tests/**'],
  },
});
