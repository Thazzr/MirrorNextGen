import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi } from '../../test/mockApi.ts'
import { layoutOf } from '../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

class FakeWebSocket {
  addEventListener() {}
  send() {}
  close() {}
}

afterEach(() => vi.unstubAllGlobals())

function stubState(state: Record<string, unknown>) {
  return mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false, ...state }) })
}

async function stateLoaded(api: ReturnType<typeof stubState>) {
  await waitFor(() => expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1))
}

describe('night schedule black layer', () => {
  it('covers the dashboard when the kiosk state says powerOn false', async () => {
    stubState({ powerOn: false })
    renderWithProviders('/')
    const layer = await screen.findByTestId('blackout')
    expect(layer).toHaveClass('mirror-blackout')
    // It sits next to the dashboard, not inside it, so no layout rule can clip it.
    expect(layer.closest('main')).toBeNull()
  })

  it('is absent while powerOn is true', async () => {
    const api = stubState({ powerOn: true })
    renderWithProviders('/')
    await stateLoaded(api)
    expect(screen.queryByTestId('blackout')).not.toBeInTheDocument()
  })

  it('is absent when an older server sends no powerOn', async () => {
    const api = stubState({})
    renderWithProviders('/')
    await stateLoaded(api)
    expect(screen.queryByTestId('blackout')).not.toBeInTheDocument()
  })

  it('is absent when the kiosk state cannot be loaded', async () => {
    const api = mockApi({ 'GET /api/v1/kiosk/state': () => new Response('{"title":"Kaputt","status":500}', { status: 500 }) })
    renderWithProviders('/')
    await stateLoaded(api)
    expect(screen.queryByTestId('blackout')).not.toBeInTheDocument()
  })

  it('also covers the window in browser rotation mode, outside the rotated wrapper', async () => {
    stubState({ rotation: 90, powerOn: false })
    renderWithProviders('/?rotation=browser')
    const layer = await screen.findByTestId('blackout')
    expect(screen.getByTestId('rotated-viewport')).toBeInTheDocument()
    expect(layer.closest('[data-testid="rotated-viewport"]')).toBeNull()
  })

  it('goes away when the monitor is switched on again', async () => {
    let powerOn = false
    mockApi({ 'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false, powerOn }) })
    const { queryClient } = renderWithProviders('/')
    await screen.findByTestId('blackout')
    powerOn = true
    await queryClient.invalidateQueries({ queryKey: ['kiosk', 'state'] })
    await waitFor(() => expect(screen.queryByTestId('blackout')).not.toBeInTheDocument())
  })

  it('renders no widget at night and brings them back when the monitor is switched on', async () => {
    vi.stubGlobal('WebSocket', FakeWebSocket)
    let powerOn = false
    mockApi({
      'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false, powerOn }),
      'GET /api/v1/dashboard/layout': () => json(layoutOf({ topLeft: ['clock'], topRight: ['camera'] })),
      'GET /api/v1/dashboard/camera': () =>
        json({ configured: true, title: 'Haustür', brightness: 100, contrast: 100, saturation: 100, grayscale: false }),
    })
    const { queryClient, container } = renderWithProviders('/')
    await screen.findByTestId('blackout')
    await new Promise((r) => setTimeout(r, 30))
    expect(container.querySelector('main')).toBeNull()
    expect(container.querySelector('.mirror-clock')).toBeNull()
    expect(document.querySelector('video-rtc')).toBeNull()

    powerOn = true
    await queryClient.invalidateQueries({ queryKey: ['kiosk', 'state'] })
    await waitFor(() => expect(container.querySelector('.mirror-clock')).not.toBeNull())
    await waitFor(() => expect(document.querySelector('video-rtc')).not.toBeNull())
    expect(screen.queryByTestId('blackout')).not.toBeInTheDocument()
  })
})
