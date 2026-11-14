import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockApi } from './mockApi.ts'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('mockApi', () => {
  it('records a request whose body cannot be parsed, with the reason, and logs it', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    const api = mockApi({ 'POST /api/x': () => new Response(null, { status: 204 }) })
    const res = await fetch(new Request('http://localhost/api/x', { method: 'POST', body: '{not json' }))
    expect(res.status).toBe(204)
    const [call] = api.callsTo('POST /api/x')
    expect(call.body).toBeUndefined()
    expect(call.bodyError).toMatch(/JSON/i)
    expect(error).toHaveBeenCalledWith(expect.stringContaining('POST /api/x'))
  })

  it('reads a multipart body even though jsdom owns the global File', async () => {
    const api = mockApi({ 'POST /api/up': () => new Response(null, { status: 204 }) })
    // A real file part: Node's FormData and File, as the generated client sends them in the icon upload.
    const NodeFormData = (await new Response(new URLSearchParams()).formData()).constructor as typeof FormData
    const { File: NodeFile } = await import('node:buffer')
    const form = new NodeFormData()
    form.append('file', new NodeFile([new Uint8Array([1, 2])], 'a.png', { type: 'image/png' }) as unknown as File)
    form.append('name', 'Logo')
    await fetch(new Request('http://localhost/api/up', { method: 'POST', body: form }))
    const [call] = api.callsTo('POST /api/up')
    expect(call.bodyError).toBeUndefined()
    const parsed = call.body as FormData
    expect((parsed.get('file') as File).name).toBe('a.png')
    expect(parsed.get('name')).toBe('Logo')
    // The globals are restored afterwards.
    expect(globalThis.File).toBe(window.File)
  })
})
