import { beforeAll } from 'vitest'

/**
 * Loads the lazily imported admin chunk (Mantine and all pages) before the first test.
 * Otherwise the first test of a file pays the module transform inside its own
 * findBy timeout, which is short and fails intermittently on a loaded machine.
 * The hook runs under hookTimeout, not under the 1 s of the query.
 */
export function preloadAdmin() {
  beforeAll(async () => {
    await import('../features/admin/AdminApp.tsx')
    await (await import('../features/admin/modules.ts')).preloadAdminPages()
  })
}
