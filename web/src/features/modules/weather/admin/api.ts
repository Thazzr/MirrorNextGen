import { api, unwrap } from '../../../../shared/api/client.ts'
import type { components } from '../../../../shared/api/schema.gen.ts'

export type WeatherSettings = components['schemas']['WeatherSettings']
export type WeatherSettingsUpdate = components['schemas']['WeatherSettingsUpdate']
export type DashboardWeather = components['schemas']['DashboardWeather']

export const weatherSettingsQueryKey = ['settings', 'weather'] as const
export const dashboardWeatherQueryKey = ['dashboard', 'weather'] as const

export async function fetchWeatherSettings(): Promise<WeatherSettings> {
  return unwrap(await api.GET('/api/v1/settings/weather'))
}

export async function saveWeatherSettings(body: WeatherSettingsUpdate): Promise<WeatherSettings> {
  return unwrap(await api.PUT('/api/v1/settings/weather', { body }))
}

export async function fetchDashboardWeather(): Promise<DashboardWeather> {
  return unwrap(await api.GET('/api/v1/dashboard/weather'))
}
