import { vi } from 'vitest'

export type RecordedCall = { method: string; path: string; csrf: string | null; body: unknown }
export type Handler = Response | ((call: RecordedCall) => Response | Promise<Response>)

export const json = (body: unknown, init?: ResponseInit) => Response.json(body, init)
export const noContent = () => new Response(null, { status: 204 })
export const problem = (status: number, title: string, extra: Record<string, unknown> = {}, headers: HeadersInit = {}) =>
  new Response(JSON.stringify({ title, status, ...extra }), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })

/**
 * Stubs `fetch` underneath the generated client. Handlers are keyed "METHOD /path" and can be
 * replaced while a test runs; every request is recorded with its CSRF header and parsed body.
 */
export function mockApi(initial: Record<string, Handler> = {}) {
  const handlers: Record<string, Handler> = { ...initial }
  const calls: RecordedCall[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: Request) => {
      const text = await input.text()
      const call: RecordedCall = {
        method: input.method,
        path: new URL(input.url).pathname,
        csrf: input.headers.get('X-CSRF-Token'),
        body: text ? JSON.parse(text) : undefined,
      }
      calls.push(call)
      const handler = handlers[`${call.method} ${call.path}`]
      if (!handler) return problem(404, `unhandled ${call.method} ${call.path}`)
      return typeof handler === 'function' ? handler(call) : handler.clone()
    }),
  )
  return {
    handlers,
    calls,
    callsTo: (key: string) => calls.filter((c) => `${c.method} ${c.path}` === key),
  }
}

export const authedSession = { setupRequired: false, authenticated: true, csrfToken: 'tok-123' }
export const anonymousSession = { setupRequired: false, authenticated: false }
export const info = { version: '1.2.3', commit: 'abc1234', buildTime: '' }
