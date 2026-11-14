import { api, unwrap } from '../../../shared/api/client.ts'

export const infoQueryKey = ['info'] as const

export async function fetchInfo() {
  return unwrap(await api.GET('/api/v1/info'))
}
