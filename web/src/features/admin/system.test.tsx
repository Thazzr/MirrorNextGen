import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

const UPDATE = 'GET /api/v1/system/update'
const POST = 'POST /api/v1/system/update'
const idle = { available: true, pending: false, stalled: false, state: 'idle' }
const buildInfo = { version: '1.2.3', commit: 'abcdef0123456789', buildTime: '2026-01-02T03:04:05Z' }

function arrange(update: Handler, over: Record<string, Handler> = {}) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(buildInfo),
    [UPDATE]: update,
    ...over,
  })
}

const open = async () => {
  const view = renderWithProviders('/admin/system')
  await screen.findByRole('heading', { level: 1, name: 'System' })
  return view
}

const updateButton = () => screen.queryByRole('button', { name: 'Jetzt aktualisieren' })

describe('system page', () => {
  it('shows version, short commit and build time under one h1 with h2 sections', async () => {
    arrange(() => json(idle))
    await open()
    expectSinglePageTitle('System')
    expect(await screen.findByText('Installierte Version: 1.2.3')).toBeInTheDocument()
    expect(screen.getByText('Commit: abcdef0')).toBeInTheDocument()
    // Berlin time zone: 03:04 UTC is 04:04.
    expect(screen.getByText('Gebaut am: 02.01.26, 04:04')).toBeInTheDocument()
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual(['Version', 'Aktualisieren'])
  })

  it('says "unbekannt" for a build without commit and time', async () => {
    arrange(() => json(idle), { 'GET /api/v1/info': () => json({ version: 'dev', commit: 'unknown', buildTime: '' }) })
    await open()
    expect(await screen.findByText('Commit: unbekannt')).toBeInTheDocument()
    expect(screen.getByText('Gebaut am: unbekannt')).toBeInTheDocument()
  })

  it('shows only the hint, no button, when updates are not set up', async () => {
    arrange(() => json({ ...idle, available: false }))
    await open()
    expect(
      await screen.findByText(
        'Updates per Knopf sind auf diesem Spiegel nicht eingerichtet. Wie das geht, steht in deploy/INSTALL.md im Abschnitt „Updates per Knopf“.',
      ),
    ).toBeInTheDocument()
    expect(updateButton()).not.toBeInTheDocument()
  })

  it('shows the error at the place when the update state cannot be loaded', async () => {
    arrange(() => problem(500, 'Kaputt'))
    await open()
    expect(await screen.findByText('Der Update-Status konnte nicht geladen werden.')).toBeInTheDocument()
    expect(updateButton()).not.toBeInTheDocument()
  })

  it('asks for confirmation and only then posts with the CSRF token', async () => {
    let state: object = idle
    const api = arrange(() => json(state), {
      [POST]: () => {
        state = { ...idle, pending: true }
        return json(state, { status: 202 })
      },
    })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Jetzt aktualisieren' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Der Spiegel lädt die neuen Images und startet neu. Das dauert einige Minuten; die Anzeige ist kurz schwarz.')).toBeInTheDocument()
    expect(api.callsTo(POST)).toHaveLength(0)

    await userEvent.click(within(dialog).getByRole('button', { name: 'Abbrechen' }))
    expect(api.callsTo(POST)).toHaveLength(0)

    await userEvent.click(screen.getByRole('button', { name: 'Jetzt aktualisieren' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Aktualisieren' }))
    await waitFor(() => expect(api.callsTo(POST)).toHaveLength(1))
    expect(api.callsTo(POST)[0].csrf).toBe('tok-123')
    expect(await screen.findByText('Angefordert – wartet auf den Update-Dienst …')).toBeInTheDocument()
    expect(updateButton()).not.toBeInTheDocument()
  })

  it('shows the conflict message and refreshes the state on a 409', async () => {
    let state: object = idle
    arrange(() => json(state), {
      [POST]: () => {
        state = { ...idle, pending: true }
        return problem(409, 'Conflict', { detail: 'Ein Update läuft bereits.' })
      },
    })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Jetzt aktualisieren' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Aktualisieren' }))
    expect(await screen.findByText('Ein Update läuft bereits.')).toBeInTheDocument()
    expect(await screen.findByText('Angefordert – wartet auf den Update-Dienst …')).toBeInTheDocument()
  })

  const texts: [string, object, string | RegExp][] = [
    ['idle', idle, 'Noch kein Update angefordert.'],
    ['pending', { ...idle, pending: true }, 'Angefordert – wartet auf den Update-Dienst …'],
    ['running', { ...idle, pending: true, state: 'running', startedAt: '2026-11-14T10:05:00Z' }, 'Update läuft seit 11:05 …'],
    ['running without start time', { ...idle, state: 'running' }, 'Update läuft …'],
    ['succeeded without any time', { ...idle, state: 'succeeded' }, 'Zuletzt aktualisiert.'],
    ['succeeded with only a start time', { ...idle, state: 'succeeded', startedAt: '2026-11-14T10:00:00Z' }, 'Zuletzt aktualisiert am 14.11.26, 11:00.'],
    ['failed without any time', { ...idle, state: 'failed' }, 'Update fehlgeschlagen:'],
    ['failed with only a start time', { ...idle, state: 'failed', startedAt: '2026-11-14T10:00:00Z' }, 'Update fehlgeschlagen am 14.11.26, 11:00:'],
    ['unknown without message', { ...idle, state: 'unknown' }, 'Der Status des Updates ist nicht lesbar.'],
    ['succeeded', { ...idle, state: 'succeeded', finishedAt: '2026-11-14T10:20:00Z' }, 'Zuletzt aktualisiert am 14.11.26, 11:20.'],
    ['unknown', { ...idle, state: 'unknown', message: 'Der Status des Updates ist nicht lesbar.' }, 'Der Status des Updates ist nicht lesbar.'],
  ]
  it.each(texts)('shows the status line for %s', async (_name, state, text) => {
    arrange(() => json(state))
    await open()
    expect(await screen.findByText(text)).toBeInTheDocument()
  })

  it('shows the failure with the host message in a wrapped monospace block and keeps the button', async () => {
    arrange(() =>
      json({ ...idle, state: 'failed', finishedAt: '2026-11-14T10:20:00Z', message: 'pull access denied\nfor ghcr.io/x/very-long-name' }),
    )
    await open()
    expect(await screen.findByText('Update fehlgeschlagen am 14.11.26, 11:20:')).toBeInTheDocument()
    const block = screen.getByText(/pull access denied/)
    expect(block).toHaveStyle({ whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontFamily: 'monospace' })
    expect(updateButton()).toBeInTheDocument()
  })

  it('hides the button while running and while pending', async () => {
    arrange(() => json({ ...idle, state: 'running', startedAt: '2026-11-14T10:05:00Z' }))
    const view = await open()
    await screen.findByText(/Update läuft seit/)
    expect(updateButton()).not.toBeInTheDocument()
    view.unmount()
    vi.unstubAllGlobals()

    arrange(() => json({ ...idle, pending: true }))
    await open()
    await screen.findByText('Angefordert – wartet auf den Update-Dienst …')
    expect(updateButton()).not.toBeInTheDocument()
  })

  it('polls every three seconds while running and stops when it succeeded', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let state: object = { ...idle, state: 'running', startedAt: '2026-11-14T10:05:00Z' }
    const api = arrange(() => json(state))
    await open()
    await screen.findByText(/Update läuft seit/)
    expect(api.callsTo(UPDATE)).toHaveLength(1)

    await vi.advanceTimersByTimeAsync(3100)
    await waitFor(() => expect(api.callsTo(UPDATE)).toHaveLength(2))

    state = { ...idle, state: 'succeeded', finishedAt: '2026-11-14T10:20:00Z' }
    await vi.advanceTimersByTimeAsync(3100)
    expect(await screen.findByText('Zuletzt aktualisiert am 14.11.26, 11:20.')).toBeInTheDocument()
    expect(updateButton()).toBeInTheDocument()

    const after = api.callsTo(UPDATE).length
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.callsTo(UPDATE)).toHaveLength(after)
  })

  it('does not poll earlier than three seconds and polls once at three', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json({ ...idle, state: 'running', startedAt: '2026-11-14T10:05:00Z' }))
    await open()
    await screen.findByText(/Update läuft seit/)
    expect(api.callsTo(UPDATE)).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(2900)
    expect(api.callsTo(UPDATE)).toHaveLength(1)
    // No waitFor: its real-time retries would let a slower interval pass too.
    await vi.advanceTimersByTimeAsync(200)
    expect(api.callsTo(UPDATE)).toHaveLength(2)
  })

  it('polls the build info while an update is active and not while idle', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json({ ...idle, pending: true }))
    await open()
    await screen.findByText('Installierte Version: 1.2.3')
    expect(api.callsTo('GET /api/v1/info')).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(3100)
    await waitFor(() => expect(api.callsTo('GET /api/v1/info')).toHaveLength(2))
  })

  it('does not poll the build info while idle', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json(idle))
    await open()
    await screen.findByText('Installierte Version: 1.2.3')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.callsTo('GET /api/v1/info')).toHaveLength(1)
  })

  const STALLED_TEXT =
    'Der Update-Dienst hat die Anfrage nicht übernommen. Ist er auf dem Spiegel eingerichtet? Siehe deploy/INSTALL.md, Abschnitt „Updates per Knopf“.'

  it('says nobody picked up a stalled request, shows the button again and stops polling', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json({ ...idle, pending: true, stalled: true }))
    await open()
    expect(await screen.findByText(STALLED_TEXT)).toBeInTheDocument()
    expect(screen.queryByText('Angefordert – wartet auf den Update-Dienst …')).not.toBeInTheDocument()
    expect(updateButton()).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.callsTo(UPDATE)).toHaveLength(1)
    expect(api.callsTo('GET /api/v1/info')).toHaveLength(1)
  })

  it('requests again from a stalled state', async () => {
    const api = arrange(() => json({ ...idle, pending: true, stalled: true }), {
      [POST]: () => json({ ...idle, pending: true }, { status: 202 }),
    })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Jetzt aktualisieren' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Aktualisieren' }))
    await waitFor(() => expect(api.callsTo(POST)).toHaveLength(1))
    expect(await screen.findByText('Angefordert – wartet auf den Update-Dienst …')).toBeInTheDocument()
    expect(updateButton()).not.toBeInTheDocument()
  })

  it('shows the normal error, not "startet neu", when a poll fails while no update is active', async () => {
    let failing = false
    arrange(() => (failing ? problem(500, 'Kaputt') : json(idle)), {
      [POST]: () => {
        failing = true
        return problem(409, 'Conflict', { detail: 'Ein Update läuft bereits.' })
      },
    })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Jetzt aktualisieren' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Aktualisieren' }))
    expect(await screen.findByText('Der Update-Status konnte nicht geladen werden.')).toBeInTheDocument()
    expect(screen.queryByText('Der Spiegel startet neu …')).not.toBeInTheDocument()
  })

  it('shows the pending line, not the old error block, for failed together with pending', async () => {
    arrange(() => json({ ...idle, state: 'failed', pending: true, message: 'old failure text' }))
    await open()
    expect(await screen.findByText('Angefordert – wartet auf den Update-Dienst …')).toBeInTheDocument()
    expect(screen.queryByText('old failure text')).not.toBeInTheDocument()
    expect(screen.queryByText(/Update fehlgeschlagen/)).not.toBeInTheDocument()
  })

  it('does not poll while idle', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json(idle))
    await open()
    await screen.findByText('Noch kein Update angefordert.')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.callsTo(UPDATE)).toHaveLength(1)
  })

  it('says the mirror is restarting when a poll fails during an update, keeps polling and shows the new version', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let phase: 'running' | 'down' | 'back' = 'running'
    const api = arrange(
      () => {
        if (phase === 'down') return problem(502, 'Bad Gateway')
        if (phase === 'back') return json({ ...idle, state: 'succeeded', finishedAt: '2026-11-14T10:20:00Z' })
        return json({ ...idle, state: 'running', startedAt: '2026-11-14T10:05:00Z' })
      },
      {
        'GET /api/v1/info': () => (phase === 'back' ? json({ ...buildInfo, version: '1.3.0' }) : json(buildInfo)),
      },
    )
    await open()
    await screen.findByText(/Update läuft seit/)
    expect(await screen.findByText('Installierte Version: 1.2.3')).toBeInTheDocument()

    phase = 'down'
    await vi.advanceTimersByTimeAsync(3100)
    expect(await screen.findByText('Der Spiegel startet neu …')).toBeInTheDocument()
    expect(screen.queryByText(/Update-Status konnte nicht geladen werden/)).not.toBeInTheDocument()
    expect(screen.getByText('Installierte Version: 1.2.3')).toBeInTheDocument()

    const before = api.callsTo(UPDATE).length
    await vi.advanceTimersByTimeAsync(3100)
    await waitFor(() => expect(api.callsTo(UPDATE).length).toBeGreaterThan(before))

    phase = 'back'
    await vi.advanceTimersByTimeAsync(3100)
    expect(await screen.findByText('Installierte Version: 1.3.0')).toBeInTheDocument()
    expect(screen.queryByText('Der Spiegel startet neu …')).not.toBeInTheDocument()
    expect(await screen.findByText('Zuletzt aktualisiert am 14.11.26, 11:20.')).toBeInTheDocument()
  })
})

describe('system tile', () => {
  const tile = async () => within(await screen.findByRole('link', { name: /^System/ }))

  it('shows the version', async () => {
    arrange(() => json(idle))
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Version 1.2.3')).toBeInTheDocument()
  })

  it('adds "Update läuft" while pending or running', async () => {
    arrange(() => json({ ...idle, pending: true }))
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Version 1.2.3 · Update läuft')).toBeInTheDocument()
  })

  it('adds "Update hängt" for a stalled request instead of "Update läuft"', async () => {
    arrange(() => json({ ...idle, pending: true, stalled: true }))
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Version 1.2.3 · Update hängt')).toBeInTheDocument()
  })

  it('adds "Update fehlgeschlagen" after a failure', async () => {
    arrange(() => json({ ...idle, state: 'failed' }))
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Version 1.2.3 · Update fehlgeschlagen')).toBeInTheDocument()
  })

  it('keeps the plain version when the update call fails', async () => {
    arrange(() => problem(500, 'Kaputt'))
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Version 1.2.3')).toBeInTheDocument()
  })

  it('shows "Status nicht verfügbar" when the info call fails', async () => {
    arrange(() => json(idle), { 'GET /api/v1/info': () => problem(500, 'Kaputt') })
    renderWithProviders('/admin')
    expect(await (await tile()).findByText('Status nicht verfügbar')).toBeInTheDocument()
  })
})
