import { act, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi } from '../../test/mockApi.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

describe('DashboardPage', () => {
  beforeEach(() => {
    mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }) })
    vi.useFakeTimers()
    // 09:05:07 in Europe/Berlin (CET, UTC+1)
    vi.setSystemTime(new Date('2026-11-14T08:05:07Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('shows date, time, seconds and calendar week', () => {
    renderWithProviders('/')
    expect(screen.getByText('Samstag, 14. November 2026')).toBeInTheDocument()
    expect(screen.getByText('09:05')).toBeInTheDocument()
    expect(screen.getByText('07')).toBeInTheDocument()
    expect(screen.getByText('KW 46')).toBeInTheDocument()
  })

  it('ticks every second', () => {
    renderWithProviders('/')
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText('08')).toBeInTheDocument()
  })

  it('marks the body as kiosk only while mounted', () => {
    const { unmount } = renderWithProviders('/')
    expect(document.body).toHaveClass('mirror-kiosk')
    unmount()
    expect(document.body).not.toHaveClass('mirror-kiosk')
  })

  it('shows no diagnose overlay without the query parameter', () => {
    renderWithProviders('/')
    expect(screen.queryByTestId('diagnose')).not.toBeInTheDocument()
  })

  it('has exactly one h1', () => {
    renderWithProviders('/')
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
  })
})

describe('DashboardPage kiosk state', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('shows the diagnose overlay when the kiosk state asks for the test pattern', async () => {
    mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 90, output: '', testPattern: true }) })
    renderWithProviders('/')
    expect(await screen.findByTestId('diagnose')).toBeInTheDocument()
  })

  it('shows no overlay when the test pattern is off', async () => {
    const api = mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }) })
    renderWithProviders('/')
    await waitFor(() => expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1))
    await act(async () => {})
    expect(screen.queryByTestId('diagnose')).not.toBeInTheDocument()
  })

  it('stays calm when the kiosk state request fails', async () => {
    const api = mockApi({
      'GET /api/v1/kiosk/state': () => new Response(JSON.stringify({ title: 'Kaputt', status: 500 }), { status: 500 }),
    })
    renderWithProviders('/')
    await waitFor(() => expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1))
    await act(async () => {})
    expect(screen.queryByTestId('diagnose')).not.toBeInTheDocument()
    expect(screen.queryByText(/Kaputt|HTTP 500|Fehler/)).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
  })

  it('polls the kiosk state every five seconds', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
    const api = mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }) })
    renderWithProviders('/')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(2)
  })
})
