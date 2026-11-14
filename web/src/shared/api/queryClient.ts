import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from './client.ts'

export const sessionQueryKey = ['session'] as const

/**
 * One factory for the app and the tests. A 401 from any call means the session is gone,
 * so the session query is refetched and the route guard sends the user to the login.
 */
export function createQueryClient({ retry = 1 }: { retry?: number | false } = {}): QueryClient {
  const onError = (error: unknown) => {
    if (error instanceof ApiError && error.status === 401) {
      void client.invalidateQueries({ queryKey: sessionQueryKey })
    }
  }
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
    defaultOptions: { queries: { retry, refetchOnWindowFocus: false } },
  })
  // The kiosk Chromium (Cage, no user) may report the page as hidden or the network as offline, and React Query
  // would then pause every interval refetch: the mirror would show admin changes only after a reboot.
  // Dashboard and kiosk queries therefore keep polling regardless; admin queries keep the defaults.
  for (const prefix of ['dashboard', 'kiosk']) {
    client.setQueryDefaults([prefix], { refetchIntervalInBackground: true, networkMode: 'always' })
  }
  return client
}
