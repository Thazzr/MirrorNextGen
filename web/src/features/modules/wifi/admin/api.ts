import { api, unwrap } from '../../../../shared/api/client.ts'
import type { components } from '../../../../shared/api/schema.gen.ts'

export type WifiSettings = components['schemas']['WifiSettings']
export type WifiSettingsUpdate = components['schemas']['WifiSettingsUpdate']
export type DashboardWifi = components['schemas']['DashboardWifi']

export const wifiSettingsQueryKey = ['settings', 'wifi'] as const
export const dashboardWifiQueryKey = ['dashboard', 'wifi'] as const

export async function fetchWifiSettings(): Promise<WifiSettings> {
  return unwrap(await api.GET('/api/v1/settings/wifi'))
}

export async function saveWifiSettings(body: WifiSettingsUpdate): Promise<WifiSettings> {
  return unwrap(await api.PUT('/api/v1/settings/wifi', { body }))
}

export async function fetchDashboardWifi(): Promise<DashboardWifi> {
  return unwrap(await api.GET('/api/v1/dashboard/wifi'))
}
