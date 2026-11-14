import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const cal = (id: string, name: string) => ({ id, name, icon: { kind: 'none' }, hasUrl: true, urlHint: 'https://x/.../a.ics' })
const section = (id: string) => ({ id, title: 'Familie', type: 'list', calendarIds: ['a'], maxEvents: 8, lookAheadDays: 14, shiftDays: 2, shifts: [] })
const emptyLayout = { topLeft: [], topRight: [], middle: [], bottomLeft: [], bottomRight: [] }

function arrange(over: Record<string, Handler> = {}) {
  mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/display': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/settings/layout': () => json({ ...emptyLayout, topLeft: ['clock', 'weather'], bottomRight: ['wifi'] }),
    'GET /api/v1/dashboard/weather': () =>
      json({ configured: true, stale: false, data: { location: 'Berlin', temperatureC: 12, condition: 'Leichter Regen' } }),
    'GET /api/v1/dashboard/wifi': () => json({ configured: true, ssid: 'Gäste' }),
    'GET /api/v1/dashboard/bring': () => json({ configured: true, stale: false, items: [{ name: 'Milch', iconUrl: '' }, { name: 'Brot', iconUrl: '' }] }),
    'GET /api/v1/settings/calendar': () => json({ refreshMinutes: 15, calendars: [cal('a', 'Familie'), cal('b', 'Arbeit')], sections: [section('s1')], rules: [] }),
    'GET /api/v1/icons': () => json([{ id: 'i1', name: 'Logo', contentType: 'image/png', size: 10 }]),
    ...over,
  })
}

const tile = (name: RegExp) => screen.findByRole('link', { name })

describe('overview tiles of the module registry', () => {
  it('shows the tiles in the fixed order with their links', async () => {
    arrange()
    renderWithProviders('/admin')
    await screen.findByRole('heading', { level: 1, name: 'Übersicht' })
    const names = (await screen.findAllByRole('link'))
      .filter((l) => l.getAttribute('href')?.startsWith('/admin/') && l.getAttribute('href') !== '/admin/logout')
      .map((l) => [l.getAttribute('href'), l.textContent])
    expect(names.map(([href]) => href)).toEqual([
      '/admin/layout',
      '/admin/display',
      '/admin/weather',
      '/admin/wifi',
      '/admin/calendar',
      '/admin/icons',
      '/admin/bring',
      '/admin/account',
    ])
  })

  it('counts the placed modules, plural and singular', async () => {
    arrange()
    renderWithProviders('/admin')
    expect(await within(await tile(/^Anordnung/)).findByText('3 Module angezeigt')).toBeInTheDocument()
  })

  it('says "1 Modul angezeigt" for a single module', async () => {
    arrange({ 'GET /api/v1/settings/layout': () => json({ ...emptyLayout, middle: ['clock'] }) })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Anordnung/)).findByText('1 Modul angezeigt')).toBeInTheDocument()
  })

  it('shows the weather and the SSID from the public state', async () => {
    arrange()
    renderWithProviders('/admin')
    expect(await within(await tile(/^Wetter/)).findByText('Berlin · 12°')).toBeInTheDocument()
    expect(await within(await tile(/^WLAN-QR-Code/)).findByText('„Gäste“')).toBeInTheDocument()
  })

  it('says "Nicht eingerichtet" and "Aus" when nothing is configured', async () => {
    arrange({
      'GET /api/v1/dashboard/weather': () => json({ configured: false, stale: false }),
      'GET /api/v1/dashboard/wifi': () => json({ configured: false }),
    })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Wetter/)).findByText('Nicht eingerichtet')).toBeInTheDocument()
    expect(await within(await tile(/^WLAN-QR-Code/)).findByText('Aus')).toBeInTheDocument()
  })

  it('shows the weather error', async () => {
    arrange({
      'GET /api/v1/dashboard/weather': () => json({ configured: true, stale: false, error: 'API-Key ungültig' }),
    })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Wetter/)).findByText('Fehler: API-Key ungültig')).toBeInTheDocument()
  })

  it('shows "Status nicht verfügbar" per tile when its call fails', async () => {
    arrange({ 'GET /api/v1/settings/layout': () => problem(500, 'Kaputt') })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Anordnung/)).findByText('Status nicht verfügbar')).toBeInTheDocument()
    expect(await within(await tile(/^Wetter/)).findByText('Berlin · 12°')).toBeInTheDocument()
  })

  it('shows the number of shopping list entries, singular and plural', async () => {
    arrange()
    renderWithProviders('/admin')
    expect(await within(await tile(/^Bring!-Einkaufsliste/)).findByText('2 Einträge')).toBeInTheDocument()
  })

  it('says "1 Eintrag" for a single entry', async () => {
    arrange({ 'GET /api/v1/dashboard/bring': () => json({ configured: true, stale: false, items: [{ name: 'Milch', iconUrl: '' }] }) })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Bring!-Einkaufsliste/)).findByText('1 Eintrag')).toBeInTheDocument()
  })

  it('says "Nicht eingerichtet" for an unconfigured shopping list and shows its error', async () => {
    arrange({ 'GET /api/v1/dashboard/bring': () => json({ configured: false, stale: false, items: [] }) })
    const first = renderWithProviders('/admin')
    expect(await within(await tile(/^Bring!-Einkaufsliste/)).findByText('Nicht eingerichtet')).toBeInTheDocument()
    first.unmount()

    arrange({
      'GET /api/v1/dashboard/bring': () => json({ configured: true, stale: false, error: 'Anmeldung bei Bring! fehlgeschlagen.', items: [] }),
    })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Bring!-Einkaufsliste/)).findByText('Fehler: Anmeldung bei Bring! fehlgeschlagen.')).toBeInTheDocument()
  })

  it('shows calendar and section counts and the icon count', async () => {
    arrange()
    renderWithProviders('/admin')
    expect(await within(await tile(/^Kalender/)).findByText('2 Kalender · 1 Bereich')).toBeInTheDocument()
    expect(await within(await tile(/^Symbole/)).findByText('1 eigenes Symbol')).toBeInTheDocument()
  })

  it('says "Keine eigenen Symbole" and uses the plural forms', async () => {
    arrange({
      'GET /api/v1/icons': () => json([]),
      'GET /api/v1/settings/calendar': () =>
        json({ refreshMinutes: 15, calendars: [cal('a', 'Familie')], sections: [section('s1'), section('s2')], rules: [] }),
    })
    renderWithProviders('/admin')
    expect(await within(await tile(/^Symbole/)).findByText('Keine eigenen Symbole')).toBeInTheDocument()
    expect(await within(await tile(/^Kalender/)).findByText('1 Kalender · 2 Bereiche')).toBeInTheDocument()
  })
})
