import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, setCsrfToken, unwrap } from '../../../shared/api/client.ts'
import { sessionQueryKey } from '../../../shared/api/queryClient.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type Session = components['schemas']['Session']

/** Remembers the CSRF token that comes with every session answer. */
export function rememberSession(session: Session): Session {
  setCsrfToken(session.csrfToken)
  return session
}

export async function fetchSession(): Promise<Session> {
  return rememberSession(unwrap(await api.GET('/api/v1/auth/session')))
}

export function useSession() {
  // Kept until a 401 invalidates it: the session only changes through our own calls.
  return useQuery({ queryKey: sessionQueryKey, queryFn: fetchSession, staleTime: Infinity })
}

/** Writes a session state into the cache so the route guard reacts to it at once. */
export function useSetSession() {
  const queryClient = useQueryClient()
  return (session: Session) => queryClient.setQueryData(sessionQueryKey, session)
}
