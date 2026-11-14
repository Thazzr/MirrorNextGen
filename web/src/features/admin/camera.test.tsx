import { screen, waitFor, within } from '@testing-library/react'
import { QueryObserver } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'
import { dashboardCameraQueryKey, fetchDashboardCamera } from '../modules/camera/api.ts'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const empty = { enabled: false, title: 'Kamera', hasStreamUrl: false, streamUrlHint: '', brightness: 100, contrast: 100, saturation: 100, grayscale: false }
const stored = { ...empty, enabled: true, title: 'Haustür', hasStreamUrl: true, streamUrlHint: 'rtsp://192.168.1.20', brightness: 120 }
const placeholder = 'rtsp://benutzer:passwort@192.168.1.20:554/stream1'

function arrange(put?: Handler, settings: object = stored) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/camera': () => json(settings),
    ...(put ? { 'PUT /api/v1/settings/camera': put } : {}),
  })
}

async function open() {
  const view = renderWithProviders('/admin/camera')
  await screen.findByLabelText('Stream-Adresse')
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const save = () => userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
const putsOf = (api: ReturnType<typeof arrange>) => api.callsTo('PUT /api/v1/settings/camera')
const previewFilter = () => screen.getByTestId('camera-preview').style.filter

async function nudge(name: string, key: '{ArrowRight}' | '{ArrowLeft}', times: number) {
  screen.getByRole('slider', { name }).focus()
  for (let i = 0; i < times; i++) await userEvent.keyboard(key)
}

describe('camera page', () => {
  it('loads the values, shows the stored address only as a hint and has one h1', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Kamera')
    expect(screen.getByRole('switch', { name: 'Kamera anzeigen' })).toBeChecked()
    expect(screen.getByLabelText('Titel')).toHaveValue('Haustür')
    const field = screen.getByLabelText('Stream-Adresse')
    expect(field).toHaveValue('')
    expect(field).toHaveAttribute('placeholder', 'Gespeichert – leer lassen, um ihn zu behalten')
    expect(screen.getByText(/Gespeichert für rtsp:\/\/192\.168\.1\.20\. Nur RTSP und RTSPS\. Die Adresse bleibt auf dem Spiegel\./)).toBeInTheDocument()
    expect(screen.getByRole('slider', { name: 'Helligkeit' })).toHaveAttribute('aria-valuenow', '120')
    expect(screen.getByRole('slider', { name: 'Kontrast' })).toHaveAttribute('aria-valuenow', '100')
    expect(screen.getByRole('slider', { name: 'Sättigung' })).toHaveAttribute('aria-valuenow', '100')
    expect(bar()).not.toBeInTheDocument()
  })

  it('shows the example address and the hint while none is stored', async () => {
    arrange(undefined, empty)
    await open()
    expect(screen.getByLabelText('Stream-Adresse')).toHaveAttribute('placeholder', placeholder)
    expect(screen.getByText('Nur RTSP und RTSPS. Die Adresse bleibt auf dem Spiegel.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Stream-Adresse entfernen' })).not.toBeInTheDocument()
  })

  it('previews the filters on a static placeholder and follows the sliders and the switch', async () => {
    arrange()
    await open()
    expect(previewFilter()).toBe('brightness(120%) contrast(100%) saturate(100%)')
    await nudge('Kontrast', '{ArrowRight}', 5)
    await nudge('Sättigung', '{ArrowLeft}', 3)
    expect(previewFilter()).toBe('brightness(120%) contrast(105%) saturate(97%)')
    await userEvent.click(screen.getByRole('switch', { name: 'Schwarzweiß' }))
    expect(previewFilter()).toBe('brightness(120%) contrast(105%) saturate(97%) grayscale(1)')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Bild')
    // The admin shows a gradient, never the video.
    expect(document.querySelector('video, video-rtc')).toBeNull()
  })

  it('sends the typed address only when typed, plus the sliders', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object), streamUrl: undefined }))
    await open()
    await nudge('Helligkeit', '{ArrowRight}', 10)
    await userEvent.type(screen.getByLabelText('Stream-Adresse'), ' rtsp://u:p@cam/s1 ')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect(putsOf(api)[0].body).toEqual({
      enabled: true,
      title: 'Haustür',
      streamUrl: 'rtsp://u:p@cam/s1',
      brightness: 130,
      contrast: 100,
      saturation: 100,
      grayscale: false,
    })
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
  })

  it('does not send the stored address when only the picture changed', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object) }))
    await open()
    await userEvent.click(screen.getByRole('switch', { name: 'Kamera anzeigen' }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Anzeige')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    const body = putsOf(api)[0].body as Record<string, unknown>
    expect(body.enabled).toBe(false)
    expect(body).not.toHaveProperty('streamUrl')
    expect(body).not.toHaveProperty('clearStreamUrl')
  })

  it('removes the stored address with clearStreamUrl and can take it back', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object), hasStreamUrl: false, streamUrlHint: '' }))
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Stream-Adresse entfernen' }))
    expect(screen.getByText('Wird beim Speichern entfernt')).toBeInTheDocument()
    expect(bar()).toHaveTextContent('Stream-Adresse')
    await userEvent.click(screen.getByRole('button', { name: 'Rückgängig' }))
    expect(bar()).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Stream-Adresse entfernen' }))
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    const body = putsOf(api)[0].body as Record<string, unknown>
    expect(body.clearStreamUrl).toBe(true)
    expect(body).not.toHaveProperty('streamUrl')
  })

  it('does not send a typed address when the stored one is removed afterwards', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object), hasStreamUrl: false, streamUrlHint: '' }))
    await open()
    await userEvent.type(screen.getByLabelText('Stream-Adresse'), 'rtsp://u:p@cam/typed')
    await userEvent.click(screen.getByRole('button', { name: 'Stream-Adresse entfernen' }))
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    const body = putsOf(api)[0].body as Record<string, unknown>
    expect(body.clearStreamUrl).toBe(true)
    expect(body).not.toHaveProperty('streamUrl')
  })

  it('trims the title on save', async () => {
    const api = arrange((call) => json({ ...stored, ...(call.body as object) }))
    await open()
    const title = screen.getByLabelText('Titel')
    await userEvent.clear(title)
    await userEvent.type(title, '  Flur  ')
    await save()
    await waitFor(() => expect(putsOf(api)).toHaveLength(1))
    expect((putsOf(api)[0].body as { title: string }).title).toBe('Flur')
  })

  it('does not count a title that differs only by surrounding spaces as a change', async () => {
    arrange()
    await open()
    await userEvent.type(screen.getByLabelText('Titel'), '  ')
    expect(bar()).not.toBeInTheDocument()
  })

  it('names "Anzeige" when only the title changed', async () => {
    arrange()
    await open()
    await userEvent.type(screen.getByLabelText('Titel'), 'x')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Anzeige')
    expect(bar()).not.toHaveTextContent('Bild')
  })

  it('names "Bild" when only the contrast, or only the saturation, changed', async () => {
    arrange()
    await open()
    await nudge('Kontrast', '{ArrowRight}', 2)
    expect(bar()).toHaveTextContent(/^Ungespeicherte Änderungen: Bild/)
    await nudge('Kontrast', '{ArrowLeft}', 2)
    expect(bar()).not.toBeInTheDocument()
    await nudge('Sättigung', '{ArrowRight}', 2)
    expect(bar()).toHaveTextContent(/^Ungespeicherte Änderungen: Bild/)
  })

  it('shows server errors for contrast and saturation at their sliders and not in the bar', async () => {
    arrange(() => problem(400, 'Bad Request', { errors: { contrast: 'Kontrast: Wert zwischen 0 und 200.', saturation: 'Sättigung: Wert zwischen 0 und 200.' } }))
    await open()
    await nudge('Kontrast', '{ArrowRight}', 1)
    await save()
    const contrast = await screen.findByText('Kontrast: Wert zwischen 0 und 200.')
    const saturation = await screen.findByText('Sättigung: Wert zwischen 0 und 200.')
    expect(contrast.closest('.mantine-InputWrapper-root')).toContainElement(screen.getByRole('slider', { name: 'Kontrast' }))
    expect(saturation.closest('.mantine-InputWrapper-root')).toContainElement(screen.getByRole('slider', { name: 'Sättigung' }))
    // A field error is not repeated as a general error text.
    expect(within(bar()!).queryByText(/Wert zwischen/)).not.toBeInTheDocument()
    expect(within(bar()!).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('fetches the dashboard camera again after saving', async () => {
    const dash = { configured: true, title: 'Haustür', brightness: 120, contrast: 100, saturation: 100, grayscale: false }
    const api = mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/info': () => json(info),
      'GET /api/v1/settings/camera': () => json(stored),
      'GET /api/v1/dashboard/camera': () => json(dash),
      'PUT /api/v1/settings/camera': (call) => json({ ...stored, ...(call.body as object) }),
    })
    const { queryClient } = await open()
    // Stands in for a dashboard in the same browser that shows the camera.
    const observer = new QueryObserver(queryClient, { queryKey: dashboardCameraQueryKey, queryFn: fetchDashboardCamera })
    const unsubscribe = observer.subscribe(() => {})
    await waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1))
    await userEvent.click(screen.getByRole('switch', { name: 'Schwarzweiß' }))
    await save()
    await waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(2))
    unsubscribe()
  })

  it('keeps the inputs and names the field when the server refuses the address', async () => {
    const api = arrange(() =>
      problem(400, 'Bad Request', { errors: { streamUrl: 'Die Adresse muss mit rtsp:// oder rtsps:// beginnen und einen Rechner nennen.' } }),
    )
    await open()
    await userEvent.type(screen.getByLabelText('Stream-Adresse'), 'http://cam/video')
    await save()
    expect(await screen.findByText(/muss mit rtsp:\/\/ oder rtsps:\/\/ beginnen/)).toBeInTheDocument()
    expect(putsOf(api)).toHaveLength(1)
    expect(screen.getByLabelText('Stream-Adresse')).toHaveValue('http://cam/video')
    expect(bar()).toBeInTheDocument()
  })

  it('discards the changes', async () => {
    arrange()
    await open()
    await nudge('Helligkeit', '{ArrowLeft}', 4)
    await userEvent.type(screen.getByLabelText('Titel'), 'x')
    expect(bar()).toBeInTheDocument()
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Verwerfen' }))
    expect(bar()).not.toBeInTheDocument()
    expect(screen.getByLabelText('Titel')).toHaveValue('Haustür')
    expect(screen.getByRole('slider', { name: 'Helligkeit' })).toHaveAttribute('aria-valuenow', '120')
    expect(previewFilter()).toBe('brightness(120%) contrast(100%) saturate(100%)')
  })

  it('asks before leaving with unsaved changes', async () => {
    arrange()
    await open()
    await userEvent.click(screen.getByRole('switch', { name: 'Schwarzweiß' }))
    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText('Titel')).toBeInTheDocument()
  })

  it('shows the load error instead of the form', async () => {
    mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/info': () => json(info),
      'GET /api/v1/settings/camera': () => problem(500, 'Kaputt'),
    })
    renderWithProviders('/admin/camera')
    expect(await screen.findByText('Die Einstellungen konnten nicht geladen werden.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Stream-Adresse')).not.toBeInTheDocument()
  })
})
