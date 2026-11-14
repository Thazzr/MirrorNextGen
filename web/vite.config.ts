import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const outDir = '../server/internal/web/dist'

// emptyOutDir deletes the committed dist/.gitignore; the Go server embeds the
// directory and git needs this file to keep it, so write it back byte for byte.
// Only for the embedded directory itself: a build into another outDir (the
// verify gate builds into a temp dir) must not create files there, let alone
// touch the embedded directory.
function keepDistGitignore(): Plugin {
  let dir: string | null = null
  return {
    name: 'keep-dist-gitignore',
    apply: 'build',
    configResolved(config) {
      const resolved = resolve(config.root, config.build.outDir)
      dir = resolved === resolve(import.meta.dirname, outDir) ? resolved : null
    },
    closeBundle() {
      if (!dir) return
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
    // The gate runs the Go tests in parallel and CI runners are slower: leave headroom over the 5 s default.
    testTimeout: 15000,
    hookTimeout: 15000,
  },
})
