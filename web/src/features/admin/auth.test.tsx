import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { anonymousSession, authedSession, info, json, mockApi, noContent, problem } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const setupRequired = { setupRequired: true, authenticated: false }
const settings = { rotation: 90, output: '', testPattern: true }

describe('setup page', () => {
  function arrange(extra = {}) {
    return mockApi({
      'GET /api/v1/auth/session': () => json(setupRequired),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
      ...extra,
    })
  }

  it('has exactly one h1', async () => {
    arrange()
    renderWithProviders('/admin/setup')
    await screen.findByRole('heading', { level: 1, name: 'Spiegel einrichten' })
    expectSinglePageTitle()
  })

  it('validates length and repetition on the client and sends nothing', async () => {
    const api = arrange()
    renderWithProviders('/admin/setup')
    await screen.findByRole('heading', { level: 1 })

    await userEvent.type(screen.getByLabelText('Passwort', { selector: 'input' }), 'kurz')
    await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    expect(await screen.findByText('Das Passwort muss mindestens 10 Zeichen lang sein.')).toBeInTheDocument()

    await userEvent.clear(screen.getByLabelText('Passwort', { selector: 'input' }))
    await userEvent.type(screen.getByLabelText('Passwort', { selector: 'input' }), 'lang-genug-1')
    await userEvent.type(screen.getByLabelText('Passwort wiederholen'), 'anders-genug')
    await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    expect(await screen.findByText('Die Passwörter stimmen nicht überein.')).toBeInTheDocument()
    expect(api.callsTo('POST /api/v1/auth/setup')).toHaveLength(0)
  })

  describe('password length limits', () => {
    const tooShort = 'Das Passwort muss mindestens 10 Zeichen lang sein.'
    const tooLong = 'Das Passwort darf höchstens 128 Zeichen lang sein.'

    async function submitLength(length: number) {
      const password = 'a'.repeat(length)
      renderWithProviders('/admin/setup')
      await screen.findByRole('heading', { level: 1 })
      await userEvent.click(screen.getByLabelText('Passwort', { selector: 'input' }))
      await userEvent.paste(password)
      await userEvent.click(screen.getByLabelText('Passwort wiederholen'))
      await userEvent.paste(password)
      await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    }

    it.each([
      [9, tooShort],
      [129, tooLong],
    ])('rejects %i characters on the client', async (length, message) => {
      const api = arrange({ 'POST /api/v1/auth/setup': () => json(authedSession) })
      await submitLength(length)
      expect(await screen.findByText(message)).toBeInTheDocument()
      expect(api.callsTo('POST /api/v1/auth/setup')).toHaveLength(0)
    })

    it.each([10, 128])('accepts %i characters', async (length) => {
      const api = arrange({ 'POST /api/v1/auth/setup': () => json(authedSession) })
      await submitLength(length)
      await waitFor(() => expect(api.callsTo('POST /api/v1/auth/setup')).toHaveLength(1))
    })
  })

  it('shows the server message on the password field', async () => {
    arrange({
      'POST /api/v1/auth/setup': () => problem(400, 'Ungültige Eingabe', { errors: { password: 'Zu einfach.' } }),
    })
    renderWithProviders('/admin/setup')
    await screen.findByRole('heading', { level: 1 })
    await userEvent.type(screen.getByLabelText('Passwort', { selector: 'input' }), 'lang-genug-1')
    await userEvent.type(screen.getByLabelText('Passwort wiederholen'), 'lang-genug-1')
    await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    expect(await screen.findByText('Zu einfach.')).toBeInTheDocument()
  })

  it('goes to the overview after a successful setup', async () => {
    const api = arrange({ 'POST /api/v1/auth/setup': () => json(authedSession) })
    const { router } = renderWithProviders('/admin/setup')
    await screen.findByRole('heading', { level: 1 })
    await userEvent.type(screen.getByLabelText('Passwort', { selector: 'input' }), 'lang-genug-1')
    await userEvent.type(screen.getByLabelText('Passwort wiederholen'), 'lang-genug-1')
    await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin')
    expect(api.callsTo('POST /api/v1/auth/setup')[0].body).toEqual({ password: 'lang-genug-1' })
  })

  it('goes to the login with a notice on 409', async () => {
    arrange({ 'POST /api/v1/auth/setup': () => problem(409, 'Konflikt') })
    const { router } = renderWithProviders('/admin/setup')
    await screen.findByRole('heading', { level: 1 })
    await userEvent.type(screen.getByLabelText('Passwort', { selector: 'input' }), 'lang-genug-1')
    await userEvent.type(screen.getByLabelText('Passwort wiederholen'), 'lang-genug-1')
    await userEvent.click(screen.getByRole('button', { name: 'Zugang anlegen' }))
    expect(await screen.findByText('Der Zugang ist bereits eingerichtet.')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/login')
  })
})

describe('login page', () => {
  function arrange(extra = {}) {
    return mockApi({
      'GET /api/v1/auth/session': () => json(anonymousSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
      ...extra,
    })
  }

  async function submit(password = 'falsch-falsch') {
    await screen.findByRole('heading', { level: 1, name: 'Anmelden' })
    expectSinglePageTitle()
    await userEvent.type(screen.getByLabelText('Passwort'), password)
    await userEvent.click(screen.getByRole('button', { name: 'Anmelden' }))
  }

  it('shows "Das Passwort stimmt nicht." on 401', async () => {
    arrange({ 'POST /api/v1/auth/login': () => problem(401, 'Nicht angemeldet') })
    renderWithProviders('/admin/login')
    await submit()
    expect(await screen.findByText('Das Passwort stimmt nicht.')).toBeInTheDocument()
  })

  it('tells how long to wait on 429 from Retry-After', async () => {
    arrange({
      'POST /api/v1/auth/login': () => problem(429, 'Zu viele Anfragen', {}, { 'Retry-After': '900' }),
    })
    renderWithProviders('/admin/login')
    await submit()
    expect(await screen.findByText('Zu viele Fehlversuche. Versuch es in 15 Minuten noch einmal.')).toBeInTheDocument()
  })

  it('says "in einer Minute" for a wait up to one minute', async () => {
    arrange({ 'POST /api/v1/auth/login': () => problem(429, 'Zu viele Anfragen', {}, { 'Retry-After': '20' }) })
    renderWithProviders('/admin/login')
    await submit()
    expect(await screen.findByText('Zu viele Fehlversuche. Versuch es in einer Minute noch einmal.')).toBeInTheDocument()
  })

  it.each(['90', '61'])('rounds a Retry-After of %s seconds up to 2 minutes', async (seconds) => {
    arrange({ 'POST /api/v1/auth/login': () => problem(429, 'Zu viele Anfragen', {}, { 'Retry-After': seconds }) })
    renderWithProviders('/admin/login')
    await submit()
    expect(await screen.findByText('Zu viele Fehlversuche. Versuch es in 2 Minuten noch einmal.')).toBeInTheDocument()
  })

  it('goes to the setup on 409', async () => {
    arrange({ 'POST /api/v1/auth/login': () => problem(409, 'Konflikt') })
    const { router } = renderWithProviders('/admin/login')
    await submit()
    expect(await screen.findByRole('heading', { level: 1, name: 'Spiegel einrichten' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/setup')
  })

  it('goes to the overview after signing in', async () => {
    arrange({ 'POST /api/v1/auth/login': () => json(authedSession) })
    renderWithProviders('/admin/login')
    await submit('richtig-richtig')
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument()
  })
})

describe('CSRF header', () => {
  it('is sent with logout, which then lands on the login', async () => {
    const api = mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
      'POST /api/v1/auth/logout': () => noContent(),
    })
    const { router } = renderWithProviders('/admin')
    await userEvent.click(await screen.findByRole('link', { name: 'Abmelden' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Anmelden' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/login')
    expect(api.callsTo('POST /api/v1/auth/logout')).toHaveLength(1)
    expect(api.callsTo('POST /api/v1/auth/logout')[0].csrf).toBe('tok-123')
  })

  it('is not sent with GET requests', async () => {
    const api = mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/settings/display': () => json(settings),
      'GET /api/v1/info': () => json(info),
    })
    renderWithProviders('/admin')
    await screen.findByRole('heading', { level: 1, name: 'Übersicht' })
    expect(api.calls.every((c) => c.method !== 'GET' || c.csrf === null)).toBe(true)
  })
})
