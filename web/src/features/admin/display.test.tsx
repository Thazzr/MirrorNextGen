import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const schedule = { enabled: false, offTime: '23:00', onTime: '06:00' }
const stored = { rotation: 0, output: '', testPattern: false, schedule }
const statusAlways = {
  powerOn: true,
  reason: 'always',
  now: '2026-11-14T13:00:00+01:00',
  timeZone: 'Europe/Berlin',
}

function arrange(put?: Handler, settings: typeof stored = stored, status: object = statusAlways) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/settings/display': () => json(settings),
    'GET /api/v1/display/status': () => json(status),
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
  it('has one h1, the four h2 sections and no bar at first', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Anzeige')
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual(['Drehung', 'Monitor-Ausgang', 'Testbild', 'Nachtruhe'])
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
    expect(put[0].body).toEqual({
      rotation: 90,
      output: '',
      testPattern: true,
      schedule,
    })
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
    const api = arrange((call) => json(call.body), {
      rotation: 0,
      output: 'HDMI-A-1',
      testPattern: false,
      schedule,
    })
    renderWithProviders('/admin/display')
    expect(await screen.findByLabelText('Ausgang')).toHaveValue('HDMI-A-1')
    await userEvent.click(screen.getByRole('radio', { name: 'Automatisch' }))
    expect(bar()).toHaveTextContent('Monitor-Ausgang')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/display')).toHaveLength(1))
    expect(api.callsTo('PUT /api/v1/settings/display')[0].body).toEqual({
      rotation: 0,
      output: '',
      testPattern: false,
      schedule,
    })
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

describe('night schedule', () => {
  const night = { enabled: true, offTime: '23:00', onTime: '06:00' }
  const saved = { ...stored, schedule: night }
  const section = () => screen.getByRole('region', { name: 'Nachtruhe' })

  it('shows the switch, both times and the time zone note from the status', async () => {
    arrange(undefined, saved)
    await open()
    expect(within(section()).getByRole('switch', { name: /Nachtruhe aktivieren/ })).toBeChecked()
    expect(within(section()).getByLabelText('Ausschalten um')).toHaveValue('23:00')
    expect(within(section()).getByLabelText('Einschalten um')).toHaveValue('06:00')
    expect(
      await within(section()).findByText(
        'Gilt in der Zeitzone des Spiegels (Europe/Berlin). Gleiche Zeiten bedeuten: keine Nachtruhe.',
      ),
    ).toBeInTheDocument()
  })

  it('shows the bar naming "Nachtruhe" for the switch and for each time', async () => {
    arrange()
    await open()
    await userEvent.click(within(section()).getByRole('switch', { name: /Nachtruhe aktivieren/ }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Nachtruhe')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Verwerfen' }))
    expect(bar()).not.toBeInTheDocument()

    fireEvent.change(within(section()).getByLabelText('Ausschalten um'), {
      target: { value: '22:15' },
    })
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Nachtruhe')
    fireEvent.change(within(section()).getByLabelText('Ausschalten um'), {
      target: { value: '23:00' },
    })
    expect(bar()).not.toBeInTheDocument()
    fireEvent.change(within(section()).getByLabelText('Einschalten um'), {
      target: { value: '07:30' },
    })
    expect(bar()).toHaveTextContent('Nachtruhe')
  })

  it('"Speichern" sends the schedule, and the status is fetched again', async () => {
    const api = arrange((call) => json(call.body))
    await open()
    await userEvent.click(within(section()).getByRole('switch', { name: /Nachtruhe aktivieren/ }))
    fireEvent.change(within(section()).getByLabelText('Ausschalten um'), {
      target: { value: '22:30' },
    })
    fireEvent.change(within(section()).getByLabelText('Einschalten um'), {
      target: { value: '05:45' },
    })
    const statusCalls = api.callsTo('GET /api/v1/display/status').length
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))

    expect(await screen.findByText('Gespeichert.')).toBeInTheDocument()
    const put = api.callsTo('PUT /api/v1/settings/display')
    expect(put).toHaveLength(1)
    expect(put[0].body).toEqual({
      rotation: 0,
      output: '',
      testPattern: false,
      schedule: { enabled: true, offTime: '22:30', onTime: '05:45' },
    })
    expect(put[0].csrf).toBe('tok-123')
    await waitFor(() => expect(api.callsTo('GET /api/v1/display/status')).toHaveLength(statusCalls + 1), {
      timeout: 1500,
    })
    expect(bar()).not.toBeInTheDocument()
  })

  it('shows the new status answer right after saving, without waiting for the interval', async () => {
    let status: object = statusAlways
    const api = arrange((call) => json(call.body))
    api.handlers['GET /api/v1/display/status'] = () => json(status)
    await open()
    const line = () => within(section()).findByRole('status')
    expect(await line()).toHaveTextContent('Monitor an · keine Nachtruhe')
    const before = api.callsTo('GET /api/v1/display/status').length

    status = { ...statusAlways, powerOn: false, reason: 'test' }
    await userEvent.click(within(section()).getByRole('switch', { name: /Nachtruhe aktivieren/ }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))

    // Well below the 5 s polling interval, so only the invalidation can deliver the answer.
    await waitFor(() => expect(within(section()).getByRole('status')).toHaveTextContent('Monitor aus (Test)'), {
      timeout: 1500,
    })
    expect(api.callsTo('GET /api/v1/display/status')).toHaveLength(before + 1)
  })

  it('shows server errors at the time fields and keeps the bar and the input', async () => {
    arrange(() =>
      problem(400, 'Ungültig', {
        errors: {
          'schedule.offTime': 'Erlaubt ist eine Uhrzeit im Format HH:mm.',
          'schedule.onTime': 'Das geht nicht.',
        },
      }),
    )
    await open()
    fireEvent.change(within(section()).getByLabelText('Ausschalten um'), {
      target: { value: '22:30' },
    })
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    expect(await within(section()).findByText('Erlaubt ist eine Uhrzeit im Format HH:mm.')).toBeInTheDocument()
    expect(within(section()).getByText('Das geht nicht.')).toBeInTheDocument()
    expect(bar()).toBeInTheDocument()
    expect(within(section()).getByLabelText('Ausschalten um')).toHaveValue('22:30')
  })

  describe('status line', () => {
    const line = () => within(section()).findByRole('status')

    it('says the monitor is on by schedule with the next switch off', async () => {
      arrange(undefined, saved, {
        powerOn: true,
        reason: 'schedule',
        nextSwitchAt: '2026-11-14T23:00:00+01:00',
        nextSwitchOn: false,
        now: '2026-11-14T13:00:00+01:00',
        timeZone: 'Europe/Berlin',
      })
      await open()
      expect(await line()).toHaveTextContent('Monitor laut Zeitplan an · nächste Schaltung 23:00 (aus)')
    })

    it('says the monitor is off by schedule with the next switch tomorrow, in the mirror time zone', async () => {
      arrange(undefined, saved, {
        powerOn: false,
        reason: 'schedule',
        nextSwitchAt: '2026-11-15T05:00:00Z', // 06:00 in Berlin, whatever the browser zone is
        nextSwitchOn: true,
        now: '2026-11-14T23:30:00+01:00',
        timeZone: 'Europe/Berlin',
      })
      await open()
      expect(await line()).toHaveTextContent('Monitor laut Zeitplan aus · nächste Schaltung morgen 06:00 (an)')
    })

    it('says the monitor is off for the test', async () => {
      arrange(undefined, saved, {
        ...statusAlways,
        powerOn: false,
        reason: 'test',
      })
      await open()
      expect(await line()).toHaveTextContent('Monitor aus (Test)')
    })

    it('says there is no night rest without a schedule in effect', async () => {
      arrange()
      await open()
      expect(await line()).toHaveTextContent('Monitor an · keine Nachtruhe')
    })

    it('refreshes itself every 5 seconds', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
      try {
        let status: object = {
          ...statusAlways,
          powerOn: false,
          reason: 'test',
        }
        const api = arrange(undefined, saved)
        api.handlers['GET /api/v1/display/status'] = () => json(status)
        await open()
        expect(await line()).toHaveTextContent('Monitor aus (Test)')
        const before = api.callsTo('GET /api/v1/display/status').length

        status = statusAlways
        await vi.advanceTimersByTimeAsync(5100)
        await waitFor(() =>
          expect(within(section()).getByRole('status')).toHaveTextContent('Monitor an · keine Nachtruhe'),
        )
        expect(api.callsTo('GET /api/v1/display/status')).toHaveLength(before + 1)
      } finally {
        vi.useRealTimers()
      }
    })

    it('says so when the status cannot be loaded', async () => {
      mockApi({
        'GET /api/v1/auth/session': () => json(authedSession),
        'GET /api/v1/settings/display': () => json(stored),
        'GET /api/v1/display/status': () => problem(500, 'Kaputt'),
        'GET /api/v1/info': () => json(info),
      })
      await open()
      expect(await line()).toHaveTextContent('Der Monitorstatus konnte nicht geladen werden.')
    })
  })

  it('"Monitor 10 Sekunden ausschalten" posts with the CSRF token, shows the test and needs no unsaved bar', async () => {
    const api = arrange()
    api.handlers['POST /api/v1/display/test-off'] = () => json({ ...statusAlways, powerOn: false, reason: 'test' })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Monitor 10 Sekunden ausschalten' }))

    expect(await within(section()).findByText('Monitor aus (Test)')).toBeInTheDocument()
    const posts = api.callsTo('POST /api/v1/display/test-off')
    expect(posts).toHaveLength(1)
    expect(posts[0].csrf).toBe('tok-123')
    expect(bar()).not.toBeInTheDocument()
  })

  it('works while there are unsaved changes and keeps them', async () => {
    const api = arrange()
    api.handlers['POST /api/v1/display/test-off'] = () => json({ ...statusAlways, powerOn: false, reason: 'test' })
    await open()
    await userEvent.click(screen.getByRole('radio', { name: '90°' }))
    await userEvent.click(screen.getByRole('button', { name: 'Monitor 10 Sekunden ausschalten' }))
    await waitFor(() => expect(api.callsTo('POST /api/v1/display/test-off')).toHaveLength(1))
    expect(screen.getByRole('radio', { name: '90°' })).toBeChecked()
    expect(bar()).toBeInTheDocument()
  })

  it('shows an error when the test switch-off fails', async () => {
    const api = arrange()
    api.handlers['POST /api/v1/display/test-off'] = () => problem(500, 'Kaputt', { detail: 'Test nicht möglich.' })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Monitor 10 Sekunden ausschalten' }))
    expect(await screen.findByText('Test nicht möglich.')).toBeInTheDocument()
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
