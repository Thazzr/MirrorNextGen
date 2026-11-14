import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi } from '../../test/mockApi.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

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
})
