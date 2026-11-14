import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { anonymousSession, authedSession, info, json, mockApi, problem } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

afterEach(() => vi.unstubAllGlobals())

const settings = { rotation: 0, output: '', testPattern: false }

describe('admin route guard', () => {
  it('sends a first start to the setup page', async () => {
    mockApi({ 'GET /api/v1/auth/session': () => json({ setupRequired: true, authenticated: false }) })
    const { router } = renderWithProviders('/admin')
    expect(await screen.findByRole('heading', { level: 1, name: 'Spiegel einrichten' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/setup')
    expectSinglePageTitle()
  })

  it('sends an anonymous visitor to the login and back to the requested page after signing in', async () => {
    const api = mockApi({
      'GET /api/v1/auth/session': () => json(anonymousSession),
      'POST /api/v1/auth/login': () => json(authedSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
    })
    const { router } = renderWithProviders('/admin/display')
    expect(await screen.findByRole('heading', { level: 1, name: 'Anmelden' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/login')

    await userEvent.type(screen.getByLabelText('Passwort'), 'geheim-geheim')
    await userEvent.click(screen.getByRole('button', { name: 'Anmelden' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Anzeige' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/display')
    expect(api.callsTo('POST /api/v1/auth/login')[0].body).toEqual({ password: 'geheim-geheim' })
  })

  it('sends a signed-in user from the login to the overview', async () => {
    mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
    })
    const { router } = renderWithProviders('/admin/login')
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin')
  })

  it('sends the setup page to the login once setup is done', async () => {
    mockApi({ 'GET /api/v1/auth/session': () => json(anonymousSession) })
    const { router } = renderWithProviders('/admin/setup')
    expect(await screen.findByRole('heading', { level: 1, name: 'Anmelden' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/login')
  })

  it('sends the user to the login when a protected call answers 401', async () => {
    let signedIn = true
    mockApi({
      'GET /api/v1/auth/session': () => json(signedIn ? authedSession : anonymousSession),
      'GET /api/v1/settings/display': () => {
        signedIn = false
        return problem(401, 'Nicht angemeldet')
      },
      'GET /api/v1/info': () => json(info),
    })
    const { router } = renderWithProviders('/admin/display')
    expect(await screen.findByRole('heading', { level: 1, name: 'Anmelden' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/login')
  })

  it('shows the version in the footer of signed-in pages', async () => {
    mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
    })
    renderWithProviders('/admin')
    expect(await screen.findByText('Version 1.2.3 (abc1234)')).toBeInTheDocument()
    expect(screen.getByText('Spiegel-Verwaltung').tagName).not.toMatch(/^H\d$/)
  })
})
