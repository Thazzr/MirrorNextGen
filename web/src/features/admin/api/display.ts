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

export type DisplayStatus = components['schemas']['DisplayStatus']

export const displayStatusQueryKey = ['display', 'status'] as const

export async function fetchDisplayStatus(): Promise<DisplayStatus> {
  return unwrap(await api.GET('/api/v1/display/status'))
}

/** Switches the monitor off for ten seconds; answers the monitor state right after. */
export async function testDisplayOff(): Promise<DisplayStatus> {
  return unwrap(await api.POST('/api/v1/display/test-off'))
}
