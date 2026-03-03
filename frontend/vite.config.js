import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  resolve: {
    alias: {
      // elkjs bundled build tries to require('web-worker') for optional Worker support.
      // In the browser we use the synchronous (non-worker) variant, so stub it out.
      'web-worker': '/dev/null',
    },
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        manualChunks: {
          cytoscape: ['cytoscape', 'cytoscape-elk', 'elkjs'],
          'tom-select': ['tom-select'],
        },
      },
    },
  },
});
