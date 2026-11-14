import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type BringSettings = components['schemas']['BringSettings']
export type BringSettingsUpdate = components['schemas']['BringSettingsUpdate']
export type BringList = components['schemas']['BringList']
export type DashboardBring = components['schemas']['DashboardBring']
export type BringItem = components['schemas']['BringItem']

export const bringSettingsQueryKey = ['settings', 'bring'] as const
export const dashboardBringQueryKey = ['dashboard', 'bring'] as const

export async function fetchBringSettings(): Promise<BringSettings> {
  return unwrap(await api.GET('/api/v1/settings/bring'))
}

export async function saveBringSettings(body: BringSettingsUpdate): Promise<BringSettings> {
  return unwrap(await api.PUT('/api/v1/settings/bring', { body }))
}

export async function fetchBringLists(): Promise<BringList[]> {
  return unwrap(await api.GET('/api/v1/settings/bring/lists'))
}

export async function fetchDashboardBring(): Promise<DashboardBring> {
  return unwrap(await api.GET('/api/v1/dashboard/bring'))
}
