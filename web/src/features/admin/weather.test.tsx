import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const stored = { location: 'Berlin', hasApiKey: true, showIcons: true, refreshMinutes: 30 }

function arrange(put?: Handler, settings = stored, dashboard?: Handler) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/weather': () => json(settings),
    'GET /api/v1/dashboard/weather': dashboard ?? (() => json({ configured: false, stale: false })),
    ...(put ? { 'PUT /api/v1/settings/weather': put } : {}),
  })
}

async function open() {
  const view = renderWithProviders('/admin/weather')
  await screen.findByRole('textbox', { name: 'Ort' })
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const save = () => userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))

describe('weather page', () => {
  it('loads the values, has one h1 and the three sections', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Wetter')
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual([
      'Ort',
      'OpenWeather',
      'Darstellung',
    ])
    expect(screen.getByRole('textbox', { name: 'Ort' })).toHaveValue('Berlin')
    expect(screen.getByLabelText('API-Key')).toHaveAttribute('placeholder', 'Gespeichert – leer lassen zum Behalten')
    expect(screen.getByRole('switch', { name: 'Wettersymbole anzeigen' })).toBeChecked()
    expect(screen.getByLabelText('Aktualisierung')).toHaveValue('30 Minuten')
    expect(bar()).not.toBeInTheDocument()
  })

  it('counts a typed key as a change and sends it only then', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object), apiKey: undefined }))
    await open()
    await userEvent.type(screen.getByLabelText('API-Key'), 'geheim')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: API-Key')
    await save()
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/weather')).toHaveLength(1))
    expect(api.callsTo('PUT /api/v1/settings/weather')[0].body).toEqual({
      location: 'Berlin',
      apiKey: 'geheim',
      showIcons: true,
      refreshMinutes: 30,
    })
  })

  it('omits apiKey and clearApiKey when only the place changed', async () => {
    const api = arrange(() => json({ ...stored, location: 'Köln' }))
    await open()
    await userEvent.clear(screen.getByRole('textbox', { name: 'Ort' }))
    await userEvent.type(screen.getByRole('textbox', { name: 'Ort' }), 'Köln')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Ort')
    await save()
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/weather')).toHaveLength(1))
    const body = api.callsTo('PUT /api/v1/settings/weather')[0].body as Record<string, unknown>
    expect(body).toEqual({ location: 'Köln', showIcons: true, refreshMinutes: 30 })
    expect('apiKey' in body).toBe(false)
    expect('clearApiKey' in body).toBe(false)
  })

  it('"Key entfernen" sends clearApiKey, "Rückgängig" withdraws it', async () => {
    const api = arrange(() => json({ ...stored, hasApiKey: false }))
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Key entfernen' }))
    expect(screen.getByText('Wird beim Speichern entfernt')).toBeInTheDocument()
    expect(bar()).toHaveTextContent('API-Key')

    await userEvent.click(screen.getByRole('button', { name: 'Rückgängig' }))
    expect(bar()).not.toBeInTheDocument()
    expect(screen.getByLabelText('API-Key')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Key entfernen' }))
    await save()
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/weather')).toHaveLength(1))
    expect(api.callsTo('PUT /api/v1/settings/weather')[0].body).toEqual({
      location: 'Berlin',
      clearApiKey: true,
      showIcons: true,
      refreshMinutes: 30,
    })
  })

  it('shows server errors on the fields and keeps the input', async () => {
    arrange(() =>
      problem(400, 'Ungültig', {
        errors: { refreshMinutes: 'Mindestens 10 Minuten.', location: 'Ort nicht gefunden.' },
      }),
    )
    await open()
    await userEvent.clear(screen.getByLabelText('Aktualisierung'))
    await userEvent.type(screen.getByLabelText('Aktualisierung'), '5')
    await save()
    expect(await screen.findByText('Mindestens 10 Minuten.')).toBeInTheDocument()
    expect(screen.getByText('Ort nicht gefunden.')).toBeInTheDocument()
    expect(bar()).toBeInTheDocument()
    expect(screen.getByLabelText('Aktualisierung')).toHaveValue('5 Minuten')
  })

  it('shows the current dashboard weather after saving', async () => {
    let configured = false
    const api = arrange(
      () => json({ ...stored, showIcons: false }),
      stored,
      () =>
        json(
          configured
            ? { configured: true, stale: false, data: { location: 'Berlin', temperatureC: 12, condition: 'Leichter Regen' } }
            : { configured: false, stale: false },
        ),
    )
    await open()
    expect(screen.queryByText('Aktueller Stand')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('switch', { name: 'Wettersymbole anzeigen' }))
    configured = true
    await save()
    expect(await screen.findByRole('heading', { level: 2, name: 'Aktueller Stand' })).toBeInTheDocument()
    expect(await screen.findByText('Berlin · 12° · Leichter Regen')).toBeInTheDocument()
    expect(api.callsTo('GET /api/v1/dashboard/weather').length).toBeGreaterThanOrEqual(1)
    expectSinglePageTitle('Wetter')
  })
})
