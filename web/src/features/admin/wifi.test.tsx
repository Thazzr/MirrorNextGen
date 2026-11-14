import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const stored = { ssid: 'Gäste', security: 'WPA', hasPassword: true }

function arrange(put?: Handler, settings: object = stored) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/wifi': () => json(settings),
    ...(put ? { 'PUT /api/v1/settings/wifi': put } : {}),
  })
}

async function open() {
  const view = renderWithProviders('/admin/wifi')
  await screen.findByLabelText('Netzwerkname (SSID)')
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const save = () => userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
const putsOf = (api: ReturnType<typeof arrange>) => api.callsTo('PUT /api/v1/settings/wifi')

describe('wifi page', () => {
  it('loads the values and has one h1', async () => {
    arrange()
    await open()
    expectSinglePageTitle('WLAN-QR-Code')
    expect(screen.getByLabelText('Netzwerkname (SSID)')).toHaveValue('Gäste')
    expect(screen.getByRole('radio', { name: 'WPA/WPA2' })).toBeChecked()
    expect(screen.getByLabelText('Passwort')).toHaveAttribute('placeholder', 'Gespeichert – leer lassen, um ihn zu behalten')
    expect(screen.getByText('Gäste scannen den QR-Code unten am Spiegel und verbinden sich ohne Tippen.')).toBeInTheDocument()
    expect(bar()).not.toBeInTheDocument()
  })

  it('"Offen" hides the password and sends security nopass without it', async () => {
    const api = arrange((call) => json({ ...(call.body as object), hasPassword: true }))
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'wirdignoriert')
    await userEvent.click(screen.getByRole('radio', { name: 'Offen' }))
    expect(screen.queryByLabelText('Passwort')).not.toBeInTheDocument()
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Sicherheit')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ ssid: 'Gäste', security: 'nopass' })
  })

  it('sends the password only when typed', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object), password: undefined }))
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'streng-geheim')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Passwort')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ ssid: 'Gäste', security: 'WPA', password: 'streng-geheim' })
  })

  it('keeps the stored password when only the SSID changes', async () => {
    const api = arrange(() => json({ ...stored, ssid: 'Neu' }))
    await open()
    await userEvent.clear(screen.getByLabelText('Netzwerkname (SSID)'))
    await userEvent.type(screen.getByLabelText('Netzwerkname (SSID)'), 'Neu')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Netzwerk')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ ssid: 'Neu', security: 'WPA' })
  })

  it('"Passwort entfernen" sends clearPassword and "Rückgängig" withdraws it', async () => {
    const api = arrange(() => json({ ...stored, hasPassword: false }))
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Passwort entfernen' }))
    expect(screen.getByText('Wird beim Speichern entfernt')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Rückgängig' }))
    expect(bar()).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Passwort entfernen' }))
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({ ssid: 'Gäste', security: 'WPA', clearPassword: true })
  })

  it('shows server errors on ssid and password', async () => {
    arrange(() =>
      problem(400, 'Ungültig', { errors: { ssid: 'Zu lang.', password: 'Mindestens 8 Zeichen.' } }),
    )
    await open()
    await userEvent.type(screen.getByLabelText('Passwort'), 'kurz')
    await save()
    expect(await screen.findByText('Zu lang.')).toBeInTheDocument()
    expect(screen.getByText('Mindestens 8 Zeichen.')).toBeInTheDocument()
    expect(screen.getByLabelText('Passwort')).toHaveValue('kurz')
    expect(bar()).toBeInTheDocument()
  })
})
