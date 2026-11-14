import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

afterEach(() => vi.unstubAllGlobals())

const stored = { rotation: 0, output: '', testPattern: false }

function arrange(put?: Handler, settings: typeof stored = stored) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/settings/display': () => json(settings),
    'GET /api/v1/info': () => json(info),
    ...(put ? { 'PUT /api/v1/settings/display': put } : {}),
  })
}

async function open() {
  const view = renderWithProviders('/admin/display')
  await screen.findByRole('radio', { name: '0°' })
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })

describe('display page', () => {
  it('has one h1, the three h2 sections and no bar at first', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Anzeige')
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual(['Drehung', 'Monitor-Ausgang', 'Testbild'])
    expect(bar()).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Speichern' })).not.toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '0°' })).toBeChecked()
  })

  it('shows a skeleton while loading and an alert when loading fails', async () => {
    mockApi({
      'GET /api/v1/auth/session': () => json(authedSession),
      'GET /api/v1/settings/display': () => problem(500, 'Kaputt'),
      'GET /api/v1/info': () => json(info),
    })
    renderWithProviders('/admin/display')
    expect(await screen.findByText('Die Einstellungen konnten nicht geladen werden.')).toBeInTheDocument()
    expectSinglePageTitle('Anzeige')
  })

  it('shows the bar naming "Drehung" after changing the rotation', async () => {
    arrange()
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Drehung')
  })

  it('moves between the rotations with the arrow keys and shows the bar', async () => {
    arrange()
    await open()
    screen.getByRole('radio', { name: '0°' }).focus()
    await userEvent.keyboard('{ArrowRight}')
    expect(screen.getByRole('radio', { name: '90°' })).toBeChecked()
    expect(screen.getByRole('radio', { name: '90°' })).toHaveFocus()
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Drehung')
    await userEvent.keyboard('{ArrowDown}{ArrowLeft}{ArrowUp}')
    expect(screen.getByRole('radio', { name: '0°' })).toBeChecked()
    expect(bar()).not.toBeInTheDocument()
  })

  it('explains the rotation without claiming a direction', async () => {
    arrange()
    await open()
    expect(
      screen.getByText(
        'Dreht das Bild am Monitor. Ein hochkant montierter Monitor braucht meist 90° oder 270° – welche Richtung passt, zeigt das Testbild.',
      ),
    ).toBeInTheDocument()
  })

  it('names both areas when two are changed', async () => {
    arrange()
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '270°' }))
    await userEvent.click(screen.getByRole('switch', { name: /Testbild auf dem Spiegel zeigen/ }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Drehung, Testbild')
  })

  it('"Verwerfen" restores the stored values and hides the bar', async () => {
    arrange()
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '180°' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Verwerfen' }))
    expect(bar()).not.toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '0°' })).toBeChecked()
  })

  it('"Speichern" sends the full body with the CSRF token and needs no further GET', async () => {
    const api = arrange((call) => json(call.body))
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    await userEvent.click(screen.getByRole('switch', { name: /Testbild auf dem Spiegel zeigen/ }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))

    expect(await screen.findByText('Gespeichert.')).toBeInTheDocument()
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    const put = api.callsTo('PUT /api/v1/settings/display')
    expect(put).toHaveLength(1)
    expect(put[0].body).toEqual({ rotation: 90, output: '', testPattern: true })
    expect(put[0].csrf).toBe('tok-123')
    expect(api.callsTo('GET /api/v1/settings/display')).toHaveLength(1)
    expect(screen.getByRole('radio', { name: '90°' })).toBeChecked()
  })

  it('shows a server error on the output field and keeps the bar and the input', async () => {
    arrange(() => problem(400, 'Ungültig', { errors: { output: 'Unbekannter Ausgang.' } }))
    await open()
    await userEvent.click(screen.getByRole('radio', { name: 'Bestimmter Ausgang' }))
    await userEvent.type(screen.getByLabelText('Ausgang'), 'XYZ-9')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    expect(await screen.findByText('Unbekannter Ausgang.')).toBeInTheDocument()
    expect(bar()).toBeInTheDocument()
    expect(screen.getByLabelText('Ausgang')).toHaveValue('XYZ-9')
  })

  it('shows an error without field in the bar', async () => {
    arrange(() => problem(500, 'Kaputt', { detail: 'Speichern nicht möglich.' }))
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    expect(await within(bar()!).findByText('Speichern nicht möglich.')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '90°' })).toBeChecked()
  })

  it('sends output "" after switching back to "Automatisch"', async () => {
    const api = arrange((call) => json(call.body), { rotation: 0, output: 'HDMI-A-1', testPattern: false })
    renderWithProviders('/admin/display')
    expect(await screen.findByLabelText('Ausgang')).toHaveValue('HDMI-A-1')
    await userEvent.click(screen.getByRole('radio', { name: 'Automatisch' }))
    expect(bar()).toHaveTextContent('Monitor-Ausgang')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/display')).toHaveLength(1))
    expect(api.callsTo('PUT /api/v1/settings/display')[0].body).toEqual({ rotation: 0, output: '', testPattern: false })
  })

  it('does not send a specific output without a name', async () => {
    const api = arrange((call) => json(call.body))
    await open()
    await userEvent.click(screen.getByRole('radio', { name: 'Bestimmter Ausgang' }))
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    expect(await screen.findByText('Gib den Namen des Ausgangs ein.')).toBeInTheDocument()
    expect(api.callsTo('PUT /api/v1/settings/display')).toHaveLength(0)
  })
})

describe('unsaved changes guard', () => {
  it('asks before leaving, stays on "Weiter bearbeiten", leaves on "Verwerfen"', async () => {
    arrange()
    const { router } = await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))

    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    expect(await screen.findByText('Änderungen verwerfen?')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/display')

    await userEvent.click(screen.getByRole('button', { name: 'Weiter bearbeiten' }))
    await waitFor(() => expect(screen.queryByText('Änderungen verwerfen?')).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/admin/display')
    expect(screen.getByRole('radio', { name: '90°' })).toBeChecked()

    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Verwerfen' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin')
  })

  it('holds the logout link too', async () => {
    arrange()
    const { router } = await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    await userEvent.click(screen.getByRole('link', { name: 'Abmelden' }))
    expect(await screen.findByText('Änderungen verwerfen?')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/display')
  })

  it('leaves without a question when nothing changed', async () => {
    arrange()
    const { router } = await open()
    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Übersicht' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin')
  })

  it('registers beforeunload only while there are changes', async () => {
    const add = vi.spyOn(window, 'addEventListener')
    arrange()
    await open()
    expect(add.mock.calls.filter(([type]) => type === 'beforeunload')).toHaveLength(0)
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    expect(add.mock.calls.filter(([type]) => type === 'beforeunload').length).toBeGreaterThan(0)
    add.mockRestore()
  })
})
