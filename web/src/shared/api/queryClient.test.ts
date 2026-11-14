import { describe, expect, it, vi } from 'vitest'
import { ApiError } from './client.ts'
import { createQueryClient, sessionQueryKey } from './queryClient.ts'

describe('createQueryClient', () => {
  it('invalidates the session query when a query fails with 401', async () => {
    const client = createQueryClient({ retry: false })
    const spy = vi.spyOn(client, 'invalidateQueries')
    await client
      .fetchQuery({
        queryKey: ['x'],
        queryFn: () => {
          throw new ApiError(401, 'Nicht angemeldet')
        },
      })
      .catch(() => {})
    expect(spy).toHaveBeenCalledWith({ queryKey: sessionQueryKey })
  })

  it('invalidates the session query when a mutation fails with 401', async () => {
    const client = createQueryClient({ retry: false })
    const spy = vi.spyOn(client, 'invalidateQueries')
    await client
      .getMutationCache()
      .build(client, {
        mutationFn: async () => {
          throw new ApiError(401, 'Nicht angemeldet')
        },
      })
      .execute(undefined)
      .catch(() => {})
    expect(spy).toHaveBeenCalledWith({ queryKey: sessionQueryKey })
  })

  it('leaves other errors alone', async () => {
    const client = createQueryClient({ retry: false })
    const spy = vi.spyOn(client, 'invalidateQueries')
    await client
      .fetchQuery({
        queryKey: ['y'],
        queryFn: () => {
          throw new ApiError(500, 'Kaputt')
        },
      })
      .catch(() => {})
    expect(spy).not.toHaveBeenCalled()
  })
})
