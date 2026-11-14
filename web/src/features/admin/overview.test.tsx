import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

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
    arrange(() => json({ rotation: 90, output: '', testPattern: true }))
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
    arrange(() => json({ rotation: 0, output: '', testPattern: false }))
    renderWithProviders('/admin')
    expect(await screen.findByText('Drehung 0° · Testbild aus')).toBeInTheDocument()
  })

  it('shows "Status nicht verfügbar" when the settings fail', async () => {
    arrange(() => problem(500, 'Kaputt'))
    renderWithProviders('/admin')
    expect(await screen.findByText('Status nicht verfügbar')).toBeInTheDocument()
  })
})
