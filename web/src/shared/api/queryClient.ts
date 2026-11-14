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
  return client
}
