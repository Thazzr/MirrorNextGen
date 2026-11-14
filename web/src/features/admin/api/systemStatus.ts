import { useQuery } from '@tanstack/react-query'
import { fetchInfo, infoQueryKey } from './info.ts'
import { fetchLogbook, logbookLine, logbookQueryKey } from './logbook.ts'
import { fetchSystemUpdate, systemUpdateQueryKey, updateIsActive } from './system.ts'

export function useLogbookTileStatus() {
  // Same key as the page's "Warnungen" filter, so both share one cache entry.
  const query = useQuery({ queryKey: logbookQueryKey('warn'), queryFn: () => fetchLogbook('warn') })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  return { loading: false as const, text: logbookLine(query.data.entries) }
}

export function useSystemTileStatus() {
  const info = useQuery({ queryKey: infoQueryKey, queryFn: fetchInfo })
  // A failing update call only drops the suffix; the version is still worth showing.
  const update = useQuery({ queryKey: systemUpdateQueryKey, queryFn: fetchSystemUpdate })
  if (info.isPending) return { loading: true as const }
  if (info.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  let text = `Version ${info.data.version}`
  if (update.data?.stalled) text += ' · Update hängt'
  else if (updateIsActive(update.data)) text += ' · Update läuft'
  else if (update.data?.state === 'failed') text += ' · Update fehlgeschlagen'
  return { loading: false as const, text }
}
