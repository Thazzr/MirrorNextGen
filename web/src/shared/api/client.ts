import createClient from 'openapi-fetch'
import type { components, paths } from './schema.gen.ts'

type Problem = components['schemas']['Problem']

// The wrapper looks up globalThis.fetch per call, so tests can stub it at any time.
// In the browser the relative URL resolves against the page; outside it (jsdom) the
// origin has to be given because Request rejects relative URLs.
export const api = createClient<paths>({
  baseUrl: typeof location === 'undefined' ? '' : location.origin,
  fetch: (input) => globalThis.fetch(input),
})

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

type ApiResult<T> = {
  data?: T
  error?: unknown
  response: Response
}

function isProblem(value: unknown): value is Partial<Problem> {
  return typeof value === 'object' && value !== null
}

/** Turns the client's `{ data, error }` result into a value or a thrown ApiError. */
export function unwrap<T>(result: ApiResult<T>): T {
  const { data, error, response } = result
  if (error !== undefined || !response.ok) {
    const message = isProblem(error) ? (error.title ?? error.detail) : undefined
    const detail = isProblem(error) && error.title && error.detail ? `${error.title}: ${error.detail}` : message
    throw new ApiError(response.status, detail || `HTTP ${response.status}`)
  }
  if (data === undefined) {
    throw new ApiError(response.status, 'Leere Antwort vom Server')
  }
  return data
}
