import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const outDir = '../server/internal/web/dist'

// emptyOutDir deletes the committed dist/.gitignore; the Go server embeds the
// directory and git needs this file to keep it, so write it back byte for byte.
function keepDistGitignore(): Plugin {
  return {
    name: 'keep-dist-gitignore',
    apply: 'build',
    closeBundle() {
      const dir = resolve(import.meta.dirname, outDir)
      mkdirSync(dir, { recursive: true })
      writeFileSync(resolve(dir, '.gitignore'), '*\n!.gitignore\n')
    },
  }
}

export default defineConfig({
  plugins: [react(), keepDistGitignore()],
  build: { outDir, emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8080' } },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test/setup.ts'],
    env: { TZ: 'Europe/Berlin' },
  },
})
