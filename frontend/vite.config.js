import { defineConfig } from 'vite'
import { createRequire } from 'module'
const require = createRequire(import.meta.url)
const monacoEditorPlugin = require('vite-plugin-monaco-editor')

// https://vite.dev/config/
export default defineConfig({
  root: '.',
  publicDir: 'public',
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: true,
  },
  server: {
    port: 5173,
    open: true,
  },
  plugins: [
    monacoEditorPlugin.default({
      languages: [
        'json',
        'javascript',
        'typescript',
        'python',
        'java',
        'go',
        'cpp',
        'csharp',
        'ruby',
        'php',
        'sql',
        'html',
        'css',
        'yaml',
        'xml',
        'markdown',
      ],
    }),
  ],
})
