import { act } from '@testing-library/react'
import { focusManager, onlineManager } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { layoutOf, weatherData } from '../../test/dashboardFixtures.ts'
import { json, mockApi } from '../../test/mockApi.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'
import { createQueryClient } from './queryClient.ts'

describe('kiosk polling', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
    focusManager.setFocused(undefined)
    onlineManager.setOnline(true)
  })

  it('keeps refetching the layout and a widget while the page is hidden and the browser claims to be offline', async () => {
    const api = mockApi({
      'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
      'GET /api/v1/dashboard/layout': () => json(layoutOf({ topRight: ['weather'] })),
      'GET /api/v1/dashboard/weather': () => json({ configured: true, stale: false, data: weatherData() }),
    })
    renderWithProviders('/')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(api.callsTo('GET /api/v1/dashboard/layout')).toHaveLength(1)
    expect(api.callsTo('GET /api/v1/dashboard/weather')).toHaveLength(1)

    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' })
    focusManager.setFocused(false)
    onlineManager.setOnline(false)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(api.callsTo('GET /api/v1/dashboard/layout').length).toBeGreaterThanOrEqual(2)
    expect(api.callsTo('GET /api/v1/dashboard/weather')).toHaveLength(2)
  })

  it('leaves admin queries on the default network mode and background behaviour', () => {
    const client = createQueryClient()
    expect(client.getQueryDefaults(['settings', 'weather']).networkMode).toBeUndefined()
    expect(client.getQueryDefaults(['session']).refetchIntervalInBackground).toBeUndefined()
    expect(client.getQueryDefaults(['dashboard', 'weather'])).toMatchObject({ networkMode: 'always', refetchIntervalInBackground: true })
    expect(client.getQueryDefaults(['kiosk', 'state'])).toMatchObject({ networkMode: 'always', refetchIntervalInBackground: true })
  })
})
