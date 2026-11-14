import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf, weatherData } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'

function setup(weather: unknown | Response, layout = layoutOf({ topRight: ['weather'] })) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => json(layout),
    'GET /api/v1/dashboard/weather': () => (weather instanceof Response ? weather.clone() : json(weather)),
  })
}

describe('weather widget', () => {
  beforeEach(() => {
    // 09:05 in Europe/Berlin on Saturday; Sunday the 15th is "Morgen".
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-14T08:05:07Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('shows temperature, condition and the animated icon mapped from the OpenWeather code', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    expect(await screen.findByText('17°')).toBeInTheDocument()
    expect(screen.getAllByText('Leichter Regen').length).toBeGreaterThan(0)
    const current = container.querySelector('.mirror-weather__current img')
    expect(current).toHaveAttribute('src', '/assets/weather/meteocons/animated/rain.svg')
  })

  it('hides the big icon when showIcons is off', async () => {
    setup({ configured: true, stale: false, data: weatherData({ showIcons: false }) })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    expect(container.querySelector('.mirror-weather__current img')).toBeNull()
  })

  it('labels the first forecast day "Morgen" and the others by weekday', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    renderWithProviders('/')
    expect(await screen.findByText('Morgen')).toBeInTheDocument()
    expect(screen.getByText('Montag')).toBeInTheDocument()
    expect(screen.getByText('Dienstag')).toBeInTheDocument()
    expect(screen.getByText('3 Tage Vorhersage')).toBeInTheDocument()
    expect(screen.getByText('· 2,4 mm', { exact: false })).toBeInTheDocument()
  })

  it('does not call the first day "Morgen" when it is not tomorrow', async () => {
    const data = weatherData()
    data.forecast[0].date = '2026-11-16'
    setup({ configured: true, stale: false, data })
    renderWithProviders('/')
    await screen.findByText('17°')
    expect(screen.queryByText('Morgen')).not.toBeInTheDocument()
  })

  it('shows the last chart point as 24:00 and the others as clock times', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    const labels = [...container.querySelectorAll('.mirror-weather__hour-labels > div span')].map((el) => el.textContent)
    expect(labels).toEqual(['00:00', '03:00', '06:00', '09:00', '12:00', '15:00', '18:00', '21:00', '24:00'])
  })

  it('dims hour labels that lie in the past', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    // 00:00, 03:00, 06:00 are before 09:05.
    expect(container.querySelectorAll('.mirror-weather__hour-label--past')).toHaveLength(4)
  })

  it('draws rain bars with their percentage', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    const bars = container.querySelectorAll('.mirror-weather__rain-bars rect')
    expect(bars).toHaveLength(1)
    expect(within(container.querySelector('.mirror-weather__rain-bars') as HTMLElement).getByText('40%')).toBeInTheDocument()
  })

  describe('temperature chart', () => {
    function stubObserver(width: number) {
      vi.stubGlobal(
        'ResizeObserver',
        class {
          callback: ResizeObserverCallback
          constructor(callback: ResizeObserverCallback) {
            this.callback = callback
          }
          observe() {
            this.callback([{ contentRect: { width } } as ResizeObserverEntry], this as unknown as ResizeObserver)
          }
          unobserve() {}
          disconnect() {}
        },
      )
    }

    it('draws in real pixels: viewBox follows the measured width and is not stretched', async () => {
      stubObserver(640)
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const svg = container.querySelector('svg.mirror-weather__chart') as SVGSVGElement
      await waitFor(() => expect(svg.getAttribute('viewBox')).toBe('0 0 640 265'))
      expect(svg.hasAttribute('preserveAspectRatio')).toBe(false)
    })

    it('draws one continuous temperature curve, dimmed before "now" through a clip of the same path', async () => {
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const paths = [...container.querySelectorAll('path.mirror-weather__chart-temp')]
      expect(paths).toHaveLength(2)
      expect(paths[0].getAttribute('d')).toBe(paths[1].getAttribute('d'))
      expect(paths[0].getAttribute('d')).toMatch(/^M [\d.]+,[\d.]+ C /)
      expect(paths[0].closest('[clip-path]')?.classList.contains('mirror-weather__chart-past')).toBe(true)
      expect(paths[1].closest('[clip-path]')?.classList.contains('mirror-weather__chart-past')).toBe(false)
    })

    it('puts every hour label at the x position of its point', async () => {
      stubObserver(640)
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const svg = container.querySelector('svg.mirror-weather__chart') as SVGSVGElement
      await waitFor(() => expect(svg.getAttribute('viewBox')).toBe('0 0 640 265'))
      const pointXs = [...svg.querySelectorAll(':scope > g:not(.mirror-weather__rain-bars) > circle')].map((c) => Number(c.getAttribute('cx')))
      const labelLefts = [...container.querySelectorAll<HTMLElement>('.mirror-weather__hour-labels > div')].map((el) => parseFloat(el.style.left))
      expect(pointXs).toHaveLength(9)
      expect(labelLefts).toHaveLength(9)
      pointXs.forEach((x, i) => expect(labelLefts[i]).toBeCloseTo((x / 640) * 100, 3))
    })
  })

  it('shows the alert line', async () => {
    setup({ configured: true, stale: false, data: weatherData({ alert: 'Ab 15 Uhr Regen' }) })
    renderWithProviders('/')
    expect(await screen.findByText('Ab 15 Uhr Regen')).toBeInTheDocument()
  })

  it('shows the error text when there is no data', async () => {
    setup({ configured: true, stale: false, error: 'Der Wetterdienst antwortet nicht.' })
    renderWithProviders('/')
    expect(await screen.findByText('Der Wetterdienst antwortet nicht.')).toHaveClass('mirror-error')
  })

  it('renders nothing when the weather is not configured', async () => {
    const api = setup({ configured: false, stale: false })
    const { container } = renderWithProviders('/')
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/weather')).toHaveLength(1))
    await vi.waitFor(() => expect(screen.queryByText('Wetterdaten werden geladen …')).not.toBeInTheDocument())
    expect(container.querySelector('.mirror-weather')).toBeNull()
  })

  it('shows data plus a muted "Stand" line when stale', async () => {
    setup({ configured: true, stale: true, data: weatherData({ updatedAt: '2026-11-14T06:30:00Z' }) })
    renderWithProviders('/')
    expect(await screen.findByText('17°')).toBeInTheDocument()
    expect(screen.getByText('Stand: 07:30')).toBeInTheDocument()
  })

  it('shows no "Stand" line when the data is fresh', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    renderWithProviders('/')
    await screen.findByText('17°')
    expect(screen.queryByText(/Stand:/)).not.toBeInTheDocument()
  })

  it('shows a loading hint first and a failure text when the request fails', async () => {
    setup(problem(500, 'Kaputt'))
    renderWithProviders('/')
    expect(screen.getByText('Wetterdaten werden geladen …')).toBeInTheDocument()
    expect(await screen.findByText('Wetterdaten sind nicht verfügbar.')).toBeInTheDocument()
  })

  it('aligns to the left in a left region', async () => {
    setup({ configured: true, stale: false, data: weatherData() }, layoutOf({ topLeft: ['weather'] }))
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    expect(container.querySelector('.mirror-weather')).toHaveClass('mirror-weather--left')
  })

  it('is not aligned to the left in a right region', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    expect(container.querySelector('.mirror-weather')).not.toHaveClass('mirror-weather--left')
  })
})
