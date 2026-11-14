import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type DisplaySettings = components['schemas']['DisplaySettings']

export const displaySettingsQueryKey = ['settings', 'display'] as const

export async function fetchDisplaySettings(): Promise<DisplaySettings> {
  return unwrap(await api.GET('/api/v1/settings/display'))
}

export async function saveDisplaySettings(body: DisplaySettings): Promise<DisplaySettings> {
  return unwrap(await api.PUT('/api/v1/settings/display', { body }))
}
