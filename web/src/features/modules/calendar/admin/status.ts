import { useQuery } from '@tanstack/react-query'
import { calendarSettingsQueryKey, fetchCalendarSettings } from './api.ts'

export function useCalendarTileStatus() {
  const query = useQuery({ queryKey: calendarSettingsQueryKey, queryFn: fetchCalendarSettings })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  const c = query.data.calendars.length
  const s = query.data.sections.length
  return {
    loading: false as const,
    text: `${c} Kalender · ${s === 1 ? '1 Bereich' : `${s} Bereiche`}`,
  }
}
