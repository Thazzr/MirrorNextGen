import { useQuery } from '@tanstack/react-query'
import { dashboardWifiQueryKey, fetchDashboardWifi } from './api.ts'

export function useWifiTileStatus() {
  const query = useQuery({ queryKey: dashboardWifiQueryKey, queryFn: fetchDashboardWifi })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  const { configured, ssid } = query.data
  return { loading: false as const, text: configured && ssid ? `„${ssid}“` : 'Aus' }
}
