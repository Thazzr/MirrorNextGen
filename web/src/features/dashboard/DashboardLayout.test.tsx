import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../test/mockApi.ts'
import { layoutOf, weatherData } from '../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

const wifi = { configured: true, ssid: 'Gast', qr: { size: 1, rows: ['1'] } }

function setup(layout: unknown | Response) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => (layout instanceof Response ? layout.clone() : json(layout)),
    'GET /api/v1/dashboard/weather': () => json({ configured: true, stale: false, data: weatherData() }),
    'GET /api/v1/dashboard/wifi': () => json(wifi),
  })
}

const region = (container: HTMLElement, name: string) => container.querySelector(`[data-region="${name}"]`)

describe('dashboard regions', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('renders the widgets of a region in the configured order', async () => {
    setup(layoutOf({ topRight: ['wifi', 'weather'] }))
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    await screen.findByRole('img', { name: 'WLAN Gast' })
    const children = [...(region(container, 'topRight')?.children ?? [])].map((el) => el.tagName + '.' + el.className.split(' ')[0])
    expect(children).toEqual(['ASIDE.mirror-wifi', 'SECTION.mirror-weather'])
  })

  it('places each key in its own region', async () => {
    setup(layoutOf({ topLeft: ['weather'], middle: ['clock'], bottomLeft: ['wifi'] }))
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    expect(region(container, 'topLeft')?.querySelector('.mirror-weather')).not.toBeNull()
    expect(region(container, 'middle')?.querySelector('.mirror-clock')).not.toBeNull()
    expect(await screen.findByRole('img', { name: 'WLAN Gast' })).toBeInTheDocument()
    expect(region(container, 'bottomLeft')?.querySelector('.mirror-wifi')).not.toBeNull()
    expect(region(container, 'topRight')).toBeNull()
    expect(region(container, 'bottomRight')).toBeNull()
  })

  it('skips unknown keys silently', async () => {
    setup(layoutOf({ topLeft: ['calendar:familie', 'toString', 'clock'] }))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(region(container, 'topLeft')?.children).toHaveLength(1))
    expect(region(container, 'topLeft')?.querySelector('.mirror-clock')).not.toBeNull()
  })

  it('filters unknown keys in every region', async () => {
    setup(layoutOf({ middle: ['calendar:x', 'toString', 'clock'], bottomLeft: ['constructor'], bottomRight: ['unbekannt', 'wifi'] }))
    const { container } = renderWithProviders('/')
    expect(await screen.findByRole('img', { name: 'WLAN Gast' })).toBeInTheDocument()
    await waitFor(() => expect(region(container, 'middle')?.children).toHaveLength(1))
    expect(region(container, 'middle')?.querySelector('.mirror-clock')).not.toBeNull()
    expect(region(container, 'bottomRight')?.children).toHaveLength(1)
    expect(region(container, 'bottomLeft')).toBeNull()
  })

  it('renders no region element for an empty region or one with only unknown keys', async () => {
    const api = setup(layoutOf({ topRight: ['unbekannt'] }))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/layout')).toHaveLength(1))
    await waitFor(() => expect(container.querySelector('.mirror-clock')).toBeNull())
    expect(container.querySelector('[data-region]')).toBeNull()
    expect(container.querySelector('.mirror-header')).toBeNull()
  })

  it('shows the default layout before the layout answers', async () => {
    mockApi({
      'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
      'GET /api/v1/dashboard/layout': () => new Promise<Response>(() => {}),
      'GET /api/v1/dashboard/weather': () => json({ configured: false, stale: false }),
      'GET /api/v1/dashboard/wifi': () => json(wifi),
    })
    const { container } = renderWithProviders('/')
    expect(region(container, 'topLeft')?.querySelector('.mirror-clock')).not.toBeNull()
    expect(region(container, 'topRight')).not.toBeNull()
    // The built-in fallback holds the Bring! list at the bottom left, like the server default.
    expect(region(container, 'bottomLeft')).not.toBeNull()
    expect(await screen.findByRole('img', { name: 'WLAN Gast' })).toBeInTheDocument()
    expect(region(container, 'bottomRight')?.querySelector('.mirror-wifi')).not.toBeNull()
  })

  it('keeps the default layout when the layout request fails', async () => {
    const api = setup(problem(500, 'Kaputt'))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/layout')).toHaveLength(1))
    await screen.findByText('17°')
    expect(region(container, 'topLeft')?.querySelector('.mirror-clock')).not.toBeNull()
    expect(region(container, 'bottomRight')?.querySelector('.mirror-wifi')).not.toBeNull()
    expect(screen.queryByText(/Kaputt/)).not.toBeInTheDocument()
  })

  it('follows the server layout once it is known', async () => {
    setup(layoutOf({ middle: ['clock'] }))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(region(container, 'middle')?.querySelector('.mirror-clock')).not.toBeNull())
    await waitFor(() => expect(region(container, 'topLeft')).toBeNull())
    expect(region(container, 'topRight')).toBeNull()
  })
})
