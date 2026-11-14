import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

type Level = 'debug' | 'info' | 'warn' | 'error'
const entry = (level: Level, message: string, attrs: { key: string; value: string }[] = [], time = '2026-11-14T10:15:30Z') => ({
  time,
  level,
  message,
  attrs,
})
const book = (entries: ReturnType<typeof entry>[]) => ({ entries, capacity: 500 })

function arrange(logbook: Handler, over: Record<string, Handler> = {}) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/logbook': logbook,
    ...over,
  })
}

const logbookCalls = (api: ReturnType<typeof arrange>) => api.callsTo('GET /api/v1/logbook')

describe('logbook page', () => {
  it('lists entries with time, level badge, message and wrapped attributes under one h1', async () => {
    arrange(() =>
      json(
        book([
          entry('error', 'weather failed', [{ key: 'error', value: 'dial tcp: lookup api.openweathermap.org' }, { key: 'req.id', value: '7' }]),
          entry('warn', 'slow response'),
          entry('info', 'listening'),
        ]),
      ),
    )
    renderWithProviders('/admin/logbook')
    await screen.findByText('weather failed')
    expectSinglePageTitle('Logbuch')
    expect(screen.getByText(/Die letzten 500 Meldungen des Spiegels seit dem letzten Start\. Nach einem Neustart beginnt das Logbuch leer\./)).toBeInTheDocument()

    const items = within(screen.getByRole('list', { name: 'Meldungen' })).getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(within(items[0]).getByText('Fehler')).toBeInTheDocument()
    expect(within(items[1]).getByText('Warnung')).toBeInTheDocument()
    expect(within(items[2]).getByText('Info')).toBeInTheDocument()
    // Berlin time zone (set by the test configuration): 10:15:30 UTC is 11:15:30.
    expect(within(items[0]).getByText('14.11.26, 11:15:30')).toBeInTheDocument()

    const attrs = within(items[0]).getByText('error=dial tcp: lookup api.openweathermap.org req.id=7')
    expect(attrs).toHaveStyle({ wordBreak: 'break-word', fontFamily: 'monospace' })
    expect(within(items[1]).queryByText(/=/)).not.toBeInTheDocument()
  })

  it('colours the level badge per level', async () => {
    arrange(() => json(book([entry('error', 'e'), entry('warn', 'w'), entry('info', 'i'), entry('debug', 'd')])))
    renderWithProviders('/admin/logbook')
    await screen.findByText('e')
    const items = within(screen.getByRole('list', { name: 'Meldungen' })).getAllByRole('listitem')
    // Mantine's light badge takes its colour from a CSS variable named after the colour.
    const colour = (i: number, label: string) => within(items[i]).getByText(label).closest('.mantine-Badge-root')?.getAttribute('style') ?? ''
    expect(colour(0, 'Fehler')).toContain('--mantine-color-red-')
    expect(colour(1, 'Warnung')).toContain('--mantine-color-yellow-')
    expect(colour(2, 'Info')).toContain('--mantine-color-gray-')
    expect(colour(3, 'Debug')).toContain('--mantine-color-gray-')
  })

  it('keeps the old list visible while the new filter loads', async () => {
    let release: (r: Response) => void = () => {}
    const api = arrange((call) =>
      call.search === '?minLevel=warn'
        ? new Promise<Response>((resolve) => (release = resolve))
        : json(book([entry('info', 'old entry')])),
    )
    renderWithProviders('/admin/logbook')
    await screen.findByText('old entry')
    await userEvent.click(screen.getByText('Warnungen'))
    await waitFor(() => expect(logbookCalls(api)).toHaveLength(2))
    expect(screen.getByText('old entry')).toBeInTheDocument()
    expect(screen.queryByLabelText('Logbuch wird geladen')).not.toBeInTheDocument()

    release(json(book([entry('warn', 'new entry')])))
    expect(await screen.findByText('new entry')).toBeInTheDocument()
    expect(screen.queryByText('old entry')).not.toBeInTheDocument()
  })

  it('asks for info by default and changes the request with the filter', async () => {
    const api = arrange(() => json(book([])))
    renderWithProviders('/admin/logbook')
    await screen.findByText('Keine Meldungen.')
    const searches = () => logbookCalls(api).map((c) => c.search)
    expect(searches()).toEqual(['?minLevel=info'])

    await userEvent.click(screen.getByText('Warnungen'))
    await waitFor(() => expect(searches()).toEqual(['?minLevel=info', '?minLevel=warn']))
    await userEvent.click(screen.getByText('Fehler'))
    await waitFor(() => expect(searches()).toEqual(['?minLevel=info', '?minLevel=warn', '?minLevel=error']))
  })

  it('shows the empty state', async () => {
    arrange(() => json(book([])))
    renderWithProviders('/admin/logbook')
    expect(await screen.findByText('Keine Meldungen.')).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Meldungen' })).not.toBeInTheDocument()
  })

  it('shows the error at the place instead of an empty list', async () => {
    arrange(() => problem(500, 'Kaputt'))
    renderWithProviders('/admin/logbook')
    expect(await screen.findByText('Das Logbuch konnte nicht geladen werden.')).toBeInTheDocument()
    expect(screen.queryByText('Keine Meldungen.')).not.toBeInTheDocument()
  })

  it('reloads on the button', async () => {
    const api = arrange(() => json(book([entry('info', 'a')])))
    renderWithProviders('/admin/logbook')
    await screen.findByText('a')
    await userEvent.click(screen.getByRole('button', { name: 'Neu laden' }))
    await waitFor(() => expect(logbookCalls(api)).toHaveLength(2))
  })

  it('refreshes every ten seconds', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const api = arrange(() => json(book([entry('info', 'a')])))
    renderWithProviders('/admin/logbook')
    await screen.findByText('a')
    expect(logbookCalls(api)).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(9_000)
    expect(logbookCalls(api)).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(2_000)
    await waitFor(() => expect(logbookCalls(api)).toHaveLength(2))
  })
})

describe('logbook tile', () => {
  const tileText = async (entries: ReturnType<typeof entry>[]) => {
    arrange(() => json(book(entries)))
    renderWithProviders('/admin')
    return within(await screen.findByRole('link', { name: /^Logbuch/ }))
  }

  it('requests only warnings and above', async () => {
    const api = arrange(() => json(book([])))
    renderWithProviders('/admin')
    await within(await screen.findByRole('link', { name: /^Logbuch/ })).findByText('Keine Warnungen')
    expect(logbookCalls(api).map((c) => c.search)).toEqual(['?minLevel=warn'])
  })

  it('says "Keine Warnungen" for an empty buffer', async () => {
    expect(await (await tileText([])).findByText('Keine Warnungen')).toBeInTheDocument()
  })

  it('counts warnings and errors separately, singular and plural', async () => {
    const w = (await tileText([entry('warn', 'a')])).findByText('1 Warnung')
    expect(await w).toBeInTheDocument()
  })

  it('shows warnings and errors together', async () => {
    const t = await tileText([entry('warn', 'a'), entry('warn', 'b'), entry('warn', 'c'), entry('error', 'd')])
    expect(await t.findByText('3 Warnungen · 1 Fehler')).toBeInTheDocument()
  })

  it('shows only errors when there are no warnings', async () => {
    const t = await tileText([entry('error', 'a'), entry('error', 'b')])
    expect(await t.findByText('2 Fehler')).toBeInTheDocument()
  })

  it('shows "Status nicht verfügbar" when the call fails', async () => {
    arrange(() => problem(500, 'Kaputt'))
    renderWithProviders('/admin')
    const tile = within(await screen.findByRole('link', { name: /^Logbuch/ }))
    expect(await tile.findByText('Status nicht verfügbar')).toBeInTheDocument()
  })
})
