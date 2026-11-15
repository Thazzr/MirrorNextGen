import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf, weatherData } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'
import { temperatureStops, valueRange } from './chart.ts'

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

  it('draws rain columns with their percentage', async () => {
    setup({ configured: true, stale: false, data: weatherData() })
    const { container } = renderWithProviders('/')
    await screen.findByText('17°')
    const bars = container.querySelectorAll('.mirror-weather__rain-bars rect')
    expect(bars).toHaveLength(1)
    expect(within(container.querySelector('.mirror-weather__chart-labels') as HTMLElement).getByText('40%')).toBeInTheDocument()
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
      await waitFor(() => expect(svg.getAttribute('viewBox')).toBe('0 0 640 290'))
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
      await waitFor(() => expect(svg.getAttribute('viewBox')).toBe('0 0 640 290'))
      const pointXs = [...svg.querySelectorAll(':scope > g:not(.mirror-weather__rain-bars) > circle')].map((c) => Number(c.getAttribute('cx')))
      const labelLefts = [...container.querySelectorAll<HTMLElement>('.mirror-weather__hour-labels > div')].map((el) => parseFloat(el.style.left))
      expect(pointXs).toHaveLength(9)
      expect(labelLefts).toHaveLength(9)
      pointXs.forEach((x, i) => expect(labelLefts[i]).toBeCloseTo((x / 640) * 100, 3))
    })

    it('draws all chart text as HTML: no SVG text, one value label per point at the point x', async () => {
      stubObserver(640)
      const data = weatherData()
      setup({ configured: true, stale: false, data })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const svg = container.querySelector('svg.mirror-weather__chart') as SVGSVGElement
      await waitFor(() => expect(svg.getAttribute('viewBox')).toBe('0 0 640 290'))
      expect(svg.querySelectorAll('text')).toHaveLength(0)
      const overlay = container.querySelector('.mirror-weather__chart-labels') as HTMLElement
      expect(overlay.tagName).toBe('DIV')
      const values = [...overlay.querySelectorAll<HTMLElement>('.mirror-weather__chart-value')]
      expect(values.map((el) => el.textContent)).toEqual(data.hourly.map((h) => `${h.temperatureC}°`))
      const pointXs = [...svg.querySelectorAll(':scope > g:not(.mirror-weather__rain-bars) > circle')].map((c) => Number(c.getAttribute('cx')))
      values.forEach((el, i) => expect(parseFloat(el.style.left)).toBeCloseTo((pointXs[i] / 640) * 100, 3))
    })

    it('has no grid lines and no scale labels, only the baseline and the "now" line', async () => {
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const svg = container.querySelector('svg.mirror-weather__chart') as SVGSVGElement
      const lines = [...svg.querySelectorAll('line')]
      expect(lines.map((l) => l.getAttribute('class')).sort()).toEqual(['mirror-weather__chart-baseline', 'mirror-weather__chart-now'])
      expect(container.querySelectorAll('.mirror-weather__chart-scale')).toHaveLength(0)
    })

    it('colours the line with a vertical gradient whose stops follow the fixed thresholds of the data', async () => {
      stubObserver(640)
      const data = weatherData()
      data.hourly.forEach((h, i) => (h.temperatureC = [-2, 4, 12, 25, 18, 9, 1, -1, 3][i]))
      setup({ configured: true, stale: false, data })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const paths = [...container.querySelectorAll('path.mirror-weather__chart-temp')]
      expect(paths).toHaveLength(2)
      const expected = temperatureStops(-2, 25)
      expect(expected.length).toBeGreaterThan(2)
      for (const path of paths) {
        const id = /^url\(#(.+)\)$/.exec(path.getAttribute('stroke') ?? '')?.[1]
        expect(id).toBeTruthy()
        const gradient = container.querySelector(`linearGradient[id="${id}"]`) as Element
        expect(gradient.getAttribute('gradientUnits')).toBe('userSpaceOnUse')
        const stops = [...gradient.querySelectorAll('stop')].map((st) => ({ offset: Number(st.getAttribute('offset')), color: st.getAttribute('stop-color') }))
        expect(stops).toEqual(expected)
        // Lowest value at the gradient start, highest at its end: same mapping as the points.
        const ys = [...container.querySelectorAll('circle.mirror-weather__chart-point')].map((c) => Number(c.getAttribute('cy')))
        expect(Number(gradient.getAttribute('y1'))).toBeCloseTo(Math.max(...ys), 6)
        expect(Number(gradient.getAttribute('y2'))).toBeCloseTo(Math.min(...ys), 6)
      }
    })

    it('does not blow a flat day up to the full plot height: the padded range drives points and gradient', async () => {
      stubObserver(640)
      const data = weatherData()
      data.hourly.forEach((h, i) => (h.temperatureC = [1, 1, 2, 1, 1, 2, 1, 1, 1][i]))
      setup({ configured: true, stale: false, data })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const ys = [...container.querySelectorAll('circle.mirror-weather__chart-point')].map((c) => Number(c.getAttribute('cy')))
      const plotHeight = 175 - 45
      expect(Math.max(...ys) - Math.min(...ys)).toBeLessThanOrEqual(plotHeight / 10 + 1e-6)
      const { low, high } = valueRange([1, 2])
      const expected = temperatureStops(low, high)
      for (const path of container.querySelectorAll('path.mirror-weather__chart-temp')) {
        const id = /^url\(#(.+)\)$/.exec(path.getAttribute('stroke') ?? '')?.[1]
        const gradient = container.querySelector(`linearGradient[id="${id}"]`) as Element
        const stops = [...gradient.querySelectorAll('stop')].map((st) => ({ offset: Number(st.getAttribute('offset')), color: st.getAttribute('stop-color') }))
        expect(stops).toEqual(expected)
        // Gradient spans the padded range: its ends are the plot's bottom and top, the points sit in between.
        expect(Number(gradient.getAttribute('y1'))).toBe(175)
        expect(Number(gradient.getAttribute('y2'))).toBe(45)
      }
    })

    it('draws a rain column only for p > 0: p x 0.75 high, labelled "p%", dimmed when past', async () => {
      const data = weatherData()
      data.hourly[6].precipitationProbability = 60 // 18:00, in the future
      setup({ configured: true, stale: false, data })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const columns = [...container.querySelectorAll<SVGRectElement>('rect.mirror-weather__rain-col')]
      expect(columns).toHaveLength(2)
      const [past, future] = columns // 09:00 with 40 %, 18:00 with 60 %
      expect(Number(past.getAttribute('height'))).toBeCloseTo(30, 6)
      expect(Number(future.getAttribute('height'))).toBeCloseTo(45, 6)
      expect(future.getAttribute('width')).toBe('28')
      expect(future.getAttribute('rx')).toBe('8')
      expect(future.getAttribute('fill')).toBe('#5aa9ff')
      expect(Number(future.getAttribute('opacity'))).toBeCloseTo(0.95, 6)
      expect(past.parentElement?.classList.contains('mirror-weather__chart-past')).toBe(true)
      expect(future.parentElement?.classList.contains('mirror-weather__chart-past')).toBe(false)
      const labels = [...container.querySelectorAll('.mirror-weather__chart-rain')]
      expect(labels.map((el) => el.textContent)).toEqual(['40%', '60%'])
      expect(labels[0].classList.contains('mirror-weather__chart-past')).toBe(true)
      expect(labels[1].classList.contains('mirror-weather__chart-past')).toBe(false)
    })

    it('draws a gradient area that closes on the bottom plot line, below the line, in both clipped halves', async () => {
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const areas = [...container.querySelectorAll('path.mirror-weather__chart-area')]
      expect(areas).toHaveLength(2)
      // The plot's bottom line, not the lowest point: a flat day's values are padded and stay above it.
      const bottom = 175
      for (const area of areas) {
        const d = area.getAttribute('d') as string
        expect(d).toMatch(new RegExp(`L [\\d.]+,${bottom} L [\\d.]+,${bottom} Z$`))
        const id = /^url\(#(.+)\)$/.exec(area.getAttribute('fill') ?? '')?.[1]
        expect(id).toBeTruthy()
        expect(container.querySelector(`linearGradient[id="${id}"]`)).not.toBeNull()
        expect(area.nextElementSibling?.classList.contains('mirror-weather__chart-temp')).toBe(true)
      }
    })

    it('gives the points a black outline so the line passes behind them cleanly', async () => {
      setup({ configured: true, stale: false, data: weatherData() })
      const { container } = renderWithProviders('/')
      await screen.findByText('17°')
      const points = container.querySelectorAll('circle.mirror-weather__chart-point')
      expect(points).toHaveLength(9)
      expect(points[0].getAttribute('r')).toBe('4.5')
      points.forEach((p) => {
        expect(p.getAttribute('stroke')).toBe('#000')
        expect(p.getAttribute('stroke-width')).toBe('2')
      })
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
