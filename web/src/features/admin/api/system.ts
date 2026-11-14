import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type UpdateStatus = components['schemas']['UpdateStatus']

export const systemUpdateQueryKey = ['system', 'update'] as const

export async function fetchSystemUpdate(): Promise<UpdateStatus> {
  return unwrap(await api.GET('/api/v1/system/update'))
}

/** Asks the host to update; answers the update state right after. */
export async function requestSystemUpdate(): Promise<UpdateStatus> {
  return unwrap(await api.POST('/api/v1/system/update'))
}

/**
 * An update is requested or running: the page polls and hides the button. A
 * stalled request (nobody picked it up) is not active; it can be requested again.
 */
export const updateIsActive = (s: UpdateStatus | undefined): boolean =>
  s !== undefined && ((s.pending && !s.stalled) || s.state === 'running')

/** Poll interval while an update is active. */
export const UPDATE_POLL_MS = 3000
