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

// Held in module scope because the token belongs to the one signed-in session of this tab.
let csrfToken: string | undefined

/** Remembers the CSRF token of the current session; `undefined` forgets it (logout). */
export function setCsrfToken(token: string | undefined) {
  csrfToken = token
}

api.use({
  onRequest({ request }) {
    if (csrfToken && request.method !== 'GET' && request.method !== 'HEAD') {
      request.headers.set('X-CSRF-Token', csrfToken)
    }
    return request
  },
})

export class ApiError extends Error {
  readonly status: number
  readonly detail?: string
  readonly fieldErrors?: Record<string, string>
  readonly retryAfterSeconds?: number

  constructor(
    status: number,
    message: string,
    extra: { detail?: string; fieldErrors?: Record<string, string>; retryAfterSeconds?: number } = {},
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.detail = extra.detail
    this.fieldErrors = extra.fieldErrors
    this.retryAfterSeconds = extra.retryAfterSeconds
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

function parseRetryAfter(response: Response): number | undefined {
  const seconds = Number.parseInt(response.headers.get('Retry-After') ?? '', 10)
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : undefined
}

function fieldErrorsOf(error: Partial<Problem>): Record<string, string> | undefined {
  const { errors } = error
  if (typeof errors !== 'object' || errors === null) return undefined
  const entries = Object.entries(errors).filter((entry): entry is [string, string] => typeof entry[1] === 'string')
  return entries.length > 0 ? Object.fromEntries(entries) : undefined
}

function toApiError(error: unknown, response: Response): ApiError {
  const problem = isProblem(error) ? error : undefined
  const message = problem ? (problem.title ?? problem.detail) : undefined
  const text = problem?.title && problem.detail ? `${problem.title}: ${problem.detail}` : message
  return new ApiError(response.status, text || `HTTP ${response.status}`, {
    detail: problem?.detail,
    fieldErrors: problem ? fieldErrorsOf(problem) : undefined,
    retryAfterSeconds: parseRetryAfter(response),
  })
}

/** Turns the client's `{ data, error }` result into a value or a thrown ApiError. */
export function unwrap<T>(result: ApiResult<T>): T {
  const { data, error, response } = result
  if (error !== undefined || !response.ok) {
    throw toApiError(error, response)
  }
  if (data === undefined) {
    throw new ApiError(response.status, 'Leere Antwort vom Server')
  }
  return data
}

/** Like `unwrap` for endpoints that answer 204 without a body. */
export function unwrapEmpty(result: ApiResult<unknown>): void {
  const { error, response } = result
  if (error !== undefined || !response.ok) {
    throw toApiError(error, response)
  }
}
