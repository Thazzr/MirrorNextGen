import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const stored = { email: 'familie@example.org', hasPassword: true, listUuid: '', maxItems: 18, refreshMinutes: 5 }
const lists = [
  { uuid: 'l1', name: 'Zuhause' },
  { uuid: 'l2', name: 'Büro' },
]

function arrange(over: Record<string, Handler> = {}, settings: object = stored) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/bring': () => json(settings),
    'PUT /api/v1/settings/bring': (call) => json({ ...stored, ...(call.body as object), password: undefined, clearPassword: undefined, hasPassword: true }),
    'GET /api/v1/settings/bring/lists': () => json(lists),
    'GET /api/v1/dashboard/bring': () => json({ configured: true, stale: false, items: [{ name: 'Milch', iconUrl: '' }, { name: 'Brot', iconUrl: '' }] }),
    ...over,
  })
}

async function open() {
  const view = renderWithProviders('/admin/bring')
  await screen.findByLabelText('E-Mail')
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const save = () => userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
const putsOf = (api: ReturnType<typeof arrange>) => api.callsTo('PUT /api/v1/settings/bring')
const listBox = () => screen.getByRole('combobox', { name: 'Liste' })

describe('bring page', () => {
  it('loads the values and has one h1 with a back link', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Bring!-Einkaufsliste')
    expect(screen.getByRole('link', { name: '← Übersicht' })).toHaveAttribute('href', '/admin')
    expect(screen.getByLabelText('E-Mail')).toHaveValue('familie@example.org')
    expect(screen.getByLabelText('Passwort')).toHaveAttribute('placeholder', 'Gespeichert – leer lassen, um ihn zu behalten')
    expect(listBox()).toHaveValue('Standardliste')
    expect(screen.getByLabelText('Anzahl Einträge')).toHaveValue('Höchstens 18 Einträge')
    expect(screen.getByLabelText('Aktualisierung (Minuten)')).toHaveValue('5 Minuten')
    expect(bar()).not.toBeInTheDocument()
    expect(screen.queryByText('Aktueller Stand')).not.toBeInTheDocument()
  })

  it('keeps the stored password when only the e-mail changes', async () => {
    const api = arrange()
    await open()
    await userEvent.clear(screen.getByLabelText('E-Mail'))
    await userEvent.type(screen.getByLabelText('E-Mail'), ' neu@example.org ')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Zugang')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ email: 'neu@example.org', listUuid: '', maxItems: 18, refreshMinutes: 5 })
  })

  it('sends the password only when typed, with the session CSRF token', async () => {
    const api = arrange()
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'streng-geheim')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ email: 'familie@example.org', password: 'streng-geheim', listUuid: '', maxItems: 18, refreshMinutes: 5 })
    expect(putsOf(api)[0].csrf).toBe('tok-123')
  })

  it('"Passwort entfernen" sends clearPassword and "Rückgängig" withdraws it', async () => {
    const api = arrange()
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Passwort entfernen' }))
    expect(screen.getByText('Wird beim Speichern entfernt')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Rückgängig' }))
    expect(bar()).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Passwort entfernen' }))
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ email: 'familie@example.org', clearPassword: true, listUuid: '', maxItems: 18, refreshMinutes: 5 })
  })

  it('loads the lists on click, offers "Standardliste" first and saves the chosen uuid', async () => {
    const api = arrange()
    await open()
    expect(api.callsTo('GET /api/v1/settings/bring/lists')).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: 'Listen laden' }))
    await waitFor(() => expect(api.callsTo('GET /api/v1/settings/bring/lists')).toHaveLength(1))
    // Once loaded, the load button is gone.
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Listen laden' })).not.toBeInTheDocument())

    await userEvent.click(listBox())
    const options = (await screen.findAllByRole('option', { hidden: true })).map((o) => o.textContent)
    expect(options).toEqual(['Standardliste', 'Zuhause', 'Büro'])
    await userEvent.click(screen.getByRole('option', { name: 'Büro', hidden: true }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Liste')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ email: 'familie@example.org', listUuid: 'l2', maxItems: 18, refreshMinutes: 5 })
  })

  it('shows the German detail of a 409 under the picker and offers a retry', async () => {
    arrange({ 'GET /api/v1/settings/bring/lists': () => problem(409, 'Conflict', { detail: 'Bitte speichere zuerst E-Mail und Passwort.' }) }, { ...stored, hasPassword: false })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Listen laden' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Bitte speichere zuerst E-Mail und Passwort.')
    expect(alert).not.toHaveTextContent('Conflict')
    expect(screen.getByRole('button', { name: 'Listen laden' })).toBeInTheDocument()
    expect(bar()).not.toBeInTheDocument()
  })

  it('shows the login failure of the lists call', async () => {
    arrange({ 'GET /api/v1/settings/bring/lists': () => problem(400, 'Bad Request', { detail: 'Anmeldung bei Bring! fehlgeschlagen.' }) })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Listen laden' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Anmeldung bei Bring! fehlgeschlagen.')
  })

  it('keeps a stored list selectable before the lists are loaded', async () => {
    arrange({}, { ...stored, listUuid: 'l2' })
    await open()
    expect(listBox()).toHaveValue('Gewählte Liste')
    expect(bar()).not.toBeInTheDocument()
  })

  it('sends the changed numbers and shows the status card with the entry count after saving', async () => {
    const api = arrange()
    await open()
    const max = screen.getByLabelText('Anzahl Einträge')
    await userEvent.clear(max)
    await userEvent.type(max, '12')
    const minutes = screen.getByLabelText('Aktualisierung (Minuten)')
    await userEvent.clear(minutes)
    await userEvent.type(minutes, '15')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Darstellung')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ email: 'familie@example.org', listUuid: '', maxItems: 12, refreshMinutes: 15 })
    const card = await screen.findByRole('region', { name: 'Aktueller Stand' })
    expect(await within(card).findByText('2 Einträge')).toBeInTheDocument()
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
  })

  it('shows "Nicht eingerichtet" and the error in the status card', async () => {
    arrange({ 'GET /api/v1/dashboard/bring': () => json({ configured: false, stale: false, items: [] }) })
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'x')
    await save()
    const card = await screen.findByRole('region', { name: 'Aktueller Stand' })
    expect(await within(card).findByText('Nicht eingerichtet')).toBeInTheDocument()
  })

  it('shows the error of the dashboard state in the status card', async () => {
    arrange({ 'GET /api/v1/dashboard/bring': () => json({ configured: true, stale: false, error: 'Anmeldung bei Bring! fehlgeschlagen.', items: [] }) })
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'x')
    await save()
    const card = await screen.findByRole('region', { name: 'Aktueller Stand' })
    expect(await within(card).findByText('Fehler: Anmeldung bei Bring! fehlgeschlagen.')).toBeInTheDocument()
  })

  it('shows server errors on the fields and keeps what was typed', async () => {
    arrange({
      'PUT /api/v1/settings/bring': () =>
        problem(400, 'Ungültig', { errors: { email: 'Höchstens 200 Zeichen.', password: 'Höchstens 200 Zeichen.', maxItems: 'Erlaubt sind 1 bis 18 Einträge.' } }),
    })
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'kurz')
    await save()
    expect(await screen.findByText('Erlaubt sind 1 bis 18 Einträge.')).toBeInTheDocument()
    expect(screen.getAllByText('Höchstens 200 Zeichen.')).toHaveLength(2)
    expect(screen.getByLabelText('Passwort')).toHaveValue('kurz')
    expect(bar()).toBeInTheDocument()
  })

  it('discards the changes', async () => {
    arrange()
    await open()
    await userEvent.clear(screen.getByLabelText('E-Mail'))
    await userEvent.type(screen.getByLabelText('E-Mail'), 'x@y.de')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Verwerfen' }))
    expect(screen.getByLabelText('E-Mail')).toHaveValue('familie@example.org')
    expect(bar()).not.toBeInTheDocument()
  })

  it('asks before leaving with unsaved changes', async () => {
    arrange()
    const { router } = await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'x')
    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    expect(router.state.location.pathname).toBe('/admin/bring')
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })
})
