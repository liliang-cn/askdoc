import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  build: {
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      name: 'AskDoc',
      formats: ['es', 'umd'],
      fileName: (format) => `askdoc.${format}.js`,
    },
    rollupOptions: {
      output: {
        inlineDynamicImports: true,
      },
    },
  },
});
