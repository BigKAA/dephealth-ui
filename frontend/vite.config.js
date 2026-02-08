import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        manualChunks: {
          cytoscape: ['cytoscape', 'cytoscape-dagre', 'dagre'],
          'tom-select': ['tom-select'],
        },
      },
    },
  },
});
