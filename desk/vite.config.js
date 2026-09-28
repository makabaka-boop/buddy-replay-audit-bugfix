import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      // allocator service; inside compose the desk nginx proxies /api itself
      '/api': 'http://localhost:8080',
    },
  },
});
