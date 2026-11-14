import { Blob as NodeBlob, File as NodeFile } from 'node:buffer'
import { vi } from 'vitest'

export type RecordedCall = { method: string; path: string; csrf: string | null; body: unknown; bodyError?: string }
export type Handler = Response | ((call: RecordedCall) => Response | Promise<Response>)

export const json = (body: unknown, init?: ResponseInit) => Response.json(body, init)
export const noContent = () => new Response(null, { status: 204 })
export const problem = (status: number, title: string, extra: Record<string, unknown> = {}, headers: HeadersInit = {}) =>
  new Response(JSON.stringify({ title, status, ...extra }), {
    status,
    headers: { 'Content-Type': 'application/problem+json', ...headers },
  })

/**
 * Reads a multipart body. undici 7's parser asserts that every part is an instance of the *global* File,
 * which under jsdom is jsdom's class, not the one undici builds (undici 6 did not check). So the parse runs
 * with Node's File and Blob in place of jsdom's and puts them back afterwards.
 */
export async function readFormData(request: Request): Promise<FormData> {
  const g = globalThis as { File: unknown; Blob: unknown }
  const { File, Blob } = g
  g.File = NodeFile
  g.Blob = NodeBlob
  try {
    return await request.clone().formData()
  } finally {
    g.File = File
    g.Blob = Blob
  }
}

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
      // Multipart bodies (icon upload) are kept as FormData, everything else is parsed as JSON.
      const multipart = (input.headers.get('Content-Type') ?? '').startsWith('multipart/form-data')
      const call: RecordedCall = {
        method: input.method,
        path: new URL(input.url).pathname,
        csrf: input.headers.get('X-CSRF-Token'),
        body: undefined,
      }
      // A body that cannot be parsed must not hide the request: record it with the reason and say so loudly.
      try {
        if (multipart) call.body = await readFormData(input)
        else {
          const text = await input.text()
          call.body = text ? JSON.parse(text) : undefined
        }
      } catch (error) {
        call.bodyError = error instanceof Error ? error.message : String(error)
        console.error(`mockApi: cannot parse the body of ${call.method} ${call.path}: ${call.bodyError}`)
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
