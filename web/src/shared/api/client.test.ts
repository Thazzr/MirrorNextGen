import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, unwrap } from './client.ts'

function stubFetch(response: Response) {
  vi.stubGlobal('fetch', vi.fn(async () => response))
}

async function getInfo() {
  return unwrap(await api.GET('/api/v1/info'))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('unwrap', () => {
  it('returns the data of a successful response', async () => {
    const body = { version: '1.2.3', commit: 'abc', buildTime: '' }
    stubFetch(Response.json(body))
    await expect(getInfo()).resolves.toEqual(body)
  })

  it('throws an ApiError with the problem title on a problem+json error', async () => {
    stubFetch(
      new Response(JSON.stringify({ title: 'Kaputt', status: 500 }), {
        status: 500,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    )
    const error = await getInfo().catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 500, message: 'Kaputt' })
  })

  it('throws HTTP <status> for a non-ok response without a problem body', async () => {
    stubFetch(new Response(null, { status: 503 }))
    const error = await getInfo().catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 503, message: 'HTTP 503' })
  })

  it('throws when an ok response has no data', async () => {
    stubFetch(new Response(null, { status: 200 }))
    const error = await getInfo().catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ message: 'Leere Antwort vom Server' })
  })

  it('throws for a non-ok response even when data and error are set as if it succeeded', () => {
    const result = {
      data: { version: 'x', commit: 'y', buildTime: '' },
      error: undefined,
      response: new Response(null, { status: 500 }),
    }
    expect(() => unwrap(result)).toThrow(ApiError)
    expect(() => unwrap(result)).toThrow('HTTP 500')
    try {
      unwrap(result)
    } catch (e) {
      expect(e).toMatchObject({ status: 500 })
    }
  })
})
