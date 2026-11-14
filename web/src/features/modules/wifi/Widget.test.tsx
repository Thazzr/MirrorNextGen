import { screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'

function matrix(size: number, dark: (x: number, y: number) => boolean) {
  return Array.from({ length: size }, (_, y) => Array.from({ length: size }, (_, x) => (dark(x, y) ? '1' : '0')).join(''))
}

function setup(wifi: unknown | Response) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => json(layoutOf({ bottomRight: ['wifi'] })),
    'GET /api/v1/dashboard/wifi': () => (wifi instanceof Response ? wifi.clone() : json(wifi)),
  })
}

describe('wifi widget', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('draws a 21x21 matrix with a quiet zone of 4 modules and exactly the dark modules', async () => {
    const rows = matrix(21, (x, y) => (x + y) % 3 === 0)
    setup({ configured: true, ssid: 'Gast', qr: { size: 21, rows } })
    renderWithProviders('/')
    const svg = await screen.findByRole('img', { name: 'WLAN Gast' })
    expect(svg).toHaveAttribute('viewBox', '0 0 29 29')
    const d = svg.querySelector('path')?.getAttribute('d') ?? ''
    const squares = [...d.matchAll(/M(\d+) (\d+)h1v1h-1z/g)].map((m) => `${Number(m[1]) - 4},${Number(m[2]) - 4}`)
    const expected = rows.flatMap((row, y) => [...row].flatMap((c, x) => (c === '1' ? [`${x},${y}`] : [])))
    expect(squares).toEqual(expected)
    expect(svg.querySelector('rect')).toHaveAttribute('fill', '#000')
    expect(svg.querySelector('path')).toHaveAttribute('fill', '#fff')
  })

  it('renders nothing when not configured', async () => {
    const api = setup({ configured: false })
    renderWithProviders('/')
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/wifi')).toHaveLength(1))
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('img', { name: /WLAN/ })).not.toBeInTheDocument()
    expect(document.querySelector('.mirror-wifi')).toBeNull()
  })

  it('renders nothing, not even an error text, when the request fails', async () => {
    const api = setup(problem(500, 'Kaputt'))
    renderWithProviders('/')
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/wifi')).toHaveLength(1))
    await new Promise((r) => setTimeout(r, 20))
    expect(document.querySelector('.mirror-wifi')).toBeNull()
    expect(screen.queryByText(/Kaputt|Fehler/)).not.toBeInTheDocument()
  })
})
