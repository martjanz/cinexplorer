import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// The build goes where the Go server embeds it. In development, `npm run
// dev` serves the app and sends /api and /img to the Go server, started
// with: go run ./cmd/cinexplorer -port 8080 -no-browser
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/server/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/img': 'http://127.0.0.1:8080',
    },
  },
  test: {
    include: ['src/**/*.test.js'],
  },
})
