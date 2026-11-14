import { useQuery } from '@tanstack/react-query'
import { dashboardWeatherQueryKey, fetchDashboardWeather, type DashboardWeather } from './api.ts'

/** One line about the public weather state, e.g. "Berlin · 12°" ("… · Leichter Regen" with `withCondition`). */
export function weatherLine(weather: DashboardWeather, withCondition = false): string {
  if (weather.data) {
    const parts = [weather.data.location, `${weather.data.temperatureC}°`]
    if (withCondition) parts.push(weather.data.condition)
    return parts.join(' · ')
  }
  if (!weather.configured) return 'Nicht eingerichtet'
  if (weather.error) return `Fehler: ${weather.error}`
  return 'Noch keine Daten'
}

export function useWeatherTileStatus() {
  const query = useQuery({ queryKey: dashboardWeatherQueryKey, queryFn: fetchDashboardWeather })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  return { loading: false as const, text: weatherLine(query.data) }
}
