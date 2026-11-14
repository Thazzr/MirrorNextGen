import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

function arrange(display: () => Response) {
  mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/settings/display': display,
    'GET /api/v1/info': () => json(info),
  })
}

describe('overview', () => {
  it('shows each area as one tile that is a single link, with the display status', async () => {
    arrange(() => json({ rotation: 90, output: '', testPattern: true, schedule: { enabled: false, offTime: '23:00', onTime: '06:00' } }))
    renderWithProviders('/admin')
    await screen.findByRole('heading', { level: 1, name: 'Übersicht' })
    expectSinglePageTitle()

    const display = await screen.findByRole('link', { name: /^Anzeige/ })
    expect(display).toHaveAttribute('href', '/admin/display')
    expect(await within(display).findByText('Drehung 90° · Testbild an')).toBeInTheDocument()
    expect(within(display).queryAllByRole('link')).toHaveLength(0)

    const account = screen.getByRole('link', { name: /^Konto/ })
    expect(account).toHaveAttribute('href', '/admin/account')
    expect(within(account).getByText('Passwort ändern')).toBeInTheDocument()
    expect(within(account).queryAllByRole('link')).toHaveLength(0)
  })

  it('says the test pattern is off', async () => {
    arrange(() => json({ rotation: 0, output: '', testPattern: false, schedule: { enabled: false, offTime: '23:00', onTime: '06:00' } }))
    renderWithProviders('/admin')
    expect(await screen.findByText('Drehung 0° · Testbild aus')).toBeInTheDocument()
  })

  it('adds the night schedule to the display tile only while it is enabled', async () => {
    const schedule = (enabled: boolean) => ({ enabled, offTime: '22:30', onTime: '05:45' })
    arrange(() => json({ rotation: 0, output: '', testPattern: false, schedule: schedule(true) }))
    const view = renderWithProviders('/admin')
    expect(await screen.findByText('Drehung 0° · Testbild aus · Nachtruhe 22:30–05:45')).toBeInTheDocument()
    view.unmount()

    arrange(() => json({ rotation: 0, output: '', testPattern: false, schedule: schedule(false) }))
    renderWithProviders('/admin')
    expect(await screen.findByText('Drehung 0° · Testbild aus')).toBeInTheDocument()
    expect(screen.queryByText(/Nachtruhe/)).not.toBeInTheDocument()
  })

  it('shows "Status nicht verfügbar" when the settings fail', async () => {
    arrange(() => problem(500, 'Kaputt'))
    renderWithProviders('/admin')
    expect(await screen.findByText('Status nicht verfügbar')).toBeInTheDocument()
  })
})
