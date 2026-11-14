import { useQuery } from '@tanstack/react-query'
import { dashboardBringQueryKey, fetchDashboardBring, type DashboardBring } from '../api.ts'

/** One line about the public list state, e.g. "12 Einträge". */
export function bringLine(bring: DashboardBring): string {
  if (!bring.configured) return 'Nicht eingerichtet'
  if (bring.error) return `Fehler: ${bring.error}`
  const n = bring.items.length
  return n === 1 ? '1 Eintrag' : `${n} Einträge`
}

export function useBringTileStatus() {
  const query = useQuery({ queryKey: dashboardBringQueryKey, queryFn: fetchDashboardBring })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  return { loading: false as const, text: bringLine(query.data) }
}
