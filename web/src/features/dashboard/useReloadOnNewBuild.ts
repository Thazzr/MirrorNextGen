import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../../shared/api/client.ts'

export const dashboardInfoQueryKey = ['dashboard', 'info'] as const

const infoInterval = 60_000

/** Seam for tests: jsdom cannot reload the page. */
export const pageReload = { reload: () => window.location.reload() }

/**
 * After an image update only the server restarts; the kiosk keeps the old bundle.
 * Remember the first build seen and reload once when a later answer reports another one.
 * A failed poll never reloads (the server is simply restarting).
 */
export function useReloadOnNewBuild() {
  const info = useQuery({
    queryKey: dashboardInfoQueryKey,
    queryFn: async () => unwrap(await api.GET('/api/v1/info')),
    refetchInterval: infoInterval,
  })
  const first = useRef<string | undefined>(undefined)
  const reloaded = useRef(false)
  const build = info.data ? `${info.data.version}\n${info.data.commit}` : undefined
  useEffect(() => {
    if (build === undefined) return
    if (first.current === undefined) {
      first.current = build
    } else if (build !== first.current && !reloaded.current) {
      reloaded.current = true
      pageReload.reload()
    }
  }, [build])
}
